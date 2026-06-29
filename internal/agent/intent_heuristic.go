package agent

import (
	"strings"
)

// isPureGreeting reports brief thanks/hello with no task intent.
func isPureGreeting(instruction string) bool {
	s := strings.TrimSpace(instruction)
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	exact := []string{
		"hello", "hi", "hey", "thanks", "thank you", "thx", "ok", "okay", "bye", "goodbye",
		"你好", "嗨", "谢谢", "多谢", "好的", "好吧", "再见", "嗯", "哦",
	}
	for _, g := range exact {
		if lower == g {
			return true
		}
	}
	if len([]rune(s)) <= 16 {
		for _, g := range []string{"hi ", "hello ", "thanks ", "thank you ", "你好", "谢谢"} {
			if strings.HasPrefix(lower, g) && !strings.ContainsAny(lower, "文件代码编译buildfiximplement") {
				return true
			}
		}
	}
	return false
}

// hasPriorCodeActivity is true when this session already did tool/code work.
func (a *Agent) hasPriorCodeActivity() bool {
	if strings.TrimSpace(a.outputDir) != "" {
		return true
	}
	for _, e := range a.session.EntriesCopy() {
		if e.Role == "tool" {
			return true
		}
		if len(e.AssistantToolCalls) > 0 {
			return true
		}
	}
	userTurns := 0
	for _, e := range a.session.EntriesCopy() {
		if e.Role == "user" {
			userTurns++
		}
	}
	return userTurns > 1
}

// looksLikeSimpleChat detects clearly conversational input (strict — avoids false chat on tasks).
func looksLikeSimpleChat(instruction string) bool {
	s := strings.TrimSpace(instruction)
	if s == "" || len([]rune(s)) > 120 {
		return false
	}
	if looksLikeCodeMod(instruction) {
		return false
	}
	if isPureGreeting(s) {
		return true
	}

	lower := strings.ToLower(s)
	chatOpeners := []string{
		"hello", "hi ", "hi,", "thanks", "thank you", "what is", "what are",
		"explain", "why ", "how does", "how do", "tell me about", "describe",
		"你好", "谢谢", "什么是", "解释", "为什么", "怎么样",
	}
	for _, o := range chatOpeners {
		if strings.HasPrefix(lower, o) {
			return true
		}
	}

	// Short pure Q&A (no build/check/compile verbs) — not "完成了吗？最后编译…"
	if strings.Contains(s, "?") || strings.Contains(s, "？") {
		if len([]rune(s)) <= 48 && !looksLikeTaskQuestion(s) {
			return true
		}
	}
	return false
}

// looksLikeTaskQuestion detects questions that imply code/build/verify work.
func looksLikeTaskQuestion(instruction string) bool {
	lower := strings.ToLower(strings.TrimSpace(instruction))
	taskQ := []string{
		"check", "verify", "compile", "build", "fix", "finish", "complete", "done", "ready",
		"implement", "create", "deploy", "run test", "go build", "binary", "again",
		"检查", "编译", "构建", "完成", "好了吗", "完成了吗", "修复", "验证", "二进制",
		"打包", "再次", "重新", "游戏", "项目", "工程", "deliverables",
	}
	for _, hint := range taskQ {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

// looksLikeCodeMod detects code-change intent without an LLM call.
func looksLikeCodeMod(instruction string) bool {
	lower := strings.ToLower(strings.TrimSpace(instruction))
	if lower == "" {
		return false
	}
	if looksLikeTaskQuestion(instruction) {
		return true
	}
	codeHints := []string{
		"fix", "implement", "create", "delete", "refactor", "debug", "build", "write",
		"add ", "update ", "modify", "change", "rename", "move ", "compile", "binary",
		"file", "code", "function", "class", "module", "test", "deploy", "run ",
		".go", ".ts", ".py", ".js", ".java", ".rs", ".md",
		"实现", "修改", "创建", "删除", "重构", "调试", "文件", "函数", "代码", "编写", "修复",
		"编译", "构建", "检查", "验证", "打包", "二进制", "安装", "配置", "完成",
	}
	for _, hint := range codeHints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

// looksSimpleTask detects obvious multi-step phrasing (diagnostics / future use).
func looksSimpleTask(instruction string) bool {
	s := strings.TrimSpace(instruction)
	if s == "" || len([]rune(s)) > 280 {
		return false
	}
	lower := strings.ToLower(s)
	multiStep := []string{
		" and then ", " then ", " after that", " step 1", "1.", "2.",
		"first,", "second,", "parallel", "multiple", "several tasks",
		"同时", "然后", "接着", "最后", "第一步", "第二步", "多个",
	}
	for _, m := range multiStep {
		if strings.Contains(lower, m) {
			return false
		}
	}
	return true
}
