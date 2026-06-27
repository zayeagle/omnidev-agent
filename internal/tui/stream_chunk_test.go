package tui

import "testing"

func TestPrepareStreamChunk_PreservesLeadingSpace(t *testing.T) {
	text, marker, ok := prepareStreamChunk(" How")
	if !ok || marker != "" || text != " How" {
		t.Fatalf("got append=%q marker=%q ok=%v, want append=%q", text, marker, ok, " How")
	}
}

func TestPrepareStreamChunk_PipelineMarker(t *testing.T) {
	_, marker, ok := prepareStreamChunk("Conversation mode — responding directly.")
	if !ok || marker == "" {
		t.Fatal("expected pipeline marker")
	}
}

func TestPrepareStreamChunk_Empty(t *testing.T) {
	if _, _, ok := prepareStreamChunk(""); ok {
		t.Fatal("empty chunk should be skipped")
	}
}

func TestPrepareStreamChunk_ArchitectureHidden(t *testing.T) {
	_, marker, ok := prepareStreamChunk("Architecture: minimal — prefer a single file or very few files; no DDD scaffold.")
	if !ok || marker == "" {
		t.Fatal("architecture status should be pipeline noise, not shown in reply")
	}
}
