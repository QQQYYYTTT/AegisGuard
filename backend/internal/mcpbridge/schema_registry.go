package mcpbridge

import (
	"encoding/json"
	"sync"
)

type SchemaRegistry struct {
	mu           sync.RWMutex
	schemas      map[string]string
	pending      map[string]string
	pendingTools map[string]string
}

func NewSchemaRegistry() *SchemaRegistry {
	return &SchemaRegistry{
		schemas:      make(map[string]string),
		pending:      make(map[string]string),
		pendingTools: make(map[string]string),
	}
}

func (r *SchemaRegistry) Set(toolName string, schema string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.schemas[toolName] = schema
}

func (r *SchemaRegistry) Schema(toolName string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.schemas[toolName]
}

func (r *SchemaRegistry) RememberPending(id, method, toolName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending[id] = method
	if toolName != "" {
		r.pendingTools[id] = toolName
	}
}

func (r *SchemaRegistry) PopPendingMethod(id string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	method, ok := r.pending[id]
	if ok {
		delete(r.pending, id)
	}
	return method, ok
}

func (r *SchemaRegistry) PopPendingTool(id string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	tool := r.pendingTools[id]
	delete(r.pendingTools, id)
	return tool
}

func extractToolCall(raw json.RawMessage) (string, map[string]interface{}) {
	var params toolCallParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return "", nil
	}
	return params.Name, params.Arguments
}

func parseToolList(raw json.RawMessage) map[string]string {
	var result toolsListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}
	items := make(map[string]string, len(result.Tools))
	for _, tool := range result.Tools {
		payload, _ := json.Marshal(tool)
		items[tool.Name] = string(payload)
	}
	return items
}

