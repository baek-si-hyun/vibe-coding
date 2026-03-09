package main

import (
	"log"
	"net/http"

	"investment-news-go/internal/app"
	"investment-news-go/internal/config"
	"investment-news-go/internal/server"
)

func main() {
	cfg := config.Load()
	application := app.New(cfg)
	handler := server.NewRouter(application)

	addr := cfg.Host + ":" + cfg.Port
	log.Printf("[backend-go] starting server at http://%s", addr)
	log.Printf("[backend-go] backend dir: %s", cfg.BackendDir)
	log.Printf("[backend-go] data root: %s", cfg.DataRootDir)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
