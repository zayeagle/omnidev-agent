package agent

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/zayeagle/omnidev-agent/internal/session"
)

const cachedReadPrefix = "[CACHED — identical read_file path/range already loaded this session; do not repeat]\n"
const throttledReadPrefix = "[THROTTLED — this file was read repeatedly without edits; using last loaded content. Edit the file or read a different path instead of re-reading]\n"

const maxDiskReadsPerPathBeforeThrottle = 2

type sessionReadCache struct {
	mu          sync.Mutex
	hits        map[string]string
	pathReads   map[string]int
	pathLatest  map[string]string
}

func newSessionReadCache() *sessionReadCache {
	return &sessionReadCache{
		hits:       make(map[string]string),
		pathReads:  make(map[string]int),
		pathLatest: make(map[string]string),
	}
}

func (c *sessionReadCache) Reset() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits = make(map[string]string)
	c.pathReads = make(map[string]int)
	c.pathLatest = make(map[string]string)
}

func normalizeToolPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

func readFileCacheKey(args map[string]interface{}) string {
	path := normalizeToolPath(fmt.Sprint(args["path"]))
	if path == "" {
		return ""
	}
	offset, limit := 1, 0
	if v, ok := args["offset"]; ok {
		offset = argInt(v, 1)
	}
	if v, ok := args["limit"]; ok {
		limit = argInt(v, 0)
	}
	return fmt.Sprintf("%s:%d:%d", path, offset, limit)
}

func argInt(v interface{}, defaultVal int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return defaultVal
	}
}

func (c *sessionReadCache) Get(args map[string]interface{}) (string, bool, string) {
	if c == nil {
		return "", false, ""
	}
	key := readFileCacheKey(args)
	if key == "" {
		return "", false, ""
	}
	path := normalizeToolPath(fmt.Sprint(args["path"]))
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.hits[key]; ok {
		return v, true, cachedReadPrefix
	}
	if path != "" && c.pathReads[path] >= maxDiskReadsPerPathBeforeThrottle {
		if latest := c.pathLatest[path]; latest != "" {
			return latest, true, throttledReadPrefix
		}
	}
	return "", false, ""
}

func (c *sessionReadCache) Put(args map[string]interface{}, result string) {
	if c == nil {
		return
	}
	key := readFileCacheKey(args)
	path := normalizeToolPath(fmt.Sprint(args["path"]))
	if key == "" || result == "" || path == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits[key] = result
	c.pathLatest[path] = result
	c.pathReads[path]++
}

// InvalidatePath clears cached reads for a file after write/edit/delete.
func (c *sessionReadCache) InvalidatePath(path string) {
	if c == nil {
		return
	}
	norm := normalizeToolPath(path)
	if norm == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.hits {
		if strings.HasPrefix(k, norm+":") {
			delete(c.hits, k)
		}
	}
	delete(c.pathLatest, norm)
	c.pathReads[norm] = 0
}

func exploredFilesAddendum(entries []session.Entry) string {
	counts := map[string]int{}
	for _, e := range entries {
		for _, tc := range e.AssistantToolCalls {
			if tc.Name != "read_file" {
				continue
			}
			p := strings.TrimSpace(fmt.Sprint(tc.Arguments["path"]))
			if p == "" {
				continue
			}
			counts[filepath.Clean(p)]++
		}
	}
	if len(counts) == 0 {
		return ""
	}
	paths := make([]string, 0, len(counts))
	for p := range counts {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var b strings.Builder
	b.WriteString("\n\nFiles already read this session (do NOT call read_file again on the same path unless you need a different offset/limit):\n")
	for _, p := range paths {
		if n := counts[p]; n > 1 {
			b.WriteString(fmt.Sprintf("- %s (%d reads — redundant)\n", p, n))
		} else {
			b.WriteString("- " + p + "\n")
		}
	}
	return b.String()
}

func countUniqueReadPaths(entries []session.Entry) int {
	seen := map[string]struct{}{}
	for _, e := range entries {
		for _, tc := range e.AssistantToolCalls {
			if tc.Name != "read_file" {
				continue
			}
			p := strings.TrimSpace(fmt.Sprint(tc.Arguments["path"]))
			if p == "" {
				continue
			}
			seen[filepath.Clean(p)] = struct{}{}
		}
	}
	return len(seen)
}
