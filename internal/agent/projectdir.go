package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

// DeriveProjectDir returns a filesystem-safe folder name from the user instruction.
func DeriveProjectDir(instruction string) string {
	lower := strings.ToLower(instruction)
	for _, p := range projectKeywords {
		if strings.Contains(instruction, p.key) || strings.Contains(lower, p.key) {
			return p.slug
		}
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

	return fmt.Sprintf("project-%s", time.Now().Format("20060102-150405"))
}

var newProjectVerbs = []string{
	"implement", "build", "create", "develop", "make a", "write a", "scaffold", "design",
	"实现", "创建", "开发", "做一个", "完成一个", "编写", "搭建",
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

// EnsureProjectWorkspace creates deliverables/<name>/ under cwd and returns the absolute path.
func EnsureProjectWorkspace(cwd, instruction string) (string, error) {
	name := DeriveProjectDir(instruction)
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
