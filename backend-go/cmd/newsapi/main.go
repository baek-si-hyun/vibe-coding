package main

import (
	"fmt"
	"log"
	"net/http"

	"investment-news-go/internal/config"
	"investment-news-go/internal/httpx"
	"investment-news-go/internal/news"
)

func main() {
	cfg := config.Load()
	service := news.NewService(cfg)
	handler := httpx.WithCORS(news.NewHandler(service))

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	log.Printf("[news-go] starting server at http://%s", addr)
	log.Printf("[news-go] backend dir: %s", cfg.BackendDir)
	log.Printf("[news-go] data dir: %s", cfg.DataDir)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
