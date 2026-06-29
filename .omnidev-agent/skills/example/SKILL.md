# Example Skill

Use this template for omnidev-agent skills (Cursor-compatible layout).

## When to use

- User asks for a repeatable workflow documented in this file
- Before starting, run `load_skill` with name `example` (or `/skill example` in TUI)

## Steps

1. Confirm the user goal in one sentence
2. Use `list_dir` and `read_file` to inspect the repo
3. Make minimal changes; run tests via `shell_exec` when appropriate
4. Summarize what changed and how to verify

## Constraints

- Do not skip permission prompts for shell/delete
- Prefer editing existing files over large rewrites
