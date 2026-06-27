package llm

import "sort"

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

	props := make(map[string]interface{}, len(params))
	var required []string
	for name, raw := range params {
		schema, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		prop := make(map[string]interface{}, len(schema))
		for k, v := range schema {
			if k == "required" {
				if req, ok := v.(bool); ok && req {
					required = append(required, name)
				}
				continue
			}
			prop[k] = v
		}
		props[name] = prop
	}

	sort.Strings(required)
	out := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
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
	sort.Strings(keys)
	return keys
}
