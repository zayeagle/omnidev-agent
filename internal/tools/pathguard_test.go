package tools

import "testing"

func TestIsSensitivePath_APIConfig(t *testing.T) {
	if !IsSensitivePath(".omnidev-agent.json") {
		t.Fatal("expected sensitive")
	}
	if !IsSensitivePath(`C:\Users\me\.omnidev-agent\config.json`) {
		t.Fatal("expected global config sensitive")
	}
}

func TestIsSensitivePath_NormalFile(t *testing.T) {
	if IsSensitivePath("internal/agent/loop.go") {
		t.Fatal("expected normal file allowed")
	}
}

func TestReadFile_BlocksSensitive(t *testing.T) {
	tool := &readFileTool{}
	res := tool.Execute(t.Context(), map[string]interface{}{"path": ".env"})
	if res.Success {
		t.Fatal("expected block")
	}
}
