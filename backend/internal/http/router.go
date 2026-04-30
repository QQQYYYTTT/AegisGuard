package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"aegisguard/internal/config"
	"aegisguard/internal/gateway"
	"aegisguard/internal/vkey"

	"github.com/gin-gonic/gin"
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
	engine    *gin.Engine
	proxy     *gateway.AegisProxy
	vkeyMgr   *vkey.Manager
	logger    *zap.Logger
	targetURL string
}

func NewRouter(cfg config.Config) (*Router, error) {
	zapCfg := zap.NewProductionConfig()
	zapCfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	logger, err := zapCfg.Build()
	if err != nil {
		return nil, err
	}

	vkeyMgr, err := vkey.NewManager(logger, cfg.GatewayConfigPath)
	if err != nil {
		logger.Fatal("网关凭据配置加载失败", zap.Error(err))
		return nil, err
	}

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(requestLogger(logger))

	proxy, err := gateway.NewAegisProxy(vkeyMgr.GetTargetURL(), vkeyMgr, logger)
	if err != nil {
		return nil, err
	}

	router := &Router{
		engine:    engine,
		proxy:     proxy,
		vkeyMgr:   vkeyMgr,
		logger:    logger,
		targetURL: vkeyMgr.GetTargetURL(),
	}

	router.registerRoutes()

	return router, nil
}

func (r *Router) registerRoutes() {
	r.engine.GET("/health", r.handleHealth)

	r.engine.Any("/v1/*path", r.handleProxy)

	r.engine.GET("/audit/logs", r.handleAuditLogs)
}

func (r *Router) handleProxy(c *gin.Context) {
	start := time.Now()
	path := c.Request.URL.Path
	method := c.Request.Method
	clientIP := c.ClientIP()

	bodyBytes, _ := c.GetRawData()
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	authHeader := c.GetHeader("Authorization")
	gatewayKey := vkey.ExtractGatewayKey(authHeader)

	r.logger.Info("收到 Agent 请求",
		zap.String("method", method),
		zap.String("path", path),
		zap.String("client_ip", clientIP),
		zap.Int("body_size", len(bodyBytes)),
	)

	// 调试：完整打印请求头和请求体
	headersStr := ""
	for k, vs := range c.Request.Header {
		for _, v := range vs {
			headersStr += k + ": " + v + "\n"
		}
	}
	bodyStr := string(bodyBytes)
	if bodyStr == "" {
		bodyStr = "(empty)"
	}
	r.logger.Debug("请求详情",
		zap.String("headers", headersStr),
		zap.String("body", bodyStr),
	)

	if gatewayKey == "" {
		r.logger.Error("请求缺少网关密钥",
			zap.String("ip", clientIP),
			zap.String("path", path),
		)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "缺少网关密钥，请求头格式: Authorization: Bearer agk-xxx",
		})
		return
	}

	if !r.vkeyMgr.ValidateGatewayKey(gatewayKey) {
		r.logger.Error("网关密钥验证失败",
			zap.String("gateway_key", gatewayKey),
			zap.String("ip", clientIP),
		)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "网关密钥无效，请检查 agent 配置中的 OPENAI_API_KEY 是否与网关 gateway_key 一致",
		})
		return
	}

	r.logger.Info("网关密钥验证通过",
		zap.String("gateway_key", gatewayKey),
		zap.String("ip", clientIP),
	)

	ctx := context.WithValue(c.Request.Context(), "gateway_key", r.vkeyMgr.GatewayKeyID())
	c.Request = c.Request.WithContext(ctx)

	blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
	c.Writer = blw

	r.proxy.ServeHTTP(c.Writer, c.Request)

	duration := time.Since(start)
	r.logger.Info("请求处理完成",
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status", c.Writer.Status()),
		zap.Int("response_size", blw.body.Len()),
		zap.Duration("duration", duration),
	)
}

func (r *Router) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":      "ok",
		"target_url":  r.targetURL,
		"gateway_key": r.vkeyMgr.GatewayKeyID(),
	})
}

func (r *Router) handleAuditLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"logs": []interface{}{},
		"note": "审计日志功能开发中",
	})
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

		logger.Info("HTTP请求",
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
