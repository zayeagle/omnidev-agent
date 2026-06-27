package agent

import "testing"

func TestLooksLikeSimpleChat(t *testing.T) {
	if !looksLikeSimpleChat("hello") {
		t.Fatal("hello should be simple chat")
	}
	if looksLikeSimpleChat("fix the bug in main.go") {
		t.Fatal("code task should not be simple chat")
	}
}
