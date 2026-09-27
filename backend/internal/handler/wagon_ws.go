package handler

import (
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/service"
)

var wagonUpgrader = websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024, CheckOrigin: wagonOriginAllowed}

func wagonOriginAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if parsed, err := url.Parse(origin); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host == r.Host {
		return true
	}
	allowed := os.Getenv("CORS_ORIGINS")
	if allowed == "" {
		allowed = "http://localhost:8081,http://localhost:19006,http://127.0.0.1:8081,http://127.0.0.1:19006"
	}
	for _, candidate := range strings.Split(allowed, ",") {
		if strings.TrimSpace(candidate) == origin {
			return true
		}
	}
	return false
}

type wagonClientMessage struct {
	Type        string `json:"type"`
	Item        string `json:"item,omitempty"`
	Anchor      string `json:"anchor,omitempty"`
	SituationID string `json:"situation_id,omitempty"`
}

func (h *Handlers) WagonWS(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	playerID, _, err := h.Auth.ParseAccess(r.URL.Query().Get("token"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}
	sess, err := h.Wagon.GetOwnedSession(r.Context(), playerID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if sess.Status != domain.SessionStatusActive {
		writeError(w, http.StatusConflict, "session is finished")
		return
	}
	conn, err := wagonUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(2048)
	if !h.WagonManager.Attach(sess.ID, conn) {
		_ = conn.Close()
		return
	}
	defer func() { h.WagonManager.Detach(sess.ID, conn); _ = conn.Close() }()
	for {
		var msg wagonClientMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		situationID, err := uuid.Parse(msg.SituationID)
		if msg.Type == "give_item" || msg.Type == "redirect" {
			if err != nil {
				return
			}
		} else {
			situationID = uuid.Nil
		}
		cmd := service.NewWagonCommand(msg.Type, msg.Item, msg.Anchor, situationID)
		if err := h.WagonManager.Dispatch(r.Context(), sess.ID, cmd); err != nil {
			if err == r.Context().Err() {
				return
			}
		}
	}
}
