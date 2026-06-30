package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

var projectKeywords = []struct {
	key  string
	slug string
}{
	{"贪吃蛇", "snake-game"},
	{"snake", "snake-game"},
	{"坦克大战", "tank-game"},
	{"坦克", "tank-game"},
	{"俄罗斯方块", "tetris"},
	{"tetris", "tetris"},
	{"博客", "blog"},
	{"blog", "blog"},
	{"待办", "todo-app"},
	{"todo", "todo-app"},
	{"聊天", "chat-app"},
	{"chat", "chat-app"},
	{"计算器", "calculator"},
	{"calculator", "calculator"},
	{"hello world", "hello-server"},
	{"hello word", "hello-server"},
	{"http server", "hello-server"},
}

// Longer Chinese phrases first (matched before token splitting).
var chinesePhraseSlugs = []struct {
	phrase string
	slug   string
}{
	{"终端小游戏", "terminal-game"},
	{"终端游戏", "terminal-game"},
	{"命令行工具", "cli-tool"},
	{"小游戏", "mini-game"},
	{"网页应用", "web-app"},
	{"后台服务", "backend-service"},
}

var chineseTokenSlugs = []struct {
	token string
	slug  string
}{
	{"终端", "terminal"},
	{"命令行", "cli"},
	{"网页", "web"},
	{"后台", "backend"},
	{"服务", "server"},
	{"应用", "app"},
	{"程序", "program"},
	{"游戏", "game"},
}

// DeriveProjectDir returns a filesystem-safe folder name from the user instruction.
func DeriveProjectDir(instruction string) string {
	return DeriveProjectDirFromTexts(instruction)
}

// DeriveProjectDirFromTexts tries each instruction (newest first) until a descriptive slug is found.
func DeriveProjectDirFromTexts(instructions ...string) string {
	for _, instruction := range instructions {
		if strings.TrimSpace(instruction) == "" {
			continue
		}
		if name := deriveProjectDirSingle(instruction); !isFallbackProjectName(name) {
			return name
		}
	}
	for _, instruction := range instructions {
		if strings.TrimSpace(instruction) != "" {
			if name := deriveProjectDirSingle(instruction); name != "" {
				return name
			}
		}
	}
	return fallbackProjectName()
}

func deriveProjectDirSingle(instruction string) string {
	lower := strings.ToLower(instruction)
	for _, p := range projectKeywords {
		if strings.Contains(instruction, p.key) || strings.Contains(lower, p.key) {
			return p.slug
		}
	}

	for _, p := range chinesePhraseSlugs {
		if strings.Contains(instruction, p.phrase) {
			return p.slug
		}
	}

	if slug := deriveChineseTokenSlug(instruction); slug != "" {
		return slug
	}

	var words []string
	for _, w := range strings.FieldsFunc(instruction, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		w = strings.ToLower(w)
		if len(w) >= 2 && isASCIIWord(w) {
			words = append(words, w)
		}
		if len(words) >= 3 {
			break
		}
	}
	if len(words) > 0 {
		slug := strings.Join(words, "-")
		slug = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(slug, "")
		if slug != "" {
			return slug
		}
	}

	return fallbackProjectName()
}

func deriveChineseTokenSlug(instruction string) string {
	var parts []string
	seen := map[string]bool{}
	for _, tok := range chineseTokenSlugs {
		if !strings.Contains(instruction, tok.token) {
			continue
		}
		if seen[tok.slug] {
			continue
		}
		seen[tok.slug] = true
		parts = append(parts, tok.slug)
	}
	if len(parts) == 0 {
		return ""
	}
	sort.Strings(parts)
	return strings.Join(parts, "-")
}

func isFallbackProjectName(name string) bool {
	return strings.HasPrefix(name, "project-") && len(name) == len("project-")+len("20060102-150405")
}

func fallbackProjectName() string {
	return fmt.Sprintf("project-%s", time.Now().Format("20060102-150405"))
}

var newProjectVerbs = []string{
	"implement", "build", "create", "develop", "make a", "write a", "scaffold", "design",
	"实现", "创建", "开发", "做一个", "完成一个", "编写", "搭建", "生成", "写一个", "写个", "做个",
}

var newProjectNouns = []string{
	"app", "game", "server", "cli", "tool", "project", "program", "service",
	"游戏", "应用", "程序", "服务",
}

// IsNewProjectRequest reports whether the instruction asks for a standalone new app
// (as opposed to modifying files in the existing repository).
func IsNewProjectRequest(instruction string) bool {
	lower := strings.ToLower(instruction)

	hasVerb := false
	for _, v := range newProjectVerbs {
		if strings.Contains(lower, v) || strings.Contains(instruction, v) {
			hasVerb = true
			break
		}
	}
	if !hasVerb {
		return false
	}

	for _, p := range projectKeywords {
		if strings.Contains(instruction, p.key) || strings.Contains(lower, p.key) {
			return true
		}
	}
	for _, p := range chinesePhraseSlugs {
		if strings.Contains(instruction, p.phrase) {
			return true
		}
	}
	for _, n := range newProjectNouns {
		if strings.Contains(lower, n) || strings.Contains(instruction, n) {
			return true
		}
	}
	return false
}

func isASCIIWord(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// shouldReuseProjectWorkspace keeps the existing deliverables folder for follow-up turns.
func (a *Agent) shouldReuseProjectWorkspace(instruction string) bool {
	if strings.TrimSpace(a.outputDir) == "" {
		return false
	}
	if _, err := os.Stat(a.outputDir); err != nil {
		return false
	}
	if !a.hasPriorCodeActivity() {
		return false
	}
	return !IsNewProjectRequest(instruction)
}

func (a *Agent) userInstructionsForNaming() []string {
	var out []string
	for _, e := range a.session.EntriesCopy() {
		if e.Role != "user" {
			continue
		}
		if s := strings.TrimSpace(e.Content); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// EnsureProjectWorkspace creates deliverables/<name>/ under cwd and returns the absolute path.
func EnsureProjectWorkspace(cwd string, instructions ...string) (string, error) {
	name := DeriveProjectDirFromTexts(instructions...)
	dir := filepath.Join(cwd, "deliverables", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir, nil
	}
	return abs, nil
}
