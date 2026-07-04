package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aegisguard/internal/audit"
	"aegisguard/internal/gates"
	"aegisguard/internal/interfaces"
	"aegisguard/internal/vkey"
	"aegisguard/pkg/smcrypto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (r *Router) registerToolProxyRoutes() {
	r.engine.Any("/api/proxy", r.handleAPIProxy)
	r.engine.Any("/api/proxy/:tool", r.handleAPIProxy)
	r.engine.Any("/mcp/proxy", r.handleMCPProxy)
	r.engine.Any("/mcp/proxy/:tool", r.handleMCPProxy)
}

func (r *Router) handleAPIProxy(c *gin.Context) {
	r.handleHTTPToolProxy(c, "api")
}

func (r *Router) handleMCPProxy(c *gin.Context) {
	r.handleHTTPToolProxy(c, "mcp")
}

func (r *Router) handleHTTPToolProxy(c *gin.Context, proxyKind string) {
	start := time.Now()
	requestID := uuid.New().String()
	bodyBytes, _ := c.GetRawData()
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	gatewayKey := r.authorizeGatewayRequest(c, requestID, start, bodyBytes)
	if gatewayKey == "" {
		return
	}

	toolName := strings.TrimSpace(c.Param("tool"))
	toolName = strings.TrimPrefix(toolName, "/")
	if toolName == "" {
		toolName = strings.TrimSpace(c.Query("tool"))
	}
	if toolName == "" {
		toolName = proxyKind + "_http_tool"
	}

	upstreamURL, ok := validateUpstreamURL(c.Query("upstream"))
	if !ok {
		r.writeToolProxyError(c, requestID, start, http.StatusBadRequest, "invalid upstream url")
		return
	}

	agentID, sessionID, taskID := ensureToolExecutionContext(c.Request, gatewayKey, requestID)
	callerAgentID := strings.TrimSpace(c.GetHeader("X-Aegis-Caller-Agent-ID"))
	if proxyKind == "mcp" && callerAgentID == "" {
		r.writeToolProxyError(c, requestID, start, http.StatusBadRequest, "missing caller agent identity for MCP proxy")
		return
	}
	token, err := r.issueToolToken(toolName, agentID, sessionID, taskID, "")
	if err != nil {
		r.logger.Error("issue tool proxy token failed", zap.String("tool", toolName), zap.Error(err))
		r.writeToolProxyError(c, requestID, start, http.StatusInternalServerError, "failed to issue require token")
		return
	}
	if trustedSchema, ok := r.resolveTrustedToolSchema(toolName); ok {
		c.Request.Header.Set("X-Aegis-Tool-Schema", base64.StdEncoding.EncodeToString(trustedSchema))
	} else {
		c.Request.Header.Del("X-Aegis-Tool-Schema")
	}
	c.Request.Header.Set("X-Aegis-Token", token)
	c.Request.Header.Set("X-Aegis-Token-Status", "issued")
	c.Request.Header.Set("X-Aegis-Session-ID", sessionID)
	c.Request.Header.Set("X-Aegis-Task-ID", taskID)
	c.Request.Header.Set("X-Aegis-Agent-ID", agentID)
	if proxyKind == "mcp" {
		c.Request.Header.Set("X-Aegis-Caller-Agent-ID", callerAgentID)
		c.Request.Header.Set("X-Aegis-Boundary-Channel", "mcp_http")
	}

	params := extractToolProxyParams(bodyBytes, c.Request.URL.Query())
	actionResult := r.gateEvaluator.EvaluateAction(requestID, toolName, params, c.Request.Header, agentID)
	setToolProxyHeaders(c.Writer.Header(), "action", actionResult)
	if actionResult.Decision != interfaces.Allow {
		status := http.StatusForbidden
		if actionResult.Decision == interfaces.HumanApproval {
			status = http.StatusAccepted
		}
		r.auditToolProxyResponse(requestID, start, status, actionResult)
		c.JSON(status, gin.H{
			"error": gin.H{
				"message":  "request stopped by AegisGuard",
				"type":     "aegis_gate_decision",
				"decision": actionResult.Decision.String(),
				"reason":   actionResult.Reason,
			},
		})
		return
	}

	ctx := context.WithValue(c.Request.Context(), "request_id", requestID)
	upstreamReq, err := http.NewRequestWithContext(ctx, c.Request.Method, upstreamURL.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		r.writeToolProxyError(c, requestID, start, http.StatusBadGateway, "failed to build upstream request")
		return
	}
	copyToolProxyHeaders(upstreamReq.Header, c.Request.Header)
	upstreamReq.ContentLength = int64(len(bodyBytes))

	resp, err := http.DefaultClient.Do(upstreamReq)
	if err != nil {
		r.logger.Error("tool proxy forwarding failed", zap.String("tool", toolName), zap.String("upstream", upstreamURL.String()), zap.Error(err))
		r.writeToolProxyError(c, requestID, start, http.StatusBadGateway, "upstream request failed")
		return
	}
	defer resp.Body.Close()

	forwardToolProxyHeaders(c.Writer.Header(), resp.Header)
	contentType := resp.Header.Get("Content-Type")

	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			r.writeToolProxyError(c, requestID, start, http.StatusBadGateway, "failed to read upstream streaming response")
			return
		}
		carrierRules := []string(nil)
		respBody, carrierRules = normalizeToolProxyResponse(respBody, contentType, resp.Header.Get("Content-Encoding"), c.Writer.Header())
		returnResult := r.gateEvaluator.EvaluateReturn(requestID, respBody, agentID)
		if len(carrierRules) > 0 {
			returnResult.MatchedRules = append(carrierRules, returnResult.MatchedRules...)
		}
		setToolProxyHeaders(c.Writer.Header(), "return", returnResult)
		switch returnResult.Decision {
		case interfaces.Block, interfaces.Deny:
			r.auditToolProxyResponse(requestID, start, http.StatusForbidden, returnResult)
			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"message":  "response stopped by AegisGuard",
					"type":     "aegis_gate_decision",
					"decision": returnResult.Decision.String(),
					"reason":   returnResult.Reason,
				},
			})
			return
		case interfaces.Degrade:
			if r.contentFilter != nil {
				filtered, removed := r.contentFilter.FilterToolResponse(respBody, toolName)
				respBody = filtered
				if len(removed) > 0 {
					c.Writer.Header().Set("X-Aegis-Filtered-Fields", strings.Join(removed, ","))
				}
			}
			c.Writer.Header().Set("X-Aegis-Filtered", "true")
		}
		r.auditToolProxyResponse(requestID, start, resp.StatusCode, returnResult)
		c.Data(resp.StatusCode, contentType, respBody)
		return
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		r.writeToolProxyError(c, requestID, start, http.StatusBadGateway, "failed to read upstream response")
		return
	}

	respBody, carrierRules := normalizeToolProxyResponse(respBody, contentType, resp.Header.Get("Content-Encoding"), c.Writer.Header())
	if len(carrierRules) > 0 && strings.Contains(strings.Join(carrierRules, ","), "unparseable") {
		contentType = "application/json"
	}

	returnResult := r.gateEvaluator.EvaluateReturn(requestID, respBody, agentID)
	if len(carrierRules) > 0 {
		returnResult.MatchedRules = append(carrierRules, returnResult.MatchedRules...)
	}
	setToolProxyHeaders(c.Writer.Header(), "return", returnResult)
	switch returnResult.Decision {
	case interfaces.Block, interfaces.Deny:
		r.auditToolProxyResponse(requestID, start, http.StatusForbidden, returnResult)
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"message":  "response stopped by AegisGuard",
				"type":     "aegis_gate_decision",
				"decision": returnResult.Decision.String(),
				"reason":   returnResult.Reason,
			},
		})
		return
	case interfaces.Degrade:
		if r.contentFilter != nil {
			filtered, removed := r.contentFilter.FilterToolResponse(respBody, toolName)
			respBody = filtered
			if len(removed) > 0 {
				c.Writer.Header().Set("X-Aegis-Filtered-Fields", strings.Join(removed, ","))
			}
		}
		c.Writer.Header().Set("X-Aegis-Filtered", "true")
	}

	r.auditToolProxyResponse(requestID, start, resp.StatusCode, returnResult)
	c.Data(resp.StatusCode, contentType, respBody)
}

func (r *Router) authorizeGatewayRequest(c *gin.Context, requestID string, start time.Time, body []byte) string {
	gatewayKey := r.vkeyMgr.GatewayKeyID()
	providedKey := vkey.ExtractGatewayCredential(c.Request.Header)

	r.auditor.LogRequest(auditInput(requestID, providedKey, c, body))

	if providedKey == "" {
		r.writeToolProxyError(c, requestID, start, http.StatusUnauthorized, "missing gateway key")
		return ""
	}
	if !r.vkeyMgr.ValidateGatewayKey(providedKey) {
		r.writeToolProxyError(c, requestID, start, http.StatusUnauthorized, "invalid gateway key")
		return ""
	}
	return gatewayKey
}

func auditInput(requestID, gatewayKey string, c *gin.Context, body []byte) audit.LogInput {
	return audit.LogInput{
		RequestID:  requestID,
		GatewayKey: gatewayKey,
		Method:     c.Request.Method,
		Path:       c.Request.URL.Path,
		Body:       body,
		ClientIP:   c.ClientIP(),
	}
}

func (r *Router) issueToolToken(toolName, agentID, sessionID, taskID, schemaHeader string) (string, error) {
	token, err := r.tokenStore.Issue(toolName, toolName+":invoke", agentID, sessionID, taskID, 5*time.Minute, 1)
	if err != nil {
		return "", err
	}
	if schemaHeader != "" {
		rawSchema, err := decodeSchemaHeader(schemaHeader)
		if err != nil {
			return "", err
		}
		token.SchemaHash = smcrypto.SM3Hex(rawSchema)
		if err := token.Sign(); err != nil {
			return "", err
		}
		if err := r.tokenStore.Save(token); err != nil {
			return "", err
		}
	}
	payload, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func (r *Router) resolveTrustedToolSchema(toolName string) ([]byte, bool) {
	if r.toolMeta == nil {
		return nil, false
	}
	return r.toolMeta.Schema(toolName)
}

func decodeSchemaHeader(schemaHeader string) ([]byte, error) {
	schemaHeader = strings.TrimSpace(schemaHeader)
	if schemaHeader == "" {
		return nil, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(schemaHeader); err == nil {
		return decoded, nil
	}
	return []byte(schemaHeader), nil
}

func ensureToolExecutionContext(req *http.Request, gatewayKey, requestID string) (agentID, sessionID, taskID string) {
	agentID = strings.TrimSpace(req.Header.Get("X-Aegis-Agent-ID"))
	if agentID == "" {
		agentID = gatewayKey
	}
	if agentID == "" {
		agentID = "agent-anonymous"
	}

	sessionID = strings.TrimSpace(req.Header.Get("X-Aegis-Session-ID"))
	if sessionID == "" {
		sessionID = requestID
	}
	if sessionID == "" {
		sessionID = "session-anonymous"
	}

	taskID = strings.TrimSpace(req.Header.Get("X-Aegis-Task-ID"))
	if taskID == "" {
		taskID = requestID
	}
	if taskID == "" {
		taskID = "task-anonymous"
	}
	return agentID, sessionID, taskID
}

func extractToolProxyParams(body []byte, values url.Values) map[string]interface{} {
	params := map[string]interface{}{}
	for key, list := range values {
		if key == "upstream" || key == "tool" {
			continue
		}
		if len(list) == 1 {
			params[key] = list[0]
			continue
		}
		params[key] = list
	}

	var decoded any
	if err := json.Unmarshal(body, &decoded); err == nil {
		switch typed := decoded.(type) {
		case map[string]any:
			for k, v := range typed {
				params[k] = v
			}
		default:
			params["raw_body"] = string(body)
		}
	} else if len(body) > 0 {
		params["raw_body"] = string(body)
	}
	return params
}

func validateUpstreamURL(raw string) (*url.URL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return nil, false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, false
	}
	if parsed.Host == "" {
		return nil, false
	}
	return parsed, true
}

func copyToolProxyHeaders(dst, src http.Header) {
	for key, values := range src {
		lower := strings.ToLower(key)
		switch lower {
		case "authorization", "host", "content-length":
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func forwardToolProxyHeaders(dst, src http.Header) {
	for key, values := range src {
		lower := strings.ToLower(key)
		switch lower {
		case "content-length", "transfer-encoding", "connection":
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func setToolProxyHeaders(header http.Header, gateType string, result interfaces.EvaluateResult) {
	header.Set("X-Aegis-Decision", result.Decision.String())
	header.Set("X-Aegis-Gate-Type", gateType)
	header.Set("X-Aegis-Reason", result.Reason)
	header.Set("X-Aegis-Risk-Score", intToString(result.RiskScore))
	header.Set("X-Aegis-Risk-Level", result.RiskLevel)
	header.Set("X-Aegis-Matched-Rules", strings.Join(result.MatchedRules, ","))
}

func normalizeToolProxyResponse(respBody []byte, contentType, contentEncoding string, header http.Header) ([]byte, []string) {
	normalized, rules, err := gates.NormalizeReturnBody(respBody, contentType, contentEncoding)
	if err != nil {
		rules = append(rules, "return_carrier_unparseable")
		header.Del("Content-Encoding")
		header.Set("X-Aegis-Return-Carrier", strings.Join(rules, ","))
		return []byte(`{"error":"response body isolated because its carrier could not be decoded safely"}`), rules
	}
	if len(rules) > 0 || strings.TrimSpace(contentEncoding) != "" {
		header.Del("Content-Encoding")
		header.Set("X-Aegis-Return-Carrier", strings.Join(rules, ","))
	}
	return normalized, rules
}

func intToString(value int) string {
	return strconv.Itoa(value)
}

func (r *Router) auditToolProxyResponse(requestID string, start time.Time, statusCode int, result interfaces.EvaluateResult) {
	r.auditor.LogResponse(requestID, audit.LogResponseInput{
		StatusCode:   statusCode,
		Duration:     time.Since(start),
		Decision:     strings.ToLower(result.Decision.String()),
		Reason:       result.Reason,
		GateType:     "tool_proxy",
		RiskScore:    result.RiskScore,
		RiskLevel:    result.RiskLevel,
		MatchedRules: result.MatchedRules,
		TokenStatus:  "issued",
		AuthMode:     r.cfg.TokenMode,
	})
}

func (r *Router) writeToolProxyError(c *gin.Context, requestID string, start time.Time, statusCode int, reason string) {
	r.auditor.LogResponse(requestID, audit.LogResponseInput{
		StatusCode: statusCode,
		Duration:   time.Since(start),
		Decision:   "block",
		Reason:     reason,
		GateType:   "tool_proxy",
	})
	c.JSON(statusCode, gin.H{"error": reason})
}
