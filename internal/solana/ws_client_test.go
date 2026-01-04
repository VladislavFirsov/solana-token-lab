package solana

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func TestWSClient_Connect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close()

		// Keep connection open
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx := context.Background()
	client, err := NewWSClient(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("NewWSClient: %v", err)
	}
	defer client.Close()

	if client.closed.Load() {
		t.Error("client should not be closed")
	}
}

func TestWSClient_SubscribeLogs(t *testing.T) {
	var mu sync.Mutex
	var serverConn *websocket.Conn

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		mu.Lock()
		serverConn = c
		_ = serverConn // suppress unused warning
		mu.Unlock()
		defer c.Close()

		// Read subscribe request
		_, msg, err := c.ReadMessage()
		if err != nil {
			return
		}

		var req wsRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			t.Errorf("unmarshal request: %v", err)
			return
		}

		if req.Method != "logsSubscribe" {
			t.Errorf("expected logsSubscribe, got %s", req.Method)
		}

		// Send subscription confirmation
		resp := wsSubscribeResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  12345, // subscription ID
		}
		if err := c.WriteJSON(resp); err != nil {
			t.Errorf("write response: %v", err)
			return
		}

		// Send a log notification
		time.Sleep(50 * time.Millisecond)
		notif := wsNotification{
			JSONRPC: "2.0",
			Method:  "logsNotification",
			Params: &wsNotificationParams{
				Subscription: 12345,
				Result: wsNotificationResult{
					Context: &wsContext{Slot: 100},
					Value: wsLogsValue{
						Signature: "testsig",
						Logs:      []string{"Program log: Test"},
						Err:       nil,
					},
				},
			},
		}
		if err := c.WriteJSON(notif); err != nil {
			t.Errorf("write notification: %v", err)
			return
		}

		// Keep connection open
		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx := context.Background()
	client, err := NewWSClient(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("NewWSClient: %v", err)
	}
	defer client.Close()

	ch, err := client.SubscribeLogs(ctx, LogsFilter{
		Mentions: []string{"testprogram"},
	})
	if err != nil {
		t.Fatalf("SubscribeLogs: %v", err)
	}

	// Wait for notification
	select {
	case notif := <-ch:
		if notif.Signature != "testsig" {
			t.Errorf("expected testsig, got %s", notif.Signature)
		}
		if len(notif.Logs) != 1 {
			t.Errorf("expected 1 log, got %d", len(notif.Logs))
		}
		if notif.Slot != 100 {
			t.Errorf("expected slot 100, got %d", notif.Slot)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for notification")
	}
}

func TestWSClient_Close(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx := context.Background()
	client, err := NewWSClient(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("NewWSClient: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Errorf("Close: %v", err)
	}

	if !client.closed.Load() {
		t.Error("client should be closed")
	}

	// Double close should be safe
	err = client.Close()
	if err != nil {
		t.Errorf("double Close: %v", err)
	}
}

func TestWSClient_SubscribeAfterClose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx := context.Background()
	client, err := NewWSClient(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("NewWSClient: %v", err)
	}

	client.Close()

	_, err = client.SubscribeLogs(ctx, LogsFilter{})
	if err == nil {
		t.Error("expected error subscribing after close")
	}
}

func TestWSClient_CustomConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	config := &WSClientConfig{
		ReconnectDelay:    100 * time.Millisecond,
		MaxReconnectDelay: 1 * time.Second,
		PingInterval:      5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	ctx := context.Background()
	client, err := NewWSClient(ctx, wsURL, config)
	if err != nil {
		t.Fatalf("NewWSClient: %v", err)
	}
	defer client.Close()

	if client.config.PingInterval != 5*time.Second {
		t.Errorf("expected PingInterval 5s, got %v", client.config.PingInterval)
	}
}
