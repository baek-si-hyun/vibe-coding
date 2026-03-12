package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"investment-news-go/internal/app"
	"investment-news-go/internal/config"
	"investment-news-go/internal/ops"
	"investment-news-go/internal/server"
)

func main() {
	cfg := config.Load()
	application := app.New(cfg)
	handler := server.NewRouter(application)
	scheduler := ops.NewQuantAutoScheduler(cfg, application.Sync)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := cfg.Host + ":" + cfg.Port
	log.Printf("[backend-go] starting server at http://%s", addr)
	log.Printf("[backend-go] backend dir: %s", cfg.BackendDir)
	log.Printf("[backend-go] data root: %s", cfg.DataRootDir)
	if cfg.AutoQuantSync {
		go scheduler.Start(ctx)
	}

	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
