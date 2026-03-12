package server

import (
	"net/http"

	"investment-news-go/internal/app"
	"investment-news-go/internal/httpx"
	"investment-news-go/internal/news"
	"investment-news-go/internal/server/handlers"
)

func NewRouter(application *app.App) http.Handler {
	mux := http.NewServeMux()
	h := handlers.New(application)

	mux.HandleFunc("/", h.Health)
	mux.Handle("/api/news/", news.NewHandler(application.News))

	mux.HandleFunc("/api/endpoints", h.KRXEndpoints)
	mux.HandleFunc("/api/files", h.KRXFiles)
	mux.HandleFunc("/api/collect", h.KRXCollect)
	mux.HandleFunc("/api/collect/resume", h.KRXCollectResume)

	mux.HandleFunc("/api/quant/rank", h.QuantRank)
	mux.HandleFunc("/api/quant/report", h.QuantReport)
	mux.HandleFunc("/api/quant/macro", h.QuantMacro)

	mux.HandleFunc("/api/dart/export/financials", h.DartExportFinancials)

	mux.HandleFunc("/api/bithumb/screener", h.BithumbScreener)
	mux.HandleFunc("/api/upbit/screener", h.UpbitScreener)
	mux.HandleFunc("/api/telegram/chat-rooms", h.TelegramChatRooms)
	mux.HandleFunc("/api/telegram/items", h.TelegramItems)

	mux.HandleFunc("/api/", h.KRXDynamicFetch)

	return httpx.WithCORS(mux)
}
