package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"aegisguard/internal/audit"
	"aegisguard/internal/config"
	"aegisguard/internal/gateway"
	"aegisguard/internal/vkey"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

type Router struct {
	engine     *gin.Engine
	proxy      *gateway.AegisProxy
	vkeyMgr    *vkey.Manager
	auditor    *audit.Logger
	auditStore *audit.Store // 直接持有 Store 引用，用于 /audit/logs 读取
	logger     *zap.Logger
	targetURL  string
	cfg        config.Config // 保存配置引用，用于判断运行模式
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

	// 根据配置覆盖日志级别
	logLevel := getZapLevel(cfg.LogLevel)
	logger = logger.WithOptions(zap.IncreaseLevel(logLevel))

	vkeyMgr, err := vkey.NewManager(logger, cfg.GatewayConfigPath)
	if err != nil {
		logger.Fatal("网关凭据配置加载失败", zap.Error(err))
		return nil, err
	}

	// 初始化审计存储
	auditStore, err := audit.NewStore(cfg.AuditFile)
	if err != nil {
		logger.Warn("审计存储初始化失败，审计功能将不可用",
			zap.String("file", cfg.AuditFile),
			zap.Error(err),
		)
	}
	auditor := audit.NewLogger(auditStore)

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(requestLogger(logger))

	proxy, err := gateway.NewAegisProxy(vkeyMgr.GetTargetURL(), vkeyMgr, logger)
	if err != nil {
		return nil, err
	}

	router := &Router{
		engine:     engine,
		proxy:      proxy,
		vkeyMgr:    vkeyMgr,
		auditor:    auditor,
		auditStore: auditStore,
		logger:     logger,
		targetURL:  vkeyMgr.GetTargetURL(),
		cfg:        cfg,
	}

	router.registerRoutes()

	return router, nil
}

func (r *Router) registerRoutes() {
	r.engine.GET("/health", r.handleHealth)

	r.engine.Any("/v1/*path", r.handleProxy)

	r.engine.GET("/audit/logs", r.handleAuditLogs)

	if r.cfg.DevMode {
		r.registerDevRoutes()
	}
}

func (r *Router) handleProxy(c *gin.Context) {
	start := time.Now()
	path := c.Request.URL.Path
	method := c.Request.Method
	clientIP := c.ClientIP()

	// 生成请求唯一 ID 用于审计事件关联
	requestID := uuid.New().String()

	bodyBytes, _ := c.GetRawData()
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	authHeader := c.GetHeader("Authorization")
	gatewayKey := vkey.ExtractGatewayKey(authHeader)

	r.logger.Info("收到 Agent 请求",
		zap.String("request_id", requestID),
		zap.String("method", method),
		zap.String("path", path),
		zap.String("client_ip", clientIP),
		zap.Int("body_size", len(bodyBytes)),
	)

	// 记录请求审计事件（还不会写入 Store，仅缓存）
	r.auditor.LogRequest(audit.LogInput{
		RequestID:  requestID,
		GatewayKey: gatewayKey,
		Method:     method,
		Path:       path,
		Body:       bodyBytes,
		ClientIP:   clientIP,
	})

	if gatewayKey == "" {
		r.logger.Error("请求缺少网关密钥",
			zap.String("request_id", requestID),
			zap.String("ip", clientIP),
			zap.String("path", path),
		)
		r.auditor.LogResponse(requestID, audit.LogResponseInput{
			StatusCode: http.StatusUnauthorized,
			Duration:   time.Since(start),
			Decision:   "block",
			Reason:     "缺少网关密钥",
		})
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "缺少网关密钥，请求头格式: Authorization: Bearer agk-xxx",
		})
		return
	}

	if !r.vkeyMgr.ValidateGatewayKey(gatewayKey) {
		r.logger.Error("网关密钥验证失败",
			zap.String("request_id", requestID),
			zap.String("gateway_key", gatewayKey),
			zap.String("ip", clientIP),
		)
		r.auditor.LogResponse(requestID, audit.LogResponseInput{
			StatusCode: http.StatusUnauthorized,
			Duration:   time.Since(start),
			Decision:   "block",
			Reason:     "网关密钥无效",
		})
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "网关密钥无效，请检查 agent 配置中的 OPENAI_API_KEY 是否与网关 gateway_key 一致",
		})
		return
	}

	r.logger.Debug("网关密钥验证通过",
		zap.String("request_id", requestID),
		zap.String("gateway_key", gatewayKey),
		zap.String("ip", clientIP),
	)

	ctx := context.WithValue(c.Request.Context(), "gateway_key", r.vkeyMgr.GatewayKeyID())
	ctx = context.WithValue(ctx, "request_id", requestID)
	c.Request = c.Request.WithContext(ctx)

	blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
	c.Writer = blw

	r.proxy.ServeHTTP(c.Writer, c.Request)

	duration := time.Since(start)
	statusCode := c.Writer.Status()

	r.logger.Info("请求处理完成",
		zap.String("request_id", requestID),
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status", statusCode),
		zap.Int("response_size", blw.body.Len()),
		zap.Duration("duration", duration),
	)

	r.auditor.LogResponse(requestID, audit.LogResponseInput{
		StatusCode: statusCode,
		Duration:   duration,
		Decision:   "allow", // 当前未从 gates 获取决策，统一记为 allow
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

func (r *Router) handleAuditLogs(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit := 50
	if n, err := parseInt(limitStr); err == nil && n > 0 && n <= 500 {
		limit = n
	}

	if r.auditStore == nil {
		c.JSON(http.StatusOK, gin.H{
			"logs":  []audit.AuditEvent{},
			"total": 0,
			"note":  "审计存储未初始化",
		})
		return
	}

	allEvents, err := r.auditStore.ReadAll()
	if err != nil {
		r.logger.Error("读取审计日志失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "读取审计日志失败",
		})
		return
	}

	// 按时间降序取前 limit 条
	display := allEvents
	if len(display) > limit {
		display = display[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  display,
		"total": len(allEvents),
	})
}

// parseInt 简单整数解析，错误时返回默认值
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

// requestLogger Gin 中间件：记录每个 HTTP 请求的耗时和状态（DEBUG 级别，避免终端刷屏）
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

		logger.Debug("HTTP请求",
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

// getZapLevel 将字符串级别转换为 zapcore.Level
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
