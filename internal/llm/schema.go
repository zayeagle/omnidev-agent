package llm

// toolParametersSchema builds a JSON Schema object for OpenAI/Anthropic tool definitions.
func toolParametersSchema(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}
	if t, ok := params["type"].(string); ok && t == "object" {
		if _, hasProps := params["properties"]; hasProps {
			return params
		}
	}
	return map[string]interface{}{
		"type":       "object",
		"properties": params,
		"required":   requiredParamKeys(params),
	}
}

func requiredParamKeys(params map[string]interface{}) []string {
	var keys []string
	for name, raw := range params {
		schema, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if req, ok := schema["required"].(bool); ok && req {
			keys = append(keys, name)
		}
	}
	return keys
}
