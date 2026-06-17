package discord

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBuildPayload(t *testing.T) {
	m := Message{Level: slog.LevelError, Title: "falhou", Body: "erro: x", Time: time.Unix(0, 0)}
	p := buildPayload("bot", m)

	if p["username"] != "bot" {
		t.Errorf("username = %v", p["username"])
	}
	embeds := p["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	if embed["title"] != "falhou" {
		t.Errorf("title = %v", embed["title"])
	}
	if embed["description"] != "erro: x" {
		t.Errorf("description = %v", embed["description"])
	}
	if embed["color"] != 0xE01E5A {
		t.Errorf("color = %v, queria vermelho para erro", embed["color"])
	}
}

func TestSenderPostsToWebhook(t *testing.T) {
	received := make(chan map[string]any, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		received <- payload
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	s := New(srv.URL, "bot", slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.Enqueue(Message{Level: slog.LevelInfo, Title: "oi", Time: time.Now()})

	select {
	case payload := <-received:
		embeds := payload["embeds"].([]any)
		embed := embeds[0].(map[string]any)
		if embed["title"] != "oi" {
			t.Errorf("title = %v", embed["title"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("webhook não recebeu a mensagem")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.Close(ctx)
}

func TestTruncate(t *testing.T) {
	if got := truncate("abcdef", 3); got != "abc…" {
		t.Errorf("truncate = %q", got)
	}
	if got := truncate("ab", 3); got != "ab" {
		t.Errorf("truncate = %q", got)
	}
}
