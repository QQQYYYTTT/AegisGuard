package main

import (
	"net/http"

	"aegisguard/internal/config"
	httpapi "aegisguard/internal/http"

	"go.uber.org/zap"
)

func main() {
	// 初始化 zap 日志
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	logger.Info("【启动】AegisGuard 网关正在启动...")

	cfg := config.Load()
	logger.Info("【启动】配置加载完成",
		zap.String("port", cfg.Port),
		zap.String("target_url", cfg.TargetURL),
		zap.String("vkey_config", cfg.VKeyConfigPath),
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
