package agent

import (
	"strings"
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/session"
)

func TestFallbackSessionSummaryPartial(t *testing.T) {
	a := New(nil, nil, nil, session.New())
	s := a.BuildSessionSummary(nil, SessionOutcomePartial, "task iteration limit (20)", nil, nil)
	if s == "" {
		t.Fatal("expected non-empty partial summary")
	}
	if !strings.Contains(s, "Recommended solution") {
		t.Fatalf("expected solution section: %q", s)
	}
}

func TestFallbackSessionSummaryFailed(t *testing.T) {
	a := New(nil, nil, nil, session.New())
	criteria := []CriterionStatus{{Text: "build", Met: false}}
	s := a.BuildSessionSummary(nil, SessionOutcomeFailed, "verification failed", nil, criteria)
	if s == "" {
		t.Fatal("expected non-empty failed summary")
	}
	for _, want := range []string{"Failure reason", "Recommended solution", "verification failed"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %q", want, s)
		}
	}
}

func TestFormatSuccessFallbackSummary(t *testing.T) {
	s := formatSuccessFallbackSummary("build api", nil, nil)
	if !strings.Contains(s, "Changes & optimizations") || !strings.Contains(s, "Next steps") {
		t.Fatalf("bad success fallback: %q", s)
	}
}
