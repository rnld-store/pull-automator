package runner

import (
	"context"
	"log/slog"
	"time"

	"github.com/rnld-dev/pull-automator/internal/puller"
)

// RunPolling executa um pull imediatamente e depois a cada interval, até o
// contexto ser cancelado.
func RunPolling(ctx context.Context, p *puller.Puller, interval time.Duration, log *slog.Logger) error {
	log.Debug("iniciando modo polling", "intervalo", interval.String())

	// Pull inicial para já sincronizar ao subir. O resultado (sucesso/erro) já
	// é logado pelo puller, então aqui apenas ignoramos o retorno.
	_ = p.Pull(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("encerrando modo polling")
			return nil
		case <-ticker.C:
			_ = p.Pull(ctx)
		}
	}
}
