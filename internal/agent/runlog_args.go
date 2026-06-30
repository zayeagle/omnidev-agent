package agent

import (
	"fmt"
	"sort"
	"strings"
)

// SummarizeToolArgsForLog returns a compact one-line summary for run logs (no bulky payloads).
func SummarizeToolArgsForLog(name string, args map[string]interface{}) string {
	if args == nil {
		return ""
	}
	slim := SlimToolArguments(name, args)
	keys := make([]string, 0, len(slim))
	for k := range slim {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, slim[k]))
	}
	return strings.Join(parts, " ")
}
