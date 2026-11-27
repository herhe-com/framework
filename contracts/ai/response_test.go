package ai

import (
	"encoding/json"
	"testing"
)

func TestResponseFormatOfJSON(t *testing.T) {

	raw, err := json.Marshal(ChatRequest{
		Model:          "test",
		Messages:       []Message{{Role: RoleOfUser, Content: "你好"}},
		ResponseFormat: ResponseFormatOfJSON(),
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	const expected = `{"model":"test","messages":[{"role":"user","content":"你好"}],"response_format":{"type":"json_object"}}`
	if string(raw) != expected {
		t.Errorf("unexpected marshal:\n got: %s\nwant: %s", raw, expected)
	}
}

func TestResponseFormatOfSchema(t *testing.T) {

	format, err := ResponseFormatOfSchema("medication", []byte(`{"type":"object","required":["related"],"properties":{"related":{"type":"boolean"}}}`), true)
	if err != nil {
		t.Fatalf("build response format: %v", err)
	}

	raw, err := json.Marshal(format)
	if err != nil {
		t.Fatalf("marshal response format: %v", err)
	}

	const expected = `{"type":"json_schema","json_schema":{"name":"medication","strict":true,"schema":{"type":"object","required":["related"],"properties":{"related":{"type":"boolean"}}}}}`
	if string(raw) != expected {
		t.Errorf("unexpected marshal:\n got: %s\nwant: %s", raw, expected)
	}
}

func TestResponseFormatOfSchemaOfInvalid(t *testing.T) {

	tests := []struct {
		label  string
		name   string
		schema string
	}{
		{label: "empty-name", name: ""},
		{label: "broken-json", name: "medication", schema: `{"type":`},
		{label: "not-object", name: "medication", schema: `[]`},
		{label: "empty", name: "medication", schema: ``},
	}

	for _, item := range tests {
		t.Run(item.label, func(t *testing.T) {
			format, err := ResponseFormatOfSchema(item.name, []byte(item.schema), false)
			if err == nil {
				t.Fatalf("expected error, got %+v", format)
			}
		})
	}
}

func TestChatRequestOfNoResponseFormat(t *testing.T) {

	raw, err := json.Marshal(ChatRequest{Model: "test", Messages: []Message{{Role: RoleOfUser, Content: "你好"}}})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var received map[string]any
	if err := json.Unmarshal(raw, &received); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	// 未设置时不下发，交由服务端默认行为，避免不认识该参数的网关直接报错。
	if _, exists := received["response_format"]; exists {
		t.Errorf("response_format should be omitted: %s", raw)
	}
}
