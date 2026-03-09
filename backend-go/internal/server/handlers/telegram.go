package handlers

import (
	"net/http"

	"investment-news-go/internal/httpx"
)

func (h *Handlers) TelegramChatRooms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"chatRooms": h.app.Telegram.ChatRooms()})
}

func (h *Handlers) TelegramItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
		return
	}

	q := r.URL.Query()
	page := httpx.ToInt(q.Get("page"), 1)
	limit := httpx.ToInt(q.Get("limit"), 50)
	if limit > 100 {
		limit = 100
	}
	search := q.Get("q")
	if search == "" {
		search = q.Get("search")
	}
	chat := q.Get("chat")

	httpx.WriteJSON(w, http.StatusOK, h.app.Telegram.Items(page, limit, search, chat))
}
