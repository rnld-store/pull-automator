package runner

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rnld-dev/pull-automator/internal/puller"
)

// maxBodyBytes limita o tamanho do payload aceito para evitar abuso.
const maxBodyBytes = 5 << 20 // 5 MiB

// RunWebhook sobe um servidor HTTP que dispara um git pull a cada entrega
// válida do GitHub. Bloqueia até o contexto ser cancelado.
func RunWebhook(ctx context.Context, p *puller.Puller, addr, path, secret string, log *slog.Logger) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("POST "+path, webhookHandler(p, secret, log))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Debug("iniciando modo webhook", "addr", addr, "path", path)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("encerrando modo webhook")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func webhookHandler(p *puller.Puller, secret string, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
		if err != nil {
			http.Error(w, "falha ao ler corpo", http.StatusBadRequest)
			return
		}

		if !validSignature(secret, r.Header.Get("X-Hub-Signature-256"), body) {
			log.Warn("assinatura de webhook inválida", "remote", r.RemoteAddr)
			http.Error(w, "assinatura inválida", http.StatusUnauthorized)
			return
		}

		event := r.Header.Get("X-GitHub-Event")
		switch event {
		case "ping":
			// GitHub manda "ping" ao cadastrar o webhook; respondemos sem pull.
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "pong\n")
			return
		case "push":
			// segue para o pull
		default:
			// Evento que não nos interessa: aceita mas ignora.
			log.Debug("evento ignorado", "evento", event)
			w.WriteHeader(http.StatusAccepted)
			return
		}

		// O resultado do pull (com/sem mudança ou erro) é logado pelo puller.
		log.Debug("push recebido, disparando git pull", "evento", event)
		if err := p.Pull(r.Context()); err != nil {
			http.Error(w, "git pull falhou", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "pulled\n")
	}
}

// validSignature confere o header X-Hub-Signature-256 ("sha256=<hex>")
// usando comparação em tempo constante para evitar timing attacks.
func validSignature(secret, header string, body []byte) bool {
	if secret == "" || header == "" {
		return false
	}
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	want, err := hex.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	got := mac.Sum(nil)

	return hmac.Equal(got, want)
}
