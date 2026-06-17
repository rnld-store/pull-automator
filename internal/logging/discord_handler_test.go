package logging

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rnld-dev/pull-automator/internal/notify/discord"
)

// newCountingSender devolve um Sender apontando para um servidor que conta as
// entregas recebidas.
func newCountingSender(t *testing.T, count *atomic.Int32) (*discord.Sender, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	s := discord.New(srv.URL, "bot", slog.New(slog.NewTextHandler(io.Discard, nil)))
	return s, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		s.Close(ctx)
		cancel()
		srv.Close()
	}
}

func TestHandlerForwardsByLevel(t *testing.T) {
	var count atomic.Int32
	sender, cleanup := newCountingSender(t, &count)
	defer cleanup()

	// Limiar em WARN: DEBUG e INFO não devem ir ao Discord; WARN e ERROR sim.
	h := NewDiscordHandler(slog.NewJSONHandler(io.Discard, nil), sender, slog.LevelWarn)
	log := slog.New(h)

	log.Debug("debug")
	log.Info("info")
	log.Warn("warn")
	log.Error("error")

	// Espera as entregas serem processadas pelo worker.
	deadline := time.Now().Add(2 * time.Second)
	for count.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if got := count.Load(); got != 2 {
		t.Errorf("entregas = %d, queria 2 (warn + error)", got)
	}
}
