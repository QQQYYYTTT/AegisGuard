package main

import (
	"net/http"

	"aegisguard/internal/config"
	httpapi "aegisguard/internal/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	cfg := config.Load()

	// 初始化 zap 日志
	var logger *zap.Logger
	var err error
	if cfg.LogEncoding == "production" {
		logger, err = zap.NewProduction()
	} else {
		devCfg := zap.NewDevelopmentConfig()
		devCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		logger, err = devCfg.Build()
	}
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	logger.Info("【启动】启动日志编码", zap.String("encoding", cfg.LogEncoding))

	logger.Info("【启动】AegisGuard 网关正在启动...")

	logger.Info("【启动】配置加载完成",
		zap.String("port", cfg.Port),
		zap.String("gateway_config", cfg.GatewayConfigPath),
	)

	router, err := httpapi.NewRouter(cfg)
	if err != nil {
		logger.Fatal("【启动】构建路由失败", zap.Error(err))
	}

	logger.Info("【启动】AegisGuard 网关启动成功",
		zap.String("url", "http://localhost:"+cfg.Port),
		zap.String("docs", "https://github.com/your-org/aegisguard"),
	)

	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		logger.Fatal("【启动】监听失败", zap.Error(err))
	}
}
