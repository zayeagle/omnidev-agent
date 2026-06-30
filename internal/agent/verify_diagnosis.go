package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// VerifyFailureKind classifies mechanical verify failures before recovery.
type VerifyFailureKind string

const (
	VerifyFailureNone        VerifyFailureKind = ""
	VerifyFailureProgram     VerifyFailureKind = "program"
	VerifyFailurePath        VerifyFailureKind = "path"
	VerifyFailureEnvironment VerifyFailureKind = "environment"
	VerifyFailureTestHarness VerifyFailureKind = "test"
	VerifyFailureUnknown     VerifyFailureKind = "unknown"
)

// VerifyDiagnosis explains likely root cause and recommended next steps.
type VerifyDiagnosis struct {
	Kind       VerifyFailureKind
	Confidence string // high, medium, low
	Summary    string
	Evidence   []string
	Actions    []string
}

func (k VerifyFailureKind) Label() string {
	switch k {
	case VerifyFailureProgram:
		return "application logic / implementation bug"
	case VerifyFailurePath:
		return "path / workspace / module layout"
	case VerifyFailureEnvironment:
		return "environment / toolchain / dependencies"
	case VerifyFailureTestHarness:
		return "unit test expectations (tests may be wrong or too strict)"
	case VerifyFailureUnknown:
		return "unclear — investigate before editing code"
	default:
		return ""
	}
}

func diagnoseMechanicalVerify(verifyDir string, buildOK, testOK, testsRan bool, summary string) VerifyDiagnosis {
	if strings.TrimSpace(summary) == "" && buildOK && (!testsRan || testOK) {
		return VerifyDiagnosis{}
	}

	lower := strings.ToLower(summary)
	layoutEvidence := dedupeStrings(diagnoseWorkspaceLayout(verifyDir))
	evidence := layoutEvidence
	kind := VerifyFailureUnknown
	confidence := "low"
	if len(layoutEvidence) > 0 {
		kind = VerifyFailurePath
		confidence = "high"
	}

	if hints := matchPatterns(lower, summary, pathFailurePatterns); len(hints) > 0 {
		kind = VerifyFailurePath
		confidence = "high"
		evidence = append(evidence, hints...)
	}
	if hints := matchPatterns(lower, summary, envFailurePatterns); len(hints) > 0 {
		if kind == VerifyFailureUnknown || kind == VerifyFailurePath {
			kind = VerifyFailureEnvironment
			confidence = "high"
		}
		evidence = append(evidence, hints...)
	}

	if !buildOK {
		if kind == VerifyFailureUnknown {
			if matchAny(lower, programFailurePatterns) {
				kind = VerifyFailureProgram
				confidence = "medium"
				evidence = append(evidence, "compiler/build errors point to source code or imports")
			} else {
				kind = VerifyFailureProgram
				confidence = "low"
				evidence = append(evidence, "build failed — inspect compiler output before changing unrelated files")
			}
		}
	} else if testsRan && !testOK {
		if kind == VerifyFailurePath && confidence == "high" {
			evidence = append(evidence, "tests failed — fix workspace/path issues before changing application code")
		} else {
			switch kind {
			case VerifyFailureUnknown, VerifyFailurePath:
				if matchAny(lower, testHarnessPatterns) || strings.Contains(summary, "_test.go") {
					kind = VerifyFailureTestHarness
					confidence = "medium"
					evidence = append(evidence, "build passed but tests failed — inspect failing _test.go before rewriting main code")
				} else if matchAny(lower, programFailurePatterns) {
					kind = VerifyFailureProgram
					confidence = "medium"
					evidence = append(evidence, "test assertions indicate runtime/logic failures")
				} else {
					kind = VerifyFailureTestHarness
					confidence = "medium"
					evidence = append(evidence, "build passed; manual run may work while unit tests fail — compare test expectations vs actual behavior")
				}
			}
		}
	}

	evidence = dedupeStrings(evidence)
	d := VerifyDiagnosis{
		Kind:       kind,
		Confidence: confidence,
		Evidence:   evidence,
	}
	d.Summary = fmt.Sprintf("Likely cause: %s (%s confidence)", kind.Label(), confidence)
	d.Actions = recoveryActionsFor(kind, verifyDir)
	return d
}

var pathFailurePatterns = []string{
	"no such file or directory",
	"cannot find package",
	"no go files in",
	"build constraints exclude all go files",
	"does not contain package",
	"go.mod file not found",
	"no required module provides",
	"cannot load module",
	"wrong module path",
	"not in std",
	"no such file",
	"custom verify skipped: no workspace directory",
}

var envFailurePatterns = []string{
	"executable file not found",
	"command not found",
	"connection refused",
	"proxy.golang.org",
	"verification timed out",
	"access is denied",
	"permission denied",
	"missing go.sum",
	"checksum mismatch",
	"go: download",
	"cannot find go",
}

var programFailurePatterns = []string{
	"undefined:",
	"declared and not used",
	"cannot use",
	"syntax error",
	"panic:",
	"--- fail:",
	"expected ",
	"got ",
	"not equal",
}

var testHarnessPatterns = []string{
	"_test.go:",
	"testmain",
	"testing.t",
	"test timed out",
	"no tests to run",
}

func diagnoseWorkspaceLayout(dir string) []string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return []string{"verify directory unset — build/test may run against repo root instead of deliverables workspace"}
	}
	if _, err := os.Stat(dir); err != nil {
		return []string{"verify directory does not exist: " + dir}
	}
	var hints []string
	if !fileExists(filepath.Join(dir, "go.mod")) && hasGoSources(dir) {
		hints = append(hints, "Go sources exist but go.mod missing under "+dir)
	}
	if hasGoTests(dir) && !fileExists(filepath.Join(dir, "go.mod")) {
		hints = append(hints, "_test.go files present without go.mod in workspace")
	}
	return hints
}

func matchPatterns(lower, raw string, patterns []string) []string {
	var out []string
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			out = append(out, "output: "+p)
		}
	}
	_ = raw
	return out
}

func matchAny(lower string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func recoveryActionsFor(kind VerifyFailureKind, verifyDir string) []string {
	dir := strings.TrimSpace(verifyDir)
	if dir == "" {
		dir = "(workspace unknown)"
	}
	switch kind {
	case VerifyFailurePath:
		return []string{
			"Confirm verify runs in " + dir + " (outputDir / deliverables workspace)",
			"Check go.mod module path matches import paths in source files",
			"Ensure new files were written under the workspace, not repo root or parent dirs",
			"Use list_dir/read_file on failing paths before editing application logic",
		}
	case VerifyFailureEnvironment:
		return []string{
			"Check go/node/python toolchain is available in PATH",
			"Run go mod tidy / go mod download in " + dir + " if modules fail to resolve",
			"Inspect proxy/network/permission errors — these are not fixed by changing game logic",
		}
	case VerifyFailureTestHarness:
		return []string{
			"Read failing test names and _test.go assertions first",
			"If the app runs manually (go run) but tests fail, fix or relax tests — do not assume main code is broken",
			"Only change application code when test expectations match the user request",
		}
	case VerifyFailureProgram:
		return []string{
			"Read compiler/test output and stack traces to locate failing source",
			"Fix the specific functions/packages cited in errors",
		}
	default:
		return []string{
			"Step 1: classify failure (path vs env vs tests vs code) using verify output",
			"Step 2: list_dir/read_file/shell_exec in " + dir + " to confirm layout",
			"Step 3: apply the smallest fix for the classified cause — avoid blind rewrites",
		}
	}
}

func formatDiagnosisBlock(d VerifyDiagnosis, verifyDir string) string {
	if d.Kind == VerifyFailureNone {
		return ""
	}
	var b strings.Builder
	b.WriteString("── Failure diagnosis ──\n")
	b.WriteString(d.Summary + "\n")
	if dir := strings.TrimSpace(verifyDir); dir != "" {
		b.WriteString("Verify dir: " + dir + "\n")
	}
	if len(d.Evidence) > 0 {
		b.WriteString("Evidence:\n")
		for _, ev := range d.Evidence {
			b.WriteString("  - " + ev + "\n")
		}
	}
	if len(d.Actions) > 0 {
		b.WriteString("Suggested actions (in order):\n")
		for i, act := range d.Actions {
			b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, act))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatRecoveryDiagnosis(mech mechanicalVerifyResult) string {
	if mech.Diagnosis.Kind == VerifyFailureNone && !mech.allOK() {
		mech.Diagnosis = diagnoseMechanicalVerify(mech.VerifyDir, mech.BuildOK, mech.TestOK, mech.TestsRan, mech.Summary)
	}
	block := formatDiagnosisBlock(mech.Diagnosis, mech.VerifyDir)
	if block == "" {
		return ""
	}
	if mech.Diagnosis.Kind == VerifyFailurePath || mech.Diagnosis.Kind == VerifyFailureEnvironment {
		block += "\n\nDo NOT keep rewriting application code until path/toolchain issues are ruled out."
	}
	if mech.Diagnosis.Kind == VerifyFailureTestHarness {
		block += "\n\nDo NOT assume the program is broken when build passed and only unit tests failed."
	}
	return block
}

func formatVerifyFixPrompt(d VerifyDiagnosis, verifyDir, summary string) string {
	var b strings.Builder
	b.WriteString(`[VERIFICATION FAILED] Build or tests did not pass.

Before editing application code, classify the failure (path / environment / tests / program).
Use read_file/list_dir on the workspace and inspect the verify output below.
`)
	if block := formatDiagnosisBlock(d, verifyDir); block != "" {
		b.WriteString("\n")
		b.WriteString(block)
		b.WriteString("\n")
	}
	b.WriteString("\nVerify output:\n")
	b.WriteString(summary)
	b.WriteString("\n\nRe-check with go build ./... and go test ./... from the workspace root only.")
	return strings.TrimSpace(b.String())
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func hasGoSources(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".go") && !strings.HasSuffix(d.Name(), "_test.go") {
			found = true
		}
		return nil
	})
	return found
}

func diagnosisStallHint(d VerifyDiagnosis) string {
	switch d.Kind {
	case VerifyFailurePath:
		return "Failures look like path/workspace issues — stop rewriting application code; fix module paths and file locations."
	case VerifyFailureEnvironment:
		return "Failures look like toolchain/dependency issues — fix environment before changing source."
	case VerifyFailureTestHarness:
		return "Build passed but tests failed — inspect test expectations; manual run may already satisfy the user request."
	default:
		return ""
	}
}
