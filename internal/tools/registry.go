package tools

// RegisterAll registers every tool in the standard toolbox.
func RegisterAll(r *Registry) {
	r.Register(&listDirTool{})
	r.Register(&readFileTool{})
	r.Register(&searchFileTool{})
	r.Register(&searchCodeTool{})
	r.Register(&writeFileTool{})
	r.Register(&editFileTool{})
	r.Register(&shellExecTool{})
	r.Register(&deleteFileTool{})
	r.Register(&gitStatusTool{})
	r.Register(&gitDiffStatTool{})
}

// RegisterSkills adds list_skills / load_skill (requires SetSkillCatalog).
func RegisterSkills(r *Registry) {
	r.Register(&listSkillsTool{})
	r.Register(&loadSkillTool{})
}
