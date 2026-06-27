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
}
