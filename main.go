package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/rnld-dev/pull-automator/internal/config"
	"github.com/rnld-dev/pull-automator/internal/logging"
	"github.com/rnld-dev/pull-automator/internal/notify/discord"
	"github.com/rnld-dev/pull-automator/internal/puller"
	"github.com/rnld-dev/pull-automator/internal/runner"
)

// version é injetada no build via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	// Handler base: sempre loga em JSON no stdout. Nível Debug para que os
	// pulls sem novidade apareçam no console (eles não vão para o Discord).
	baseHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	baseLog := slog.New(baseHandler)

	cfg, err := config.Load()
	if err != nil {
		baseLog.Error("configuração inválida", "erro", err)
		os.Exit(1)
	}

	// Serviço de log para o Discord (opcional). Quando ativo, o logger da
	// aplicação espelha para o Discord os registros de nível >= DiscordLevel.
	log := baseLog
	var sender *discord.Sender
	if cfg.DiscordEnabled() {
		// O sender registra seus próprios erros via baseLog (sem passar pelo
		// Discord), evitando loops de notificação.
		sender = discord.New(cfg.DiscordWebhookURL, cfg.DiscordUsername, baseLog)
		log = slog.New(logging.NewDiscordHandler(baseHandler, sender, cfg.DiscordLevel))
		baseLog.Info("serviço de log do Discord ativo", "nivel", cfg.DiscordLevel.String())
	}

	// Cancela o contexto nos sinais de término do SO (ver shutdownSignals,
	// definido por build tag para Unix e Windows) para shutdown gracioso.
	ctx, stop := signal.NotifyContext(context.Background(), shutdownSignals...)
	defer stop()

	p := puller.New(cfg.RepoPath, cfg.Remote, cfg.Branch, log)

	log.Info("pull-automator iniciando",
		"versao", version,
		"repo", cfg.RepoPath,
		"remote", cfg.Remote,
		"branch", orDefault(cfg.Branch, "(upstream)"),
		"modo", string(cfg.Mode),
	)

	switch cfg.Mode {
	case config.ModePolling:
		err = runner.RunPolling(ctx, p, cfg.PollInterval, log)
	case config.ModeWebhook:
		err = runner.RunWebhook(ctx, p, cfg.ListenAddr, cfg.WebhookPath, cfg.WebhookSecret, log)
	}

	if err != nil {
		log.Error("serviço encerrou com erro", "erro", err)
	}

	// Drena as notificações pendentes do Discord antes de sair.
	if sender != nil {
		flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		sender.Close(flushCtx)
		cancel()
	}

	if err != nil {
		os.Exit(1)
	}
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
