package main

import (
	"net/http"
	"time"

	"aegisguard/internal/auth"
	"aegisguard/internal/config"
	httpapi "aegisguard/internal/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	cfg := config.Load()

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

	logger.Info("startup log configured", zap.String("encoding", cfg.LogEncoding))
	logger.Info("starting AegisGuard gateway")
	logger.Info("configuration loaded",
		zap.String("port", cfg.Port),
		zap.String("gateway_config", cfg.GatewayConfigPath),
		zap.String("token_mode", cfg.TokenMode),
	)

	if err := auth.InitSigningKey(cfg.SigningPrivateKey); err != nil {
		logger.Fatal("failed to initialize RequireToken signing key", zap.Error(err))
	}

	auth.StartNonceGC(time.Hour)
	defer auth.StopNonceGC()
	logger.Info("nonce GC started", zap.Duration("interval", time.Hour))

	router, err := httpapi.NewRouter(cfg)
	if err != nil {
		logger.Fatal("failed to build router", zap.Error(err))
	}

	logger.Info("AegisGuard gateway started",
		zap.String("url", "http://localhost:"+cfg.Port),
	)

	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		logger.Fatal("listen failed", zap.Error(err))
	}
}
