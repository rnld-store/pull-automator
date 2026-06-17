// Package discord envia notificações a um canal do Discord via webhook.
//
// O envio é assíncrono: as mensagens entram numa fila e um worker em background
// as despacha. Assim o git pull nunca trava por causa do Discord, e uma
// indisponibilidade do Discord não derruba o serviço (no máximo descarta
// notificações quando a fila enche).
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	queueSize   = 256
	sendTimeout = 10 * time.Second
	// maxDescLen limita a descrição do embed (limite do Discord é 4096).
	maxDescLen = 1800
)

// Message é uma notificação a ser enviada.
type Message struct {
	Level slog.Level
	Title string
	Body  string
	Time  time.Time
}

// Sender despacha mensagens para um webhook do Discord.
type Sender struct {
	url      string
	username string
	client   *http.Client
	queue    chan Message
	// diag registra erros internos do próprio Sender. NÃO deve passar pelo
	// handler do Discord, sob pena de criar um loop de notificações.
	diag *slog.Logger
	wg   sync.WaitGroup
}

// New cria um Sender e inicia seu worker. diag recebe erros internos e nunca
// deve estar conectado ao Discord.
func New(url, username string, diag *slog.Logger) *Sender {
	s := &Sender{
		url:      url,
		username: username,
		client:   &http.Client{Timeout: sendTimeout},
		queue:    make(chan Message, queueSize),
		diag:     diag,
	}
	s.wg.Add(1)
	go s.worker()
	return s
}

// Enqueue coloca a mensagem na fila sem bloquear. Se a fila estiver cheia, a
// notificação é descartada (e logada via diag).
func (s *Sender) Enqueue(m Message) {
	select {
	case s.queue <- m:
	default:
		s.diag.Warn("fila do Discord cheia, descartando notificação", "titulo", m.Title)
	}
}

func (s *Sender) worker() {
	defer s.wg.Done()
	for m := range s.queue {
		if err := s.send(m); err != nil {
			s.diag.Error("falha ao enviar notificação ao Discord", "erro", err)
		}
	}
}

func (s *Sender) send(m Message) error {
	buf, err := json.Marshal(buildPayload(s.username, m))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord retornou status %d", resp.StatusCode)
	}
	return nil
}

// Close fecha a fila e aguarda o worker drenar as mensagens pendentes, ou até
// o contexto expirar.
func (s *Sender) Close(ctx context.Context) {
	close(s.queue)
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

// buildPayload monta o corpo JSON do webhook, usando um embed colorido por nível.
func buildPayload(username string, m Message) map[string]any {
	embed := map[string]any{
		"title":     truncate(m.Title, 256),
		"color":     colorFor(m.Level),
		"timestamp": m.Time.UTC().Format(time.RFC3339),
		"footer":    map[string]any{"text": m.Level.String()},
	}
	if body := truncate(m.Body, maxDescLen); body != "" {
		embed["description"] = body
	}

	payload := map[string]any{"embeds": []any{embed}}
	if username != "" {
		payload["username"] = username
	}
	return payload
}

func colorFor(l slog.Level) int {
	switch {
	case l >= slog.LevelError:
		return 0xE01E5A // vermelho
	case l >= slog.LevelWarn:
		return 0xECB22E // amarelo
	default:
		return 0x2EB67D // verde
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
