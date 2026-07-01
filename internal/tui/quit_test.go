package tui

import "testing"

func TestIsQuitCommand(t *testing.T) {
	for _, in := range []string{"quit", "QUIT", " exit ", "Exit", ":q", ":quit"} {
		if !isQuitCommand(in) {
			t.Fatalf("%q should be quit command", in)
		}
	}
	if isQuitCommand("exit now") || isQuitCommand("help") {
		t.Fatal("partial or unrelated input should not quit")
	}
}
