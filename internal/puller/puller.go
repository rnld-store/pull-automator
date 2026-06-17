package puller

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rnld-dev/pull-automator/internal/logging"
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
		rev := short(after)
		subject := p.commitSubject(ctx, after)
		resources := p.changedResources(ctx, before, after)

		// Título no formato esperado pelo Discord, com o resumo do commit.
		title := "✅ Auto-pull OK " + rev
		if subject != "" {
			title += " - " + subject
		}

		// Corpo: lista de resources do servidor (FiveM/RedM) que precisam ser
		// reiniciados por terem sido alterados neste pull.
		var body string
		if len(resources) > 0 {
			body = "🔄 Restart pendente: " + strings.Join(resources, ", ")
		}

		p.log.Info(title,
			logging.MsgBodyKey, body,
			"de", short(before),
			"para", rev,
			"recursos", strings.Join(resources, ","),
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

// commitSubject retorna o assunto (primeira linha) do commit rev, ou "" se não
// conseguir obtê-lo.
func (p *Puller) commitSubject(ctx context.Context, rev string) string {
	out, err := exec.CommandContext(ctx, "git", "-C", p.repoPath, "log", "-1", "--format=%s", rev).Output()
	if err != nil {
		return ""
	}
	return string(bytes.TrimSpace(out))
}

// changedResources lista, em ordem alfabética, os resources do servidor
// (FiveM/RedM) afetados entre dois commits. Cada arquivo alterado é mapeado
// para o resource que o contém. O repositório pode apontar tanto para a raiz do
// servidor quanto para a pasta "resources" — o resource é identificado pelo seu
// manifesto (fxmanifest.lua/__resource.lua) no disco, então a profundidade do
// caminho e as pastas-categoria entre colchetes (ex.: "[scripts]") não importam.
func (p *Puller) changedResources(ctx context.Context, before, after string) []string {
	// core.quotepath=false mantém caracteres não-ASCII legíveis no output.
	out, err := exec.CommandContext(ctx, "git", "-C", p.repoPath,
		"-c", "core.quotepath=false", "diff", "--name-only", before, after).Output()
	if err != nil {
		p.log.Warn("não foi possível listar os resources alterados", "erro", err)
		return nil
	}

	seen := make(map[string]struct{})
	var resources []string
	for _, line := range strings.Split(string(out), "\n") {
		name := resourceName(p.repoPath, line)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		resources = append(resources, name)
	}
	sort.Strings(resources)
	return resources
}

// resourceName devolve o nome do resource ao qual o arquivo file (caminho
// relativo ao repositório, com "/" como separador, vindo do git) pertence. O
// resource é o diretório-ancestral mais raso que contém um manifesto do FiveM
// (fxmanifest.lua ou __resource.lua). Retorna "" quando o arquivo não está
// dentro de nenhum resource ou quando o resource foi removido do disco.
func resourceName(repoPath, file string) string {
	file = strings.TrimSpace(file)
	if file == "" {
		return ""
	}
	segments := strings.Split(file, "/")
	// Caminha do topo até (mas sem incluir) o próprio arquivo, procurando o
	// primeiro diretório-ancestral que seja a raiz de um resource.
	for i := 0; i < len(segments)-1; i++ {
		dir := filepath.Join(repoPath, filepath.Join(segments[:i+1]...))
		if isResourceDir(dir) {
			return segments[i]
		}
	}
	return ""
}

// isResourceDir indica se dir é a raiz de um resource do FiveM/RedM, isto é, se
// contém um manifesto fxmanifest.lua ou __resource.lua.
func isResourceDir(dir string) bool {
	for _, manifest := range []string{"fxmanifest.lua", "__resource.lua"} {
		if _, err := os.Stat(filepath.Join(dir, manifest)); err == nil {
			return true
		}
	}
	return false
}

func short(rev string) string {
	if len(rev) > 12 {
		return rev[:12]
	}
	return rev
}
