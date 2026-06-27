package agent

import "testing"

func TestLayoutFromHeuristic(t *testing.T) {
	tests := []struct {
		instruction string
		want        ProjectLayout
	}{
		{"build a calculator in Go", LayoutMinimal},
		{"写一个计算器", LayoutMinimal},
		{"hello world program", LayoutMinimal},
		{"simple HTTP hello server", LayoutMinimal},
		{"todo app with React frontend and Go backend", LayoutDDD},
		{"前后端分离的用户管理系统", LayoutDDD},
		{"REST API microservice with PostgreSQL", LayoutDDD},
	}

	for _, tt := range tests {
		got := layoutFromHeuristic(tt.instruction)
		if got != tt.want {
			t.Errorf("layoutFromHeuristic(%q) = %q, want %q", tt.instruction, got, tt.want)
		}
	}
}
