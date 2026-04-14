package main

import (
	"log"
	"net/http"

	"aegisguard/backend/internal/config"
	httpapi "aegisguard/backend/internal/http"
)

func main() {
	cfg := config.Load()
	router, err := httpapi.NewRouter(cfg)
	if err != nil {
		log.Fatalf("build router: %v", err)
	}

	log.Printf("AegisGuard backend running at http://localhost:%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
