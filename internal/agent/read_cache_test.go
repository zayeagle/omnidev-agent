package agent

import (
	"strings"
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/session"
)

func TestReadFileCacheKey(t *testing.T) {
	k1 := readFileCacheKey(map[string]interface{}{"path": "a.go", "offset": float64(1)})
	k2 := readFileCacheKey(map[string]interface{}{"path": "a.go", "offset": float64(10)})
	if k1 == k2 {
		t.Fatal("different offsets should differ")
	}
}

func TestSessionReadCacheHit(t *testing.T) {
	c := newSessionReadCache()
	args := map[string]interface{}{"path": "main.go"}
	c.Put(args, "content")
	got, ok, prefix := c.Get(args)
	if !ok || got != "content" || prefix != cachedReadPrefix {
		t.Fatalf("cache miss: ok=%v got=%q prefix=%q", ok, got, prefix)
	}
}

func TestSessionReadCacheInvalidatePath(t *testing.T) {
	c := newSessionReadCache()
	args := map[string]interface{}{"path": "game.go"}
	c.Put(args, "v1")
	c.InvalidatePath("game.go")
	if _, ok, _ := c.Get(args); ok {
		t.Fatal("expected cache miss after invalidate")
	}
}

func TestSessionReadCacheThrottleSamePath(t *testing.T) {
	c := newSessionReadCache()
	args1 := map[string]interface{}{"path": "game.go", "offset": float64(1), "limit": float64(50)}
	args2 := map[string]interface{}{"path": "game.go", "offset": float64(1), "limit": float64(0)}
	c.Put(args1, "chunk")
	c.Put(args2, "full")
	got, ok, prefix := c.Get(map[string]interface{}{"path": "game.go", "offset": float64(10)})
	if !ok || got != "full" || prefix != throttledReadPrefix {
		t.Fatalf("expected throttle, ok=%v got=%q prefix=%q", ok, got, prefix)
	}
}

func TestExploredFilesAddendum(t *testing.T) {
	entries := []session.Entry{{
		Role: "assistant",
		AssistantToolCalls: []session.ToolCallData{
			{Name: "read_file", Arguments: map[string]interface{}{"path": "internal/a.go"}},
			{Name: "read_file", Arguments: map[string]interface{}{"path": "internal/a.go"}},
		},
	}}
	addendum := exploredFilesAddendum(entries)
	if !strings.Contains(addendum, "a.go") || !strings.Contains(addendum, "2 reads") {
		t.Fatalf("unexpected addendum: %q", addendum)
	}
}

func TestSlimReadFileResultForHistory(t *testing.T) {
	got := slimReadFileResultForHistory("line1\nline2\n" + strings.Repeat("x", 3000))
	if strings.Contains(got, "use read_file on") {
		t.Fatal("should not prompt re-read")
	}
	if !strings.Contains(got, "do NOT re-read") {
		t.Fatal("should warn against re-read")
	}
}
