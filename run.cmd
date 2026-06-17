@echo off
REM Atalho para rodar o run.ps1 sem esbarrar na execution policy do PowerShell.
REM Arquivos .cmd nao sofrem essa restricao; aqui chamamos o PowerShell com
REM -ExecutionPolicy Bypass apontando para o run.ps1 ao lado deste arquivo.
REM Repassa todos os argumentos (ex.: run.cmd -RepoPath E:\repo -Mode polling).
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0run.ps1" %*
