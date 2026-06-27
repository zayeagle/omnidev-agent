package main

import (
	"flag"
)

type cliFlags struct {
	prompt   string
	provider string
	model    string
	version  bool
	yolo     bool // --yolo: auto-approve all operations (no confirmation prompts)
}

func parseFlags() cliFlags {
	var f cliFlags
	flag.StringVar(&f.prompt, "p", "", "Headless mode: execute a single prompt (no TUI)")
	flag.StringVar(&f.provider, "provider", "", "Override LLM provider (openai|deepseek|anthropic)")
	flag.StringVar(&f.model, "model", "", "Override model name")
	flag.BoolVar(&f.version, "version", false, "Print version and exit")
	flag.BoolVar(&f.version, "v", false, "Print version and exit")
	flag.BoolVar(&f.yolo, "yolo", false, "Auto-approve all operations (skip permission prompts)")
	flag.Parse()
	return f
}
