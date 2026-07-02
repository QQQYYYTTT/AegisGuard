package mcpbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"aegisguard/internal/interfaces"
)

type BackendClient struct {
	baseURL string
	client  *http.Client
	bridgeKey string
}

type BridgeActionRequest struct {
	RequestID string                 `json:"request_id"`
	ToolName  string                 `json:"tool_name"`
	AgentID   string                 `json:"agent_id"`
	SessionID string                 `json:"session_id"`
	TaskID    string                 `json:"task_id"`
	Params    map[string]interface{} `json:"params"`
	Schema    string                 `json:"schema,omitempty"`
}

type BridgeReturnRequest struct {
	RequestID    string `json:"request_id"`
	ToolName     string `json:"tool_name"`
	AgentID      string `json:"agent_id"`
	ResponseBody string `json:"response_body"`
}

type EvaluateResponse struct {
	Success bool `json:"success"`
	Data    struct {
		RequestID      string                     `json:"request_id"`
		Action         interfaces.EvaluateResult  `json:"action"`
		Return         *interfaces.EvaluateResult `json:"return,omitempty"`
		Token          string                     `json:"token,omitempty"`
		TokenStatus    string                     `json:"token_status,omitempty"`
		SchemaHash     string                     `json:"schema_hash,omitempty"`
		Filtered       bool                       `json:"filtered,omitempty"`
		FilteredFields []string                   `json:"filtered_fields,omitempty"`
		ResponseBody   string                     `json:"response_body,omitempty"`
	} `json:"data"`
}

func NewBackendClient(baseURL string, bridgeKey string) *BackendClient {
	return &BackendClient{
		baseURL: baseURL,
		bridgeKey: bridgeKey,
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

func (c *BackendClient) EvaluateAction(ctx context.Context, req BridgeActionRequest) (*EvaluateResponse, error) {
	return c.doJSON(ctx, "/aegis/bridge/evaluate/action", req)
}

func (c *BackendClient) EvaluateReturn(ctx context.Context, req BridgeReturnRequest) (*EvaluateResponse, error) {
	return c.doJSON(ctx, "/aegis/bridge/evaluate/return", req)
}

func (c *BackendClient) doJSON(ctx context.Context, path string, payload any) (*EvaluateResponse, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.bridgeKey != "" {
		httpReq.Header.Set("X-Aegis-Bridge-Key", c.bridgeKey)
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result EvaluateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
