# pull-automator

Serviço de papel único: manter um repositório git local sincronizado rodando
`git pull` automaticamente. Funciona em **Linux e Windows** (amd64/arm64), é um
binário único sem dependências (só precisa do `git` no `PATH`) e toda a
configuração vem de variáveis de ambiente.

Opera em um de dois modos:

- **polling** — executa `git pull` em intervalos regulares;
- **webhook** — sobe um servidor HTTP e dispara `git pull` a cada `push` no
  GitHub, validando a assinatura HMAC-SHA256 da entrega.

Opcionalmente, espelha os logs para um canal do **Discord** (ver
[Serviço de log para o Discord](#serviço-de-log-para-o-discord)).

---

## Sumário

- [Variáveis de ambiente](#variáveis-de-ambiente)
- [Baixando o binário (releases)](#baixando-o-binário-releases)
- [Rodando no Windows](#rodando-no-windows)
- [Rodando no Linux](#rodando-no-linux)
- [Configurando o webhook no GitHub](#configurando-o-webhook-no-github)
- [Serviço de log para o Discord](#serviço-de-log-para-o-discord)
- [Rodando como serviço](#rodando-como-serviço)

---

## Variáveis de ambiente

Toda a configuração é feita por variáveis de ambiente com prefixo `PA_`:

| Variável                 | Obrigatória | Padrão           | Descrição |
|--------------------------|-------------|------------------|-----------|
| `PA_REPO_PATH`           | sim         | —                | Caminho do repositório git onde o pull roda. |
| `PA_REMOTE`              | não         | `origin`         | Remote usado no `git pull`. |
| `PA_BRANCH`              | não         | _(upstream)_     | Branch do pull. Vazio usa o upstream da branch atual. |
| `PA_MODE`                | não         | `polling`        | `polling` ou `webhook`. |
| `PA_POLL_INTERVAL`       | polling     | `60s`            | Intervalo entre pulls (ex.: `30s`, `5m`, `1h`). |
| `PA_LISTEN_ADDR`         | webhook     | `:8080`          | Endereço/porta de escuta. |
| `PA_WEBHOOK_PATH`        | webhook     | `/webhook`       | Caminho do endpoint do webhook. |
| `PA_WEBHOOK_SECRET`      | webhook     | —                | Segredo compartilhado com o GitHub (valida a assinatura). |
| `PA_DISCORD_WEBHOOK_URL` | não         | —                | URL do webhook do Discord. Vazio desativa o serviço de log. |
| `PA_DISCORD_USERNAME`    | não         | `pull-automator` | Nome exibido nas mensagens do Discord. |
| `PA_DISCORD_LEVEL`       | não         | `INFO`           | Nível mínimo enviado ao Discord (`DEBUG`/`INFO`/`WARN`/`ERROR`). |

> O pull usa `--ff-only`: se houver alterações locais que impeçam um
> fast-forward, o pull falha (e é logado) em vez de criar merges inesperados.
> O repositório precisa já estar clonado e com autenticação configurada
> (deploy key/SSH ou token no remote) — o serviço só roda o `pull`.

---

## Baixando o binário (releases)

Cada versão publicada gera binários prontos na aba **Releases** do repositório
no GitHub:

1. Acesse **Releases** (no menu lateral direito da página do repositório) e abra
   a versão mais recente.
2. Em **Assets**, baixe o pacote do seu sistema:
   - Windows: `pull-automator_vX.Y.Z_windows_amd64.zip` (ou `arm64`);
   - Linux: `pull-automator_vX.Y.Z_linux_amd64.tar.gz` (ou `arm64`).
3. Extraia o arquivo. Dentro vêm o executável, o `README.md` e o `.env.example`
   (no Windows, também o `run.ps1` e o `run.cmd`).
4. Confira a integridade com o `SHA256SUMS.txt` da release, se quiser.

> Mantenedores: para publicar uma nova versão, basta criar e empurrar uma tag
> `vX.Y.Z` — o GitHub Actions builda e publica a release automaticamente.

---

## Rodando no Windows

Não precisa instalar Go: baixe o binário da release (ver acima) e rode. O único
pré-requisito é ter o **Git for Windows** instalado (o `git` precisa estar no
`PATH`).

As variáveis de ambiente são definidas na sessão do terminal e o executável é
chamado em seguida.

### PowerShell

```powershell
# Polling a cada 30s
$env:PA_REPO_PATH  = "C:\srv\meu-repo"
$env:PA_MODE       = "polling"
$env:PA_POLL_INTERVAL = "30s"
.\pull-automator.exe

# Webhook
$env:PA_REPO_PATH     = "C:\srv\meu-repo"
$env:PA_MODE          = "webhook"
$env:PA_WEBHOOK_SECRET = "um-segredo-forte"
.\pull-automator.exe
```

Há também o script `run.ps1` (incluído no pacote Windows), que é **interativo**:
rode sem argumentos e ele pergunta cada configuração (com valor padrão e
validação), perguntando só o que faz sentido para o modo escolhido e mascarando
o segredo do webhook:

```powershell
.\run.ps1
```

Você também pode passar valores por parâmetro — eles pulam o prompt
correspondente — e usar `-NonInteractive` para não perguntar nada (automação):

```powershell
# informa o repo, pergunta o resto
.\run.ps1 -RepoPath C:\srv\meu-repo

# sem prompts
.\run.ps1 -NonInteractive -RepoPath C:\srv\meu-repo -Mode webhook -WebhookSecret "segredo-forte"
```

> **"não está assinado digitalmente" / execution policy?** Como o `run.ps1` veio
> de um `.zip` baixado, o Windows o marca como "da internet" e o PowerShell
> recusa scripts não assinados. Soluções:
> - use o **`run.cmd`** (incluído no pacote) — ele chama o `run.ps1` já com a
>   política contornada; arquivos `.cmd` não sofrem essa restrição;
> - ou rode uma vez com `powershell -ExecutionPolicy Bypass -File .\run.ps1`;
> - ou libere de vez: `Unblock-File .\run.ps1` e
>   `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned`;
> - ou simplesmente chame o `.exe` direto (não sofre essa restrição).

### CMD (Prompt de Comando)

```bat
:: Polling a cada 30s
set PA_REPO_PATH=C:\srv\meu-repo
set PA_MODE=polling
set PA_POLL_INTERVAL=30s
pull-automator.exe

:: Webhook
set PA_REPO_PATH=C:\srv\meu-repo
set PA_MODE=webhook
set PA_WEBHOOK_SECRET=um-segredo-forte
pull-automator.exe
```

> No `cmd`, **não** use aspas nos valores do `set` (elas viram parte do valor).
> As variáveis valem só para a janela atual do terminal.

Para encerrar, use `Ctrl+C` (shutdown gracioso).

---

## Rodando no Linux

Baixe e extraia o pacote `.tar.gz` da release. Requisito: `git` instalado.

```sh
tar -xzf pull-automator_vX.Y.Z_linux_amd64.tar.gz
chmod +x pull-automator

# Polling a cada 30s
PA_REPO_PATH=/srv/meu-repo PA_MODE=polling PA_POLL_INTERVAL=30s ./pull-automator

# Webhook
PA_REPO_PATH=/srv/meu-repo PA_MODE=webhook \
  PA_WEBHOOK_SECRET=um-segredo-forte ./pull-automator
```

### Docker

```sh
docker build -t pull-automator .
docker run --rm \
  -e PA_REPO_PATH=/repo -e PA_MODE=webhook -e PA_WEBHOOK_SECRET=... \
  -v /srv/meu-repo:/repo -p 8080:8080 pull-automator
```

---

## Configurando o webhook no GitHub

Necessário apenas no modo `webhook`. No repositório do GitHub, vá em
**Settings → Webhooks → Add webhook** e preencha:

| Campo | Valor |
|-------|-------|
| **Payload URL** | `https://SEU-HOST:8080/webhook` (host público onde o serviço escuta). |
| **Content type** | `application/json`. |
| **Secret** | O mesmo valor de `PA_WEBHOOK_SECRET`. |
| **SSL verification** | Mantenha **Enable** (use HTTPS — ver [Segurança](#segurança)). |
| **Which events?** | Marque **Just the `push` event**. |
| **Active** | Marcado. |

Clique em **Add webhook**. O GitHub envia um evento `ping` na hora — o serviço
responde `pong` (a entrega aparece com ✅ em **Recent Deliveries**).

A partir daí, cada `push` no repositório dispara um `git pull`. O serviço:

- só reage a eventos `push` (responde `pong` ao `ping` e ignora os demais);
- rejeita com **`401`** qualquer entrega sem assinatura HMAC válida;
- expõe `GET /healthz` → `200 ok` (útil para liveness/readiness probes).

### Expondo o serviço com HTTPS

O GitHub precisa alcançar a Payload URL pela internet, e ela deve ser **HTTPS**
(ver [Segurança](#segurança)). O serviço fala HTTP puro na porta local, então o
TLS fica a cargo de um túnel ou proxy na frente. Opções comuns:

**Túneis (ideais para máquinas atrás de NAT, sem IP público ou porta aberta):**

- **[ngrok](https://ngrok.com/)** — rápido para testar. `ngrok http 8080` gera
  uma URL `https://...ngrok-free.app` que você usa como Payload URL. No plano
  gratuito a URL muda a cada execução (use um domínio reservado para fixá-la).
- **[Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/)**
  (`cloudflared`) — gratuito e estável, com domínio próprio. Bom para produção.
- **[Tailscale Funnel](https://tailscale.com/kb/1223/funnel)** — expõe um serviço
  da sua tailnet na internet com HTTPS, sem abrir portas.
- **[localtunnel](https://github.com/localtunnel/localtunnel)** — alternativa
  simples e open-source ao ngrok.

**Proxy reverso (quando a máquina já tem IP público/domínio):**

- **[Caddy](https://caddyserver.com/)** — HTTPS automático via Let's Encrypt com
  uma linha de config (`reverse_proxy localhost:8080`).
- **nginx** ou **Traefik** — com certificado do Let's Encrypt (certbot/ACME).

> Em qualquer opção, aponte o túnel/proxy para `localhost:8080` (ou o que estiver
> em `PA_LISTEN_ADDR`) e use a URL HTTPS pública como Payload URL no GitHub.

---

## Serviço de log para o Discord

Definindo `PA_DISCORD_WEBHOOK_URL`, os logs passam a ser espelhados para um canal
do Discord (além do stdout em JSON). O envio é assíncrono e resiliente: roda numa
fila com worker próprio, então não atrasa o `git pull` e uma indisponibilidade do
Discord não derruba o serviço — no máximo descarta notificações se a fila encher.

Cada mensagem vira um *embed* colorido por nível (verde = info, amarelo = warn,
vermelho = error). Com o padrão `PA_DISCORD_LEVEL=INFO`, você recebe:

- ✅ **pull com mudanças aplicadas** (com o hash de/para);
- ❌ **falha no `git pull`** (com a saída do git);
- ⚠️ **assinatura de webhook inválida** (alerta de segurança, no modo webhook);
- início e encerramento do serviço.

Pulls do modo polling que **não** trazem novidade ficam em nível `DEBUG` e
**não** notificam o Discord — evitando spam. A detecção de mudança compara o
`HEAD` antes/depois do pull, então independe do idioma configurado no git.

Para criar o webhook do Discord: **Configurações do canal → Integrações →
Webhooks → Novo webhook**, copie a URL e use em `PA_DISCORD_WEBHOOK_URL`.

---

## Rodando como serviço

- **Linux**: unidade systemd com as variáveis em `Environment=` ou
  `EnvironmentFile=` apontando para um arquivo no formato do `.env.example`.
- **Windows**: registre o `.exe` como serviço com [NSSM](https://nssm.cc/) ou
  `sc.exe create`, definindo as variáveis de ambiente do serviço.
