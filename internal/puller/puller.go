package puller

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"
	"time"
)

// Puller executa "git pull" em um repositório, serializando as execuções
// para que dois pulls nunca rodem em paralelo.
type Puller struct {
	repoPath string
	remote   string
	branch   string
	log      *slog.Logger

	mu sync.Mutex
}

// New cria um Puller. branch vazio deixa o git usar o upstream configurado.
func New(repoPath, remote, branch string, log *slog.Logger) *Puller {
	return &Puller{
		repoPath: repoPath,
		remote:   remote,
		branch:   branch,
		log:      log,
	}
}

// Pull roda "git pull" uma vez. É seguro chamar de múltiplas goroutines: as
// chamadas concorrentes são serializadas pelo mutex.
func (p *Puller) Pull(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	args := []string{"-C", p.repoPath, "pull", "--ff-only", p.remote}
	if p.branch != "" {
		args = append(args, p.branch)
	}

	// Timeout de segurança para um pull não travar o serviço indefinidamente.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	before := p.headRev(ctx)
	start := time.Now()
	err := cmd.Run()
	output := bytes.TrimSpace(out.Bytes())
	after := p.headRev(ctx)

	if err != nil {
		p.log.Error("git pull falhou",
			"erro", err,
			"duracao", time.Since(start).String(),
			"saida", string(output),
		)
		return fmt.Errorf("git pull falhou: %w: %s", err, string(output))
	}

	// Comparar o HEAD antes/depois é robusto e independente do idioma do git:
	// só notificamos (nível Info) quando houve mudança de fato. Pulls sem
	// novidade ficam em Debug para não poluir o canal de log no modo polling.
	if before != after {
		p.log.Info("git pull aplicou novas mudanças",
			"de", short(before),
			"para", short(after),
			"duracao", time.Since(start).String(),
		)
	} else {
		p.log.Debug("repositório já estava atualizado",
			"duracao", time.Since(start).String(),
		)
	}
	return nil
}

// headRev retorna o hash do commit atual (HEAD), ou "" se não conseguir obter.
func (p *Puller) headRev(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "git", "-C", p.repoPath, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return string(bytes.TrimSpace(out))
}

func short(rev string) string {
	if len(rev) > 12 {
		return rev[:12]
	}
	return rev
}
