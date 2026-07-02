package mcpbridge

import "encoding/json"

type RPCMessage struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func (m *RPCMessage) IDKey() string {
	if m == nil || m.ID == nil {
		return ""
	}
	switch typed := m.ID.(type) {
	case string:
		return typed
	case float64:
		return jsonNumberString(typed)
	default:
		data, _ := json.Marshal(typed)
		return string(data)
	}
}

func jsonNumberString(v float64) string {
	data, _ := json.Marshal(v)
	return string(data)
}

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type toolsListResult struct {
	Tools []toolDescriptor `json:"tools"`
}

type toolDescriptor struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

