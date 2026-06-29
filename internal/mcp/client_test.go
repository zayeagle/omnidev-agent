package mcp

import "testing"

func TestAgentToolName(t *testing.T) {
	got := AgentToolName("playwright", "browser_navigate")
	want := "mcp_playwright__browser_navigate"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSanitizeName(t *testing.T) {
	if sanitizeName("a-b.c") != "a_b_c" {
		t.Fatal(sanitizeName("a-b.c"))
	}
}
