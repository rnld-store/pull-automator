package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"
)

// Mode define o formato de funcionamento do serviço.
type Mode string

const (
	ModePolling Mode = "polling"
	ModeWebhook Mode = "webhook"
)

// Config reúne toda a configuração do serviço, lida exclusivamente de
// variáveis de ambiente (prefixo PA_).
type Config struct {
	// RepoPath é o caminho absoluto do repositório git onde o pull roda. Em
	// servidores FiveM/RedM, é o repositório do servidor — tanto a raiz que
	// contém a pasta "resources" quanto a própria pasta "resources".
	RepoPath string
	// Remote e Branch controlam o "git pull <remote> <branch>". Branch
	// vazio deixa o git resolver o upstream da branch atual.
	Remote string
	Branch string

	// Mode seleciona polling ou webhook.
	Mode Mode

	// PollInterval é o intervalo entre pulls no modo polling.
	PollInterval time.Duration

	// ListenAddr e WebhookPath valem para o modo webhook.
	ListenAddr  string
	WebhookPath string
	// WebhookSecret é o segredo compartilhado com o GitHub, usado para
	// validar a assinatura HMAC-SHA256 de cada entrega.
	WebhookSecret string

	// DiscordWebhookURL ativa o serviço de log para o Discord quando definido.
	DiscordWebhookURL string
	// DiscordUsername é o nome exibido nas mensagens do Discord.
	DiscordUsername string
	// DiscordLevel é o nível mínimo de log encaminhado ao Discord.
	DiscordLevel slog.Level
}

// DiscordEnabled indica se o serviço de log para o Discord está configurado.
func (c Config) DiscordEnabled() bool {
	return c.DiscordWebhookURL != ""
}

// Load lê e valida a configuração a partir do ambiente.
func Load() (Config, error) {
	cfg := Config{
		RepoPath:      strings.TrimSpace(os.Getenv("PA_REPO_PATH")),
		Remote:        getenvDefault("PA_REMOTE", "origin"),
		Branch:        strings.TrimSpace(os.Getenv("PA_BRANCH")),
		Mode:          Mode(strings.ToLower(getenvDefault("PA_MODE", string(ModePolling)))),
		ListenAddr:    getenvDefault("PA_LISTEN_ADDR", ":8080"),
		WebhookPath:   getenvDefault("PA_WEBHOOK_PATH", "/webhook"),
		WebhookSecret: os.Getenv("PA_WEBHOOK_SECRET"),

		DiscordWebhookURL: strings.TrimSpace(os.Getenv("PA_DISCORD_WEBHOOK_URL")),
		DiscordUsername:   getenvDefault("PA_DISCORD_USERNAME", "pull-automator"),
	}

	if err := cfg.DiscordLevel.UnmarshalText([]byte(strings.ToUpper(getenvDefault("PA_DISCORD_LEVEL", "INFO")))); err != nil {
		return Config{}, fmt.Errorf("PA_DISCORD_LEVEL inválido: %w", err)
	}
	if cfg.DiscordWebhookURL != "" {
		if u, err := url.Parse(cfg.DiscordWebhookURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return Config{}, fmt.Errorf("PA_DISCORD_WEBHOOK_URL inválido: precisa ser uma URL http(s)")
		}
	}

	if cfg.RepoPath == "" {
		return Config{}, fmt.Errorf("PA_REPO_PATH é obrigatório")
	}
	info, err := os.Stat(cfg.RepoPath)
	if err != nil {
		return Config{}, fmt.Errorf("PA_REPO_PATH inválido (%q): %w", cfg.RepoPath, err)
	}
	if !info.IsDir() {
		return Config{}, fmt.Errorf("PA_REPO_PATH não é um diretório: %q", cfg.RepoPath)
	}

	switch cfg.Mode {
	case ModePolling:
		interval, err := parseDuration("PA_POLL_INTERVAL", "60s")
		if err != nil {
			return Config{}, err
		}
		if interval <= 0 {
			return Config{}, fmt.Errorf("PA_POLL_INTERVAL deve ser maior que zero")
		}
		cfg.PollInterval = interval
	case ModeWebhook:
		if !strings.HasPrefix(cfg.WebhookPath, "/") {
			cfg.WebhookPath = "/" + cfg.WebhookPath
		}
		if cfg.WebhookSecret == "" {
			return Config{}, fmt.Errorf("PA_WEBHOOK_SECRET é obrigatório no modo webhook")
		}
	default:
		return Config{}, fmt.Errorf("PA_MODE inválido: %q (use %q ou %q)", cfg.Mode, ModePolling, ModeWebhook)
	}

	return cfg, nil
}

func getenvDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func parseDuration(key, def string) (time.Duration, error) {
	raw := getenvDefault(key, def)
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s inválido (%q): %w", key, raw, err)
	}
	return d, nil
}
