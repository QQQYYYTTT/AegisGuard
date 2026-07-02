package mcpbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"aegisguard/internal/interfaces"
)

func TestFrameReaderAndWriterContentLength(t *testing.T) {
	original := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	var buf bytes.Buffer
	writer := NewFrameWriter(&buf)
	if err := writer.WriteFrame(original); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	reader := NewFrameReader(&buf)
	got, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("unexpected frame: %s", string(got))
	}
}

func TestSchemaRegistryParseToolList(t *testing.T) {
	result := json.RawMessage(`{"tools":[{"name":"weather.query","inputSchema":{"type":"object"}}]}`)
	items := parseToolList(result)
	if items["weather.query"] == "" {
		t.Fatalf("expected schema cached")
	}
}

func TestHandleToolsCallBlocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(EvaluateResponse{
			Success: true,
			Data: struct {
				RequestID      string                     `json:"request_id"`
				Action         interfaces.EvaluateResult  `json:"action"`
				Return         *interfaces.EvaluateResult `json:"return,omitempty"`
				Token          string                     `json:"token,omitempty"`
				TokenStatus    string                     `json:"token_status,omitempty"`
				SchemaHash     string                     `json:"schema_hash,omitempty"`
				Filtered       bool                       `json:"filtered,omitempty"`
				FilteredFields []string                   `json:"filtered_fields,omitempty"`
				ResponseBody   string                     `json:"response_body,omitempty"`
			}{
				Action: interfaces.EvaluateResult{
					Decision: interfaces.HumanApproval,
					Reason:   "blocked in test",
				},
			},
		})
	}))
	defer server.Close()

	bridge, err := New(Config{
		BackendURL: server.URL,
		Command:    []string{"cmd"},
		Stdin:      bytes.NewReader(nil),
		Stdout:     io.Discard,
		Stderr:     io.Discard,
	})
	if err != nil {
		t.Fatalf("new bridge: %v", err)
	}
	frame := []byte(`{"jsonrpc":"2.0","id":"1","method":"tools/call","params":{"name":"shell.exec","arguments":{"command":"rm -rf /"}}}`)
	msg := RPCMessage{}
	if err := json.Unmarshal(frame, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := bridge.handleToolsCall(context.Background(), frame, &msg)
	if err != nil {
		t.Fatalf("handle tools call: %v", err)
	}
	if !bytes.Contains(out, []byte(`"error"`)) {
		t.Fatalf("expected MCP error response, got %s", string(out))
	}
}
