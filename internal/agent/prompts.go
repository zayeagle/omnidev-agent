package agent

// codeExplorationGuidance tells the LLM to locate code via search before partial reads.
const codeExplorationGuidance = `

Code exploration: Before read_file on unknown or large files, use search_code (or search_file for paths) to locate symbols — results are file:line: text. Then read_file with offset (1-based) and limit for a window around the hit (typically ±50–100 lines), not the entire file. Full read without offset is OK only for small files, README, or a first-pass structure overview. Do NOT call read_file again on a path you already read in this session unless you need a different line range. Prefer targeted reads over repeated full reads.`
