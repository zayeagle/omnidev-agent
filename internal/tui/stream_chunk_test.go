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
