package commands

import "testing"

func TestParse_HelpVariants(t *testing.T) {
	cases := []string{"/help", "/Help", "/HELP", "/help ", "／help", "help", "h", "?"}
	for _, in := range cases {
		cmd, _, ok := Parse(in)
		if !ok || cmd != "help" {
			t.Fatalf("Parse(%q) = %q, %v; want help, true", in, cmd, ok)
		}
	}
}

func TestParse_NotBuiltin(t *testing.T) {
	if IsBuiltin("hello") {
		t.Fatal("hello should not be builtin")
	}
	if IsBuiltin("help me fix the bug") {
		t.Fatal("phrase should not be builtin")
	}
}

func TestParse_Subcommands(t *testing.T) {
	cmd, args, ok := Parse("/skill od")
	if !ok || cmd != "skill" || args != "od" {
		t.Fatalf("got %q %q %v", cmd, args, ok)
	}
	cmd, args, ok = Parse("/checkpoint rollback t1")
	if !ok || cmd != "checkpoint" || args != "rollback t1" {
		t.Fatalf("got %q %q %v", cmd, args, ok)
	}
}
