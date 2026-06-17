<#
.SYNOPSIS
    Configura (de forma interativa) as variaveis de ambiente e executa o
    pull-automator no Windows.

.DESCRIPTION
    Para cada configuracao, usa o valor passado por parametro; se nao houver e o
    modo interativo estiver ligado (padrao), pergunta ao usuario mostrando o
    valor padrao. Pergunta apenas o que faz sentido para o modo escolhido
    (polling x webhook) e trata o segredo do webhook com entrada mascarada.

    Use -NonInteractive para nao perguntar nada (ideal para automacao/servico):
    nesse caso valem os parametros e/ou as variaveis ja presentes no ambiente.

.EXAMPLE
    # Totalmente interativo: so rodar e responder as perguntas
    .\run.ps1

.EXAMPLE
    # Parcial: informa o repo, pergunta o resto
    .\run.ps1 -RepoPath C:\srv\meu-repo

.EXAMPLE
    # Sem prompts (automacao)
    .\run.ps1 -NonInteractive -RepoPath C:\srv\meu-repo -Mode webhook -WebhookSecret "segredo-forte"
#>

[CmdletBinding()]
param(
    [string] $RepoPath,

    [ValidateSet('polling', 'webhook')]
    [string] $Mode,

    [string] $Remote,
    [string] $Branch,

    # --- Modo polling ---
    [string] $PollInterval,

    # --- Modo webhook ---
    [string] $ListenAddr,
    [string] $WebhookPath,
    [string] $WebhookSecret,

    # --- Servico de log para o Discord (opcional) ---
    [string] $DiscordWebhookUrl,
    [string] $DiscordUsername,
    [ValidateSet('DEBUG', 'INFO', 'WARN', 'ERROR')]
    [string] $DiscordLevel,

    # Caminho do executavel. Padrao: pull-automator.exe ao lado deste script.
    [string] $ExePath,

    # Nao perguntar nada; usar apenas parametros/ambiente.
    [switch] $NonInteractive
)

$ErrorActionPreference = 'Stop'
$interactive = -not $NonInteractive

# ----------------------------------------------------------------------------
# Helpers de prompt
# ----------------------------------------------------------------------------

# Pergunta um valor, aceitando Enter para usar o padrao. Suporta lista de
# opcoes validas, campo obrigatorio e entrada mascarada (segredo).
function Read-Setting {
    param(
        [string]   $Prompt,
        [string]   $Default,
        [string[]] $Options,
        [switch]   $Required,
        [switch]   $Secret
    )
    while ($true) {
        $label = $Prompt
        if ($Options) { $label += " [$($Options -join '/')]" }
        if ($Default) { $label += " (padrao: $Default)" }

        if ($Secret) {
            $secure = Read-Host -Prompt $label -AsSecureString
            $value = [System.Net.NetworkCredential]::new('', $secure).Password
        }
        else {
            $value = Read-Host -Prompt $label
        }

        if ([string]::IsNullOrWhiteSpace($value)) { $value = $Default }

        if ([string]::IsNullOrWhiteSpace($value)) {
            if ($Required) { Write-Host '  -> valor obrigatorio.' -ForegroundColor Yellow; continue }
            return ''
        }
        if ($Options -and ($Options -notcontains $value)) {
            Write-Host "  -> opcao invalida. Escolha: $($Options -join ', ')" -ForegroundColor Yellow
            continue
        }
        return $value
    }
}

function Read-YesNo {
    param([string] $Prompt, [bool] $Default = $false)
    $suffix = if ($Default) { '[S/n]' } else { '[s/N]' }
    $ans = Read-Host -Prompt "$Prompt $suffix"
    if ([string]::IsNullOrWhiteSpace($ans)) { return $Default }
    return $ans -match '^(s|sim|y|yes)$'
}

# Resolve uma configuracao: parametro informado tem prioridade; senao pergunta
# (se interativo); senao usa o padrao.
function Resolve-Setting {
    param(
        [string]   $ParamName,
        [string]   $Prompt,
        [string]   $Default,
        [string[]] $Options,
        [switch]   $Required,
        [switch]   $Secret
    )
    if ($script:PSBoundParameters_All.ContainsKey($ParamName)) {
        return $script:PSBoundParameters_All[$ParamName]
    }
    if ($interactive) {
        return Read-Setting -Prompt $Prompt -Default $Default -Options $Options -Required:$Required -Secret:$Secret
    }
    if ($Required -and [string]::IsNullOrWhiteSpace($Default)) {
        throw "Parametro obrigatorio nao informado em modo -NonInteractive: -$ParamName"
    }
    return $Default
}

# ----------------------------------------------------------------------------
# Localiza o executavel
# ----------------------------------------------------------------------------

if (-not $ExePath) { $ExePath = Join-Path $PSScriptRoot 'pull-automator.exe' }
if (-not (Test-Path -LiteralPath $ExePath)) {
    throw "Executavel nao encontrado: $ExePath. Baixe-o da pagina de Releases do projeto e deixe ao lado deste script."
}

# Guarda os parametros realmente informados para a Resolve-Setting consultar.
$script:PSBoundParameters_All = $PSBoundParameters

# ----------------------------------------------------------------------------
# Coleta da configuracao
# ----------------------------------------------------------------------------

if ($interactive) {
    Write-Host ''
    Write-Host '=== Configuracao do pull-automator ===' -ForegroundColor Cyan
    Write-Host 'Pressione Enter para aceitar o valor padrao entre parenteses.' -ForegroundColor DarkGray
    Write-Host ''
}

$cfg = [ordered]@{}
$cfg['PA_REPO_PATH'] = Resolve-Setting -ParamName 'RepoPath' -Prompt 'Caminho do repositorio git' -Required
$cfg['PA_MODE']      = Resolve-Setting -ParamName 'Mode'     -Prompt 'Modo de operacao' -Default 'polling' -Options 'polling', 'webhook'
$cfg['PA_REMOTE']    = Resolve-Setting -ParamName 'Remote'   -Prompt 'Remote do git' -Default 'origin'
$cfg['PA_BRANCH']    = Resolve-Setting -ParamName 'Branch'   -Prompt 'Branch (vazio = upstream da branch atual)'

if ($cfg['PA_MODE'] -eq 'polling') {
    $cfg['PA_POLL_INTERVAL'] = Resolve-Setting -ParamName 'PollInterval' -Prompt 'Intervalo entre pulls (ex.: 30s, 5m, 1h)' -Default '60s'
}
else {
    $cfg['PA_LISTEN_ADDR']    = Resolve-Setting -ParamName 'ListenAddr'    -Prompt 'Endereco de escuta' -Default ':8080'
    $cfg['PA_WEBHOOK_PATH']   = Resolve-Setting -ParamName 'WebhookPath'   -Prompt 'Caminho do webhook' -Default '/webhook'
    $cfg['PA_WEBHOOK_SECRET'] = Resolve-Setting -ParamName 'WebhookSecret' -Prompt 'Segredo do webhook' -Required -Secret
}

# Discord (opcional). Se a URL veio por parametro, ja considera ativo; senao,
# em modo interativo pergunta se quer ativar.
$enableDiscord = $false
if ($PSBoundParameters.ContainsKey('DiscordWebhookUrl')) { $enableDiscord = $true }
elseif ($interactive) { $enableDiscord = Read-YesNo -Prompt 'Ativar log no Discord?' -Default $false }

if ($enableDiscord) {
    $cfg['PA_DISCORD_WEBHOOK_URL'] = Resolve-Setting -ParamName 'DiscordWebhookUrl' -Prompt 'URL do webhook do Discord' -Required
    $cfg['PA_DISCORD_USERNAME']    = Resolve-Setting -ParamName 'DiscordUsername'   -Prompt 'Nome exibido no Discord' -Default 'pull-automator'
    $cfg['PA_DISCORD_LEVEL']       = Resolve-Setting -ParamName 'DiscordLevel'      -Prompt 'Nivel minimo no Discord' -Default 'INFO' -Options 'DEBUG', 'INFO', 'WARN', 'ERROR'
}

# ----------------------------------------------------------------------------
# Resumo e confirmacao
# ----------------------------------------------------------------------------

Write-Host ''
Write-Host 'Configuracao:' -ForegroundColor Cyan
foreach ($k in $cfg.Keys) {
    if ([string]::IsNullOrEmpty($cfg[$k])) { continue }
    $shown = if ($k -like '*SECRET*' -or $k -like '*WEBHOOK_URL*') { '********' } else { $cfg[$k] }
    Write-Host ("  {0,-22} = {1}" -f $k, $shown)
}
Write-Host ''

if ($interactive -and -not (Read-YesNo -Prompt 'Iniciar com essa configuracao?' -Default $true)) {
    Write-Host 'Cancelado.' -ForegroundColor Yellow
    exit 1
}

# ----------------------------------------------------------------------------
# Aplica no ambiente e executa
# ----------------------------------------------------------------------------

foreach ($k in $cfg.Keys) {
    if (-not [string]::IsNullOrEmpty($cfg[$k])) { Set-Item -Path "Env:$k" -Value $cfg[$k] }
}

Write-Host ''
Write-Host "Iniciando pull-automator | repo=$($cfg['PA_REPO_PATH']) modo=$($cfg['PA_MODE'])" -ForegroundColor Green
Write-Host '(Ctrl+C para encerrar)' -ForegroundColor DarkGray
Write-Host ''

& $ExePath
exit $LASTEXITCODE
