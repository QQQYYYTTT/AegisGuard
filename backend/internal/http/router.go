package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aegisguard/internal/audit"
	"aegisguard/internal/auth"
	"aegisguard/internal/config"
	"aegisguard/internal/contract"
	"aegisguard/internal/db"
	"aegisguard/internal/gates"
	"aegisguard/internal/gateway"
	"aegisguard/internal/interfaces"
	memorysandbox "aegisguard/internal/sandbox"
	"aegisguard/internal/user"
	"aegisguard/internal/vkey"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const ip2regionXDB = "ip2region_v4.xdb"

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

type Router struct {
	engine        *gin.Engine
	proxy         *gateway.AegisProxy
	vkeyMgr       *vkey.Manager
	auditor       *audit.Logger
	auditStore    audit.Storer
	threatMap     *audit.ThreatMapBuilder
	tokenStore    *auth.TokenStore
	verifier      *auth.Verifier
	userService   *user.Service
	policyRuntime *gates.PolicyRuntime
	gateQuery     contract.GateQuery
	gateEvaluator contract.GateEvaluator
	sandboxMgr    contract.SandboxManager
	transferMgr   contract.TransferManager
	contentFilter contract.ContentFilter
	logger        *zap.Logger
	targetURL     string
	cfg           config.Config
}

func NewRouter(cfg config.Config) (*Router, error) {
	var logger *zap.Logger
	var err error

	if cfg.LogEncoding == "production" {
		zapCfg := zap.NewProductionConfig()
		logger, err = zapCfg.Build()
	} else {
		devCfg := zap.NewDevelopmentConfig()
		devCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		logger, err = devCfg.Build()
	}
	if err != nil {
		return nil, err
	}

	logLevel := getZapLevel(cfg.LogLevel)
	logger = logger.WithOptions(zap.IncreaseLevel(logLevel))

	vkeyMgr, err := vkey.NewManager(logger, cfg.GatewayConfigPath)
	if err != nil {
		logger.Fatal("gateway credential config load failed", zap.Error(err))
		return nil, err
	}

	var auditStore audit.Storer
	if cfg.AuditStorageMode == "sqlite" {
		sqliteCfg := audit.SQLiteConfig{
			Path:        cfg.AuditDBPath,
			WALMode:     cfg.SQLiteWALMode,
			CacheSize:   64000,
			BusyTimeout: 5 * time.Second,
		}
		auditStore, err = audit.NewSQLiteStore(sqliteCfg)
		if err != nil {
			logger.Warn("sqlite audit store init failed, falling back to jsonl", zap.Error(err))
			auditStore, err = audit.NewStore(cfg.AuditFile)
			if err != nil {
				logger.Warn("jsonl audit store init failed", zap.Error(err))
			}
		}
	} else {
		auditStore, err = audit.NewStore(cfg.AuditFile)
		if err != nil {
			logger.Warn("jsonl audit store init failed", zap.Error(err))
		}
	}
	auditor := audit.NewLogger(auditStore)

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(requestLogger(logger))

	tokenStore := auth.NewTokenStore()
	verifier := auth.NewVerifier()

	userDB, err := db.OpenSQLite(cfg.UserDBPath)
	if err != nil {
		return nil, err
	}
	userRepo := user.NewRepository(userDB)
	userService := user.NewService(userRepo, cfg.UserTokenSecret)

	sandboxMgr := memorysandbox.NewManager(logger)
	policyRuntime, err := gates.NewPolicyRuntime(filepath.Join(cfg.BackendDir, "data", "policy-config.json"))
	if err != nil {
		return nil, err
	}

	tdgSettings := gates.TDGSettings{
		Enabled:   cfg.TDGEnabled,
		Mode:      cfg.TDGMode,
		MaxNodes:  cfg.TDGMaxNodes,
		MaxRepeat: cfg.TDGMaxRepeat,
		TTL:       cfg.TDGTTL,
	}
	provenanceSettings := gates.ProvenanceSettings{
		Enabled: cfg.ProvenanceEnabled,
		Mode:    cfg.ProvenanceMode,
	}

	// Phase 4（三态纯化引擎）：sandboxMgr 自身持有白名单/黑名单表并决定 log-only/enforce
	// 的具体生效方式，proxy 只需要知道"要不要在 Allow 分支也调用纯化"这一个布尔开关。
	sandboxMgr.SetPurification(cfg.PurificationEnabled, cfg.PurificationMode)

	proxy, err := gateway.NewAegisProxyWithPolicyRuntime(vkeyMgr.GetTargetURL(), vkeyMgr, tokenStore, cfg.TokenMode, cfg.DynamicRuleRoutingEnabled, tdgSettings, provenanceSettings, cfg.PurificationEnabled, logger, policyRuntime)
	if err != nil {
		return nil, err
	}
	proxy.SetSandbox(sandboxMgr, sandboxMgr, sandboxMgr)

	xdbPath := filepath.Join(cfg.BackendDir, "data", ip2regionXDB)
	locator, err := audit.NewLocator(audit.LocatorOptions{XDBPath: xdbPath})
	if err != nil {
		logger.Warn("threat map ip2region locator init failed, fallback to static locator", zap.Error(err))
		locator = audit.NewStaticLocator()
	}

	target := audit.ThreatTarget{Name: cfg.ThreatMapTarget, Coord: cfg.ThreatMapTargetCoord}
	if cfg.ThreatMapTarget == "" || cfg.ThreatMapTargetCoord[0] == 0 {
		target = audit.NewServerLocationDetector(locator).Detect()
		logger.Info("threat map target auto-detected", zap.String("city", target.Name), zap.Any("coord", target.Coord))
	}

	threatMap := audit.NewThreatMapBuilder(auditStore, locator, target)

	decisionStore := proxy.GetDecisionStore()
	gateQuery := gates.NewGateQuery(decisionStore)
	actionGate := gates.NewActionGateWithRuntime(logger, cfg.TokenMode, policyRuntime)
	actionGate.SetDynamicRuleRouting(cfg.DynamicRuleRoutingEnabled)
	actionGate.SetTDG(tdgSettings)
	gateEvaluator := gates.NewGateEvaluator(
		gates.NewMessageGateWithRuntime(policyRuntime),
		actionGate,
		gates.NewReturnGateWithRuntime(policyRuntime),
		decisionStore,
	)

	router := &Router{
		engine:        engine,
		proxy:         proxy,
		vkeyMgr:       vkeyMgr,
		auditor:       auditor,
		auditStore:    auditStore,
		threatMap:     threatMap,
		tokenStore:    tokenStore,
		verifier:      verifier,
		userService:   userService,
		policyRuntime: policyRuntime,
		gateQuery:     gateQuery,
		gateEvaluator: gateEvaluator,
		sandboxMgr:    sandboxMgr,
		transferMgr:   sandboxMgr,
		contentFilter: sandboxMgr,
		logger:        logger,
		targetURL:     vkeyMgr.GetTargetURL(),
		cfg:           cfg,
	}

	router.registerRoutes()
	return router, nil
}

func (r *Router) registerRoutes() {
	r.engine.GET("/health", r.handleHealth)
	r.engine.GET("/get-async-routes", r.handleGetAsyncRoutes)
	r.registerUserRoutes()
	r.registerAuthRoutes()
	r.registerAssistantRoutes()

	r.engine.Any("/v1/*path", r.handleProxy)

	r.engine.GET("/audit/logs", r.handleAuditLogs)
	r.engine.GET("/aegis/audit/chains", r.handleAuditChains)
	r.engine.GET("/aegis/audit/stats", r.handleAuditStats)
	r.engine.GET("/aegis/audit/threat-map", r.handleThreatMap)

	r.engine.GET("/aegis/gate/overview", r.handleGateOverview)
	r.engine.GET("/aegis/gate/decisions", r.handleGateDecisions)
	r.engine.POST("/aegis/gate/evaluate", r.handleGateEvaluate)
	r.registerSandboxRoutes()
	r.registerPolicyRoutes()

	if r.cfg.DevMode {
		r.registerDevRoutes()
	}
}

func (r *Router) registerPolicyRoutes() {
	policyGroup := r.engine.Group("/aegis/policy")
	{
		policyGroup.GET("/config", r.handleGetPolicyConfig)
		policyGroup.GET("/rules", r.handleGetPolicyRules)
		policyGroup.POST("/rules", r.handleCreatePolicyRule)
		policyGroup.PUT("/rules", r.handleUpdatePolicyRule)
		policyGroup.DELETE("/rules/:id", r.handleDeletePolicyRule)
		policyGroup.PUT("/rules/reorder", r.handleReorderPolicyRules)
		policyGroup.PUT("/config", r.handleUpdatePolicyConfig)
	}
}

func (r *Router) registerAuthRoutes() {
	authGroup := r.engine.Group("/aegis/auth")
	{
		authGroup.GET("/token", r.handleGetToken)
		authGroup.POST("/token", r.handleIssueToken)
		authGroup.POST("/verify", r.handleVerifyToken)
		authGroup.GET("/status", r.handleAuthStatus)
	}
}

func (r *Router) registerUserRoutes() {
	userGroup := r.engine.Group("/api/user")
	{
		userGroup.POST("/register", r.handleUserRegister)
		userGroup.POST("/login", r.handleUserLogin)
		userGroup.POST("/refresh", r.handleUserRefresh)
		userGroup.POST("/logout", r.handleUserLogout)
		userGroup.GET("/profile", r.handleUserProfile)
	}
}

func (r *Router) handleProxy(c *gin.Context) {
	start := time.Now()
	path := c.Request.URL.Path
	method := c.Request.Method
	clientIP := c.ClientIP()
	requestID := uuid.New().String()

	bodyBytes, _ := c.GetRawData()
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	authHeader := c.GetHeader("Authorization")
	gatewayKey := vkey.ExtractGatewayKey(authHeader)

	r.logger.Info("received agent request",
		zap.String("request_id", requestID),
		zap.String("method", method),
		zap.String("path", path),
		zap.String("client_ip", clientIP),
		zap.Int("body_size", len(bodyBytes)),
	)

	r.auditor.LogRequest(audit.LogInput{
		RequestID:  requestID,
		GatewayKey: gatewayKey,
		Method:     method,
		Path:       path,
		Body:       bodyBytes,
		ClientIP:   clientIP,
	})

	if gatewayKey == "" {
		r.auditor.LogResponse(requestID, audit.LogResponseInput{
			StatusCode: http.StatusUnauthorized,
			Duration:   time.Since(start),
			Decision:   "block",
			Reason:     "missing gateway key",
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing gateway key, expected Authorization: Bearer agk-xxx"})
		return
	}

	if !r.vkeyMgr.ValidateGatewayKey(gatewayKey) {
		r.auditor.LogResponse(requestID, audit.LogResponseInput{
			StatusCode: http.StatusUnauthorized,
			Duration:   time.Since(start),
			Decision:   "block",
			Reason:     "invalid gateway key",
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid gateway key"})
		return
	}

	ctx := context.WithValue(c.Request.Context(), "gateway_key", r.vkeyMgr.GatewayKeyID())
	ctx = context.WithValue(ctx, "request_id", requestID)
	c.Request = c.Request.WithContext(ctx)

	blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
	c.Writer = blw
	r.proxy.ServeHTTP(c.Writer, c.Request)

	duration := time.Since(start)
	statusCode := c.Writer.Status()

	r.logger.Info("request completed",
		zap.String("request_id", requestID),
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status", statusCode),
		zap.Int("response_size", blw.body.Len()),
		zap.Duration("duration", duration),
	)

	r.auditor.LogResponse(requestID, audit.LogResponseInput{
		StatusCode:        statusCode,
		Duration:          duration,
		Decision:          firstNonEmpty(c.Writer.Header().Get("X-Aegis-Decision"), "allow"),
		Reason:            c.Writer.Header().Get("X-Aegis-Reason"),
		GateType:          c.Writer.Header().Get("X-Aegis-Gate-Type"),
		RiskScore:         parseOptionalInt(c.Writer.Header().Get("X-Aegis-Risk-Score")),
		RiskLevel:         c.Writer.Header().Get("X-Aegis-Risk-Level"),
		MatchedRules:      splitCSV(c.Writer.Header().Get("X-Aegis-Matched-Rules")),
		TokenStatus:       c.Writer.Header().Get("X-Aegis-Token-Status"),
		AuthMode:          c.Writer.Header().Get("X-Aegis-Auth-Mode"),
		UnauthorizedAllow: strings.EqualFold(c.Writer.Header().Get("X-Aegis-Unauthorized-Allow"), "true"),
	})
}

func (r *Router) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"target_url":    r.targetURL,
		"gateway_key":   r.vkeyMgr.GatewayKeyID(),
		"audit_pending": r.auditor.PendingCount(),
	})
}

func (r *Router) handleGetAsyncRoutes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    []interface{}{},
	})
}

func (r *Router) handleAuditLogs(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit := 50
	if n, err := parseInt(limitStr); err == nil && n > 0 && n <= 500 {
		limit = n
	}

	if r.auditStore == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []audit.AuditEvent{}, "total": 0})
		return
	}

	allEvents, err := r.auditStore.ReadAll()
	if err != nil {
		r.logger.Error("read audit logs failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read audit logs failed"})
		return
	}

	display := allEvents
	if len(display) > limit {
		display = display[:limit]
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": display, "total": len(allEvents)})
}

func (r *Router) handleGateOverview(c *gin.Context) {
	overview, err := r.gateQuery.Overview()
	if err != nil {
		r.logger.Error("get gate overview failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "get gate overview failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": overview})
}

func (r *Router) handleGateDecisions(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit := 50
	if n, err := parseInt(limitStr); err == nil && n > 0 && n <= 500 {
		limit = n
	}

	gateType := c.Query("gate_type")
	action := c.Query("action")

	decisions, err := r.gateQuery.Decisions(limit, gateType, action)
	if err != nil {
		r.logger.Error("get gate decisions failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "get gate decisions failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": decisions, "total": len(decisions)})
}

func (r *Router) handleGateEvaluate(c *gin.Context) {
	var req struct {
		Type     string                 `json:"type"`
		Body     json.RawMessage        `json:"body,omitempty"`
		Content  string                 `json:"content,omitempty"`
		ToolName string                 `json:"tool_name,omitempty"`
		AgentID  string                 `json:"agent_id,omitempty"`
		Params   map[string]interface{} `json:"params,omitempty"`
		Headers  map[string]string      `json:"headers,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	var result interfaces.EvaluateResult
	evalRequestID := uuid.New().String()
	start := time.Now()

	switch req.Type {
	case "message":
		result = r.gateEvaluator.EvaluateMessage(evalRequestID, buildEvaluationBody(req.Body, req.Content), req.AgentID)
	case "action":
		httpHeaders := make(http.Header)
		for k, v := range req.Headers {
			httpHeaders.Set(k, v)
		}
		result = r.gateEvaluator.EvaluateAction(evalRequestID, req.ToolName, req.Params, httpHeaders, req.AgentID)
	case "return":
		result = r.gateEvaluator.EvaluateReturn(evalRequestID, buildEvaluationBody(req.Body, req.Content), req.AgentID)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid gate type"})
		return
	}

	r.auditor.LogRequest(audit.LogInput{
		RequestID: evalRequestID,
		Method:    c.Request.Method,
		Path:      c.Request.URL.Path,
		Body:      buildEvaluationBody(req.Body, req.Content),
		ClientIP:  c.ClientIP(),
	})
	r.auditor.LogResponse(evalRequestID, audit.LogResponseInput{
		StatusCode:   httpStatusForDecision(result.Decision.String()),
		Duration:     time.Since(start),
		Decision:     result.Decision.String(),
		Reason:       result.Reason,
		GateType:     req.Type,
		RiskScore:    result.RiskScore,
		RiskLevel:    result.RiskLevel,
		MatchedRules: result.MatchedRules,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": interfaces.GateDecision{
			RequestID:    "manual",
			Timestamp:    time.Now(),
			GateType:     req.Type,
			Decision:     result.Decision,
			RiskScore:    result.RiskScore,
			RiskLevel:    result.RiskLevel,
			MatchedRules: result.MatchedRules,
			Reason:       result.Reason,
			ToolName:     req.ToolName,
			AgentID:      req.AgentID,
		},
	})
}

func buildEvaluationBody(raw json.RawMessage, content string) []byte {
	if len(raw) > 0 && string(raw) != "null" {
		return raw
	}
	if strings.TrimSpace(content) == "" {
		return []byte(`{}`)
	}
	body, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		return []byte(`{}`)
	}
	return body
}

func (r *Router) handleAuditChains(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit := 50
	if n, err := parseInt(limitStr); err == nil && n > 0 && n <= 500 {
		limit = n
	}

	if r.auditStore == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []audit.AttackChain{}, "total": 0})
		return
	}

	events, err := r.auditStore.ReadAll()
	if err != nil {
		r.logger.Error("read audit chains failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "read audit chains failed"})
		return
	}
	chains := audit.BuildAttackChains(events, limit)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    chains,
		"total":   audit.CountAttackChains(events),
	})
}

func (r *Router) handleAuditStats(c *gin.Context) {
	stats := gin.H{
		"total_events":          0,
		"today_events":          0,
		"attack_chains":         0,
		"avg_duration_ms":       0,
		"top_agents":            []gin.H{},
		"decision_distribution": gin.H{},
	}
	if r.auditStore == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
		return
	}

	events, err := r.auditStore.ReadAll()
	if err != nil {
		r.logger.Error("read audit stats failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "read audit stats failed"})
		return
	}

	today := time.Now().Format("2006-01-02")
	decisionDistribution := map[string]int{}
	agentCounts := map[string]int{}
	totalDuration := int64(0)
	todayEvents := 0
	for _, event := range events {
		if event.Timestamp.Format("2006-01-02") == today {
			todayEvents++
		}
		decision := firstNonEmpty(event.Decision, "unknown")
		decisionDistribution[decision]++
		if agentID := firstNonEmpty(event.GatewayKey, ""); agentID != "" {
			agentCounts[agentID]++
		}
		totalDuration += event.DurationMs
	}

	avgDuration := int64(0)
	if len(events) > 0 {
		avgDuration = totalDuration / int64(len(events))
	}
	stats["total_events"] = len(events)
	stats["today_events"] = todayEvents
	stats["attack_chains"] = audit.CountAttackChains(events)
	stats["avg_duration_ms"] = avgDuration
	stats["decision_distribution"] = decisionDistribution
	stats["top_agents"] = buildTopAgentStats(agentCounts, 6)

	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

func buildTopAgentStats(agentCounts map[string]int, limit int) []gin.H {
	if len(agentCounts) == 0 || limit <= 0 {
		return []gin.H{}
	}

	type agentCount struct {
		agentID string
		count   int
	}

	list := make([]agentCount, 0, len(agentCounts))
	for agentID, count := range agentCounts {
		list = append(list, agentCount{agentID: agentID, count: count})
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].count == list[j].count {
			return list[i].agentID < list[j].agentID
		}
		return list[i].count > list[j].count
	})

	if len(list) > limit {
		list = list[:limit]
	}

	result := make([]gin.H, 0, len(list))
	for _, item := range list {
		result = append(result, gin.H{
			"agent_id": item.agentID,
			"count":    item.count,
		})
	}
	return result
}

func (r *Router) handleThreatMap(c *gin.Context) {
	window := time.Hour
	if w := c.Query("window"); w != "" {
		if d, err := time.ParseDuration(w); err == nil && d > 0 && d <= 24*time.Hour {
			window = d
		}
	}

	target := audit.ThreatTarget{Name: "广州", Coord: [2]float64{113.264, 23.129}}
	if r.threatMap != nil {
		target = r.threatMap.Target()
	}

	if r.threatMap == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": audit.ThreatMapData{
				Target:      target,
				Stats:       audit.ThreatMapStats{},
				Provinces:   []audit.ThreatMapProvince{},
				Cities:      []audit.ThreatMapCity{},
				Lines:       []audit.ThreatMapLine{},
				GeneratedAt: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	data, err := r.threatMap.Build(c.Request.Context(), window)
	if err != nil {
		r.logger.Error("build threat map failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "build threat map failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func parseOptionalInt(value string) int {
	n, err := parseInt(value)
	if err != nil {
		return 0
	}
	return n
}

func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func requestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		logger.Debug("http request",
			zap.String("client_ip", clientIP),
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
		)
	}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.engine.ServeHTTP(w, req)
}

func (r *Router) Engine() *gin.Engine {
	return r.engine
}

func getZapLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
