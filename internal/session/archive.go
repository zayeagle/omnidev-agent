package session

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ArchiveFile is one persisted session or log artifact on disk.
type ArchiveFile struct {
	Path    string
	Name    string
	Kind    string // "session" | "log"
	ModTime time.Time
	Size    int64
}

// ListArchives returns recent .md session/log files under the given directories.
func ListArchives(sessionsDir, logsDir string, limit int) ([]ArchiveFile, error) {
	if limit < 1 {
		limit = 20
	}
	var out []ArchiveFile
	collect := func(dir, kind string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			info, err := e.Info()
			if err != nil {
				continue
			}
			out = append(out, ArchiveFile{
				Path:    path,
				Name:    e.Name(),
				Kind:    kind,
				ModTime: info.ModTime(),
				Size:    info.Size(),
			})
		}
	}
	collect(sessionsDir, "session")
	collect(logsDir, "log")

	sort.Slice(out, func(i, j int) bool {
		return out[i].ModTime.After(out[j].ModTime)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ReadArchivePreview reads up to maxBytes from an archive file.
func ReadArchivePreview(path string, maxBytes int) (string, error) {
	if maxBytes < 512 {
		maxBytes = 8192
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) <= maxBytes {
		return string(data), nil
	}
	text := string(data[:maxBytes])
	return text + "\n\n… (truncated, open file for full content)", nil
}
