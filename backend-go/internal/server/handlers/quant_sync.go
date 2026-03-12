package handlers

import (
	"log"
	"net/http"
)

func (h *Handlers) ensureQuantSync(r *http.Request) {
	if h == nil || h.app == nil || h.app.Sync == nil {
		return
	}
	if !h.app.Config.AutoQuantRequestSync {
		return
	}
	if _, err := h.app.Sync.EnsureQuantInputsForRequest(r.Context()); err != nil {
		log.Printf("quant request sync warning: %v", err)
	}
}
