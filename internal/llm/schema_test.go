package llm

import (
	"encoding/json"
	"testing"
)

func TestToolParametersSchemaOmitsInlineRequired(t *testing.T) {
	schema := toolParametersSchema(map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Path to the file to read.",
			"required":    true,
		},
		"limit": map[string]interface{}{
			"type": "integer",
		},
	})

	path, ok := schema["properties"].(map[string]interface{})["path"].(map[string]interface{})
	if !ok {
		t.Fatal("missing path property schema")
	}
	if _, has := path["required"]; has {
		t.Fatalf("property schema must not contain inline required: %v", path)
	}

	req, ok := schema["required"].([]string)
	if !ok || len(req) != 1 || req[0] != "path" {
		t.Fatalf("required = %v, want [path]", schema["required"])
	}

	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["required"]; !ok {
		t.Fatal("expected top-level required array")
	}
}

func TestToolParametersSchemaOmitsEmptyRequired(t *testing.T) {
	schema := toolParametersSchema(map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "optional path",
		},
	})
	if _, ok := schema["required"]; ok {
		t.Fatalf("optional params should omit required key, got %v", schema["required"])
	}
}
