package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"aegisguard/internal/auth"
	"aegisguard/internal/config"
	httpapi "aegisguard/internal/http"
	"aegisguard/internal/mcpbridge"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	cfg := config.Load()
	logger := buildLogger(cfg)
	defer logger.Sync()

	if err := auth.InitSigningKey(cfg.SigningPrivateKey); err != nil {
		logger.Fatal("failed to initialize RequireToken signing key", zap.Error(err))
	}

	auth.StartNonceGC(time.Hour)
	defer auth.StopNonceGC()

	args := os.Args[1:]
	if len(args) == 0 || isServerCommand(args[0]) {
		runServer(cfg, logger)
		return
	}

	switch args[0] {
	case "bridge-stdio":
		if err := runBridge(cfg, logger, args[1:]); err != nil {
			logger.Fatal("bridge-stdio failed", zap.Error(err))
		}
	default:
		logger.Fatal("unknown subcommand", zap.String("subcommand", args[0]))
	}
}

func runServer(cfg config.Config, logger *zap.Logger) {
	logger.Info("starting AegisGuard gateway",
		zap.String("port", cfg.Port),
		zap.String("gateway_config", cfg.GatewayConfigPath),
		zap.String("token_mode", cfg.TokenMode),
	)

	router, err := httpapi.NewRouter(cfg)
	if err != nil {
		logger.Fatal("failed to build router", zap.Error(err))
	}

	logger.Info("AegisGuard gateway started", zap.String("url", "http://localhost:"+cfg.Port))
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		logger.Fatal("listen failed", zap.Error(err))
	}
}

func runBridge(cfg config.Config, logger *zap.Logger, args []string) error {
	fs := flag.NewFlagSet("bridge-stdio", flag.ContinueOnError)
	backendURL := fs.String("backend", "http://127.0.0.1:"+cfg.Port, "AegisGuard backend base URL")
	bridgeKey := fs.String("bridge-key", cfg.BridgeSharedKey, "shared key for bridge control-plane auth")
	agentID := fs.String("agent-id", "agent-bridge", "agent identifier")
	sessionID := fs.String("session-id", fmt.Sprintf("session-%d", time.Now().UnixNano()), "session identifier")
	taskID := fs.String("task-id", fmt.Sprintf("task-%d", time.Now().UnixNano()), "task identifier")
	if err := fs.Parse(args); err != nil {
		return err
	}
	command := fs.Args()
	if len(command) > 0 && command[0] == "--" {
		command = command[1:]
	}
	bridge, err := mcpbridge.New(mcpbridge.Config{
		BackendURL: *backendURL,
		BridgeKey:  *bridgeKey,
		AgentID:    *agentID,
		SessionID:  *sessionID,
		TaskID:     *taskID,
		Command:    command,
	})
	if err != nil {
		return err
	}
	logger.Info("starting stdio MCP bridge",
		zap.String("backend", *backendURL),
		zap.String("agent_id", *agentID),
	)
	return bridge.Run(context.Background())
}

func buildLogger(cfg config.Config) *zap.Logger {
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
	logger.Info("startup log configured", zap.String("encoding", cfg.LogEncoding))
	return logger
}

func isServerCommand(arg string) bool {
	return arg == "server"
}
