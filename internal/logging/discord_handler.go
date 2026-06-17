// Package logging conecta o slog ao serviço de notificações do Discord.
package logging

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/rnld-dev/pull-automator/internal/notify/discord"
)

// DiscordHandler é um slog.Handler decorator: encaminha todo registro para o
// handler base (console) e, para registros de nível >= level, também enfileira
// uma notificação no Discord.
type DiscordHandler struct {
	next   slog.Handler
	sender *discord.Sender
	level  slog.Level
	attrs  []slog.Attr
	groups []string
}

// NewDiscordHandler envolve next, enviando ao sender os registros de nível
// >= level.
func NewDiscordHandler(next slog.Handler, sender *discord.Sender, level slog.Level) *DiscordHandler {
	return &DiscordHandler{next: next, sender: sender, level: level}
}

func (h *DiscordHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l) || l >= h.level
}

func (h *DiscordHandler) Handle(ctx context.Context, r slog.Record) error {
	var err error
	if h.next.Enabled(ctx, r.Level) {
		err = h.next.Handle(ctx, r)
	}
	if r.Level >= h.level {
		h.sender.Enqueue(h.toMessage(r))
	}
	return err
}

func (h *DiscordHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nh := h.clone()
	nh.next = h.next.WithAttrs(attrs)
	prefix := nh.groupPrefix()
	for _, a := range attrs {
		a.Key = prefix + a.Key
		nh.attrs = append(nh.attrs, a)
	}
	return nh
}

func (h *DiscordHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	nh := h.clone()
	nh.next = h.next.WithGroup(name)
	nh.groups = append(nh.groups, name)
	return nh
}

func (h *DiscordHandler) clone() *DiscordHandler {
	return &DiscordHandler{
		next:   h.next,
		sender: h.sender,
		level:  h.level,
		attrs:  append([]slog.Attr(nil), h.attrs...),
		groups: append([]string(nil), h.groups...),
	}
}

func (h *DiscordHandler) groupPrefix() string {
	if len(h.groups) == 0 {
		return ""
	}
	return strings.Join(h.groups, ".") + "."
}

// toMessage transforma o registro em uma mensagem do Discord: a mensagem do log
// vira o título e os atributos viram linhas "chave: valor" no corpo.
func (h *DiscordHandler) toMessage(r slog.Record) discord.Message {
	var b strings.Builder
	// Atributos acumulados via WithAttrs já têm o prefixo de grupo embutido.
	for _, a := range h.attrs {
		writeAttr(&b, "", a)
	}
	prefix := h.groupPrefix()
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(&b, prefix, a)
		return true
	})

	return discord.Message{
		Level: r.Level,
		Title: r.Message,
		Body:  strings.TrimRight(b.String(), "\n"),
		Time:  r.Time,
	}
}

func writeAttr(b *strings.Builder, prefix string, a slog.Attr) {
	if a.Equal(slog.Attr{}) {
		return
	}
	fmt.Fprintf(b, "%s%s: %s\n", prefix, a.Key, a.Value)
}
