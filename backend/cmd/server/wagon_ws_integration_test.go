package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/handler"
	"github.com/mostransport/vsm-trainer/internal/repo/postgres"
	"github.com/mostransport/vsm-trainer/internal/service"
)

func TestWagonWebSocketEndToEnd(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	store, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	catalog, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	classes, err := content.LoadWagonClasses()
	if err != nil {
		t.Fatal(err)
	}
	auth := service.NewAuthService(store, "wagon-ws-test-secret", time.Hour, time.Hour)
	register := func() (uuid.UUID, string) {
		result, err := auth.Register(ctx, fmt.Sprintf("ws-%s@example.invalid", uuid.NewString()), "ws-test", "testpass123")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			conn, err := pgx.Connect(context.Background(), dsn)
			if err == nil {
				_, _ = conn.Exec(context.Background(), `DELETE FROM players WHERE id = $1`, result.Player.ID)
				_ = conn.Close(context.Background())
			}
		})
		return result.Player.ID, result.Tokens.AccessToken
	}
	_, token := register()
	_, foreignToken := register()
	mgr := service.NewWagonManager(store, catalog)
	defer mgr.StopAll()
	h := &handler.Handlers{Auth: auth, Wagon: service.NewWagonService(store, catalog, classes, mgr), WagonManager: mgr, WagonClasses: classes}
	server := httptest.NewServer(routes(h, auth, store))
	defer server.Close()
	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/session/wagon/start", strings.NewReader(`{"class_id":"standard"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("start status: %d", response.StatusCode)
	}
	var started struct {
		SessionID uuid.UUID `json:"session_id"`
		WSPath    string    `json:"ws_path"`
	}
	if err := json.NewDecoder(response.Body).Decode(&started); err != nil {
		t.Fatal(err)
	}
	if started.SessionID == uuid.Nil || started.WSPath == "" {
		t.Fatalf("start response: %+v", started)
	}
	wsBase := "ws" + strings.TrimPrefix(server.URL, "http") + started.WSPath
	for _, tc := range []struct {
		token string
		want  int
	}{{"invalid", http.StatusUnauthorized}, {foreignToken, http.StatusNotFound}} {
		conn, res, err := websocket.DefaultDialer.Dial(wsBase+"?token="+url.QueryEscape(tc.token), nil)
		if conn != nil {
			_ = conn.Close()
		}
		if err == nil || res == nil || res.StatusCode != tc.want {
			t.Fatalf("unauthorized WS: response=%v err=%v want=%d", res, err, tc.want)
		}
		_ = res.Body.Close()
	}
	conn, res, err := websocket.DefaultDialer.Dial(wsBase+"?token="+url.QueryEscape(token), nil)
	if err != nil {
		t.Fatalf("WS connect: response=%v err=%v", res, err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var state struct {
		Type       string `json:"type"`
		WagonState struct {
			ClassID      string   `json:"class_id"`
			CarriedItems []string `json:"carried_items"`
		} `json:"wagon_state"`
	}
	if err := conn.ReadJSON(&state); err != nil {
		t.Fatal(err)
	}
	if state.Type != "state" || state.WagonState.ClassID != "standard" {
		t.Fatalf("initial snapshot: %+v", state)
	}
	if err := conn.WriteJSON(map[string]string{"type": "pick_item", "item": "blanket"}); err != nil {
		t.Fatal(err)
	}
	for {
		if err := conn.ReadJSON(&state); err != nil {
			t.Fatal(err)
		}
		if state.Type == "state" && len(state.WagonState.CarriedItems) > 0 {
			break
		}
	}
	if state.WagonState.CarriedItems[0] != "blanket" {
		t.Fatalf("pick item snapshot: %+v", state)
	}
}
