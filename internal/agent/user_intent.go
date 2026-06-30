package agent

import "strings"

// userExplicitlyWantsTests reports whether the user asked for unit tests or test-driven work.
func userExplicitlyWantsTests(instruction string) bool {
	lower := strings.ToLower(strings.TrimSpace(instruction))
	if lower == "" {
		return false
	}
	hints := []string{
		"unit test", "unit tests", "单元测试", "写测试", "编写测试", "测试用例",
		"test coverage", "coverage", "tdd", "_test.go", "go test", "测试覆盖",
		"add tests", "write tests", "with tests",
	}
	for _, h := range hints {
		if strings.Contains(lower, h) {
			return true
		}
	}
	return false
}
