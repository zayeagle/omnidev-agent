package components

import "testing"

func TestSummarizeToolResult_ReadFile(t *testing.T) {
	got := SummarizeToolResult("read_file", true, "package main\n\nfunc main() {}\n", "")
	if got != "read 4 lines (29 bytes)" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestSummarizeToolResult_ListDir(t *testing.T) {
	data := "d cmd 0B\nf main.go 259B\n"
	got := SummarizeToolResult("list_dir", true, data, "")
	if got != "2 entries: d cmd 0B, f main.go 259B" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestSummarizeToolResult_Shell(t *testing.T) {
	got := SummarizeToolResult("shell_exec", true, "line1\nline2\n", "")
	if got != "line1 (+1 lines)" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestSummarizeToolResult_GoTestPass(t *testing.T) {
	data := "=== RUN   TestFoo\n--- PASS: TestFoo (0.00s)\nPASS\nok  \texample.com/pkg\t0.012s\n"
	got := SummarizeToolResult("shell_exec", true, data, "")
	if got != "all tests passed (1)" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestSummarizeToolResult_GoTestFail(t *testing.T) {
	err := "exit code: exit status 1\n=== RUN   TestFoo\n--- FAIL: TestFoo (0.00s)\nFAIL\n"
	got := SummarizeToolResult("shell_exec", false, "", err)
	if got != "tests failed (1 failed)" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestSummarizeToolResult_WriteKeepsShort(t *testing.T) {
	msg := "updated main.go (+45 -0)"
	got := SummarizeToolResult("write_file", true, msg, "")
	if got != msg {
		t.Fatalf("unexpected: %q", got)
	}
}
