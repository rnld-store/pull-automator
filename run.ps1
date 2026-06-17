<#
.SYNOPSIS
    Configura as variaveis de ambiente e executa o pull-automator no Windows.

.DESCRIPTION
    Define as variaveis PA_* a partir dos parametros informados e roda o
    executavel. Parametros nao informados usam o valor ja presente no ambiente
    (ou o padrao do proprio servico).

.EXAMPLE
    .\run.ps1 -RepoPath C:\srv\meu-repo -Mode polling -PollInterval 30s

.EXAMPLE
    .\run.ps1 -RepoPath C:\srv\meu-repo -Mode webhook -WebhookSecret "segredo-forte" `
              -DiscordWebhookUrl "https://discord.com/api/webhooks/..."
#>

[CmdletBinding()]
param(
    # Caminho do repositorio git onde o "git pull" sera executado (obrigatorio).
    [Parameter(Mandatory = $true)]
    [string] $RepoPath,

    # Modo de operacao.
    [ValidateSet('polling', 'webhook')]
    [string] $Mode = 'polling',

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
    [string] $ExePath
)

$ErrorActionPreference = 'Stop'

# Resolve o caminho do executavel relativo ao script, se nao informado.
if (-not $ExePath) {
    $ExePath = Join-Path $PSScriptRoot 'pull-automator.exe'
}
if (-not (Test-Path -LiteralPath $ExePath)) {
    throw "Executavel nao encontrado: $ExePath. Gere-o com 'GOOS=windows go build -o pull-automator.exe .' no WSL."
}

# Define uma variavel de ambiente apenas quando o parametro foi informado,
# preservando o que ja estiver no ambiente caso contrario.
function Set-IfProvided([string] $Name, [string] $Value) {
    if (-not [string]::IsNullOrEmpty($Value)) {
        Set-Item -Path "Env:$Name" -Value $Value
    }
}

Set-IfProvided 'PA_REPO_PATH'          $RepoPath
Set-IfProvided 'PA_MODE'               $Mode
Set-IfProvided 'PA_REMOTE'             $Remote
Set-IfProvided 'PA_BRANCH'             $Branch
Set-IfProvided 'PA_POLL_INTERVAL'      $PollInterval
Set-IfProvided 'PA_LISTEN_ADDR'        $ListenAddr
Set-IfProvided 'PA_WEBHOOK_PATH'       $WebhookPath
Set-IfProvided 'PA_WEBHOOK_SECRET'     $WebhookSecret
Set-IfProvided 'PA_DISCORD_WEBHOOK_URL' $DiscordWebhookUrl
Set-IfProvided 'PA_DISCORD_USERNAME'   $DiscordUsername
Set-IfProvided 'PA_DISCORD_LEVEL'      $DiscordLevel

Write-Host "Iniciando pull-automator | repo=$env:PA_REPO_PATH modo=$env:PA_MODE" -ForegroundColor Cyan

& $ExePath
exit $LASTEXITCODE
