package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	RootDir    string
	FrontendDir string
	AuditFile  string
	Port       string
}

func Load() Config {
	rootDir, err := os.Getwd()
	if err != nil {
		rootDir = "."
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return Config{
		RootDir:     rootDir,
		FrontendDir: filepath.Join(rootDir, "frontend"),
		AuditFile:   filepath.Join(rootDir, "backend", "data", "audit-store.json"),
		Port:        port,
	}
}
