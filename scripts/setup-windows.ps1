#Requires -Version 5.1
<#
.SYNOPSIS
  One-command Windows install: clone (if needed), build, and install the agent.

.DESCRIPTION
  Equivalent of scripts/setup-global.sh for Windows 11.
  Can be piped from the web:

    irm https://raw.githubusercontent.com/yesk993-ops/go-terminal-agent/master/scripts/setup-windows.ps1 | iex

  Or run from a local clone:

    powershell -ExecutionPolicy Bypass -File .\scripts\setup-windows.ps1
#>
[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"

$RepoURL   = "https://github.com/yesk993-ops/go-terminal-agent.git"
$CloneDir  = Join-Path $env:TEMP "ai-agent-install"
$BinDir    = Join-Path $env:LOCALAPPDATA "agent\bin"
$ConfigDir = Join-Path $env:APPDATA "agent"
$AgentExe  = Join-Path $BinDir "ai-agent.exe"

function Write-Info($msg)  { Write-Host "==> $msg" -ForegroundColor Cyan }
function Write-Ok($msg)    { Write-Host "  OK  $msg" -ForegroundColor Green }
function Write-Warn($msg)  { Write-Host " WARN  $msg" -ForegroundColor Yellow }
function Write-Err($msg)   { Write-Host "FAIL  $msg" -ForegroundColor Red; exit 1 }

Write-Host ""
Write-Host "========================================" -ForegroundColor Magenta
Write-Host "  Go Terminal AI Agent — Windows Setup" -ForegroundColor Magenta
Write-Host "========================================" -ForegroundColor Magenta
Write-Host ""

# ── Check Go ──────────────────────────────────────────────────────────
$goCmd = Get-Command go -ErrorAction SilentlyContinue
if (-not $goCmd) {
    Write-Err "Go not found. Install Go 1.22+ from https://go.dev/dl/ and re-run."
}
Write-Info "Found Go: $(& go version)"

# ── Check git ─────────────────────────────────────────────────────────
$gitCmd = Get-Command git -ErrorAction SilentlyContinue
$ProjectDir = $null

# If we are already inside a clone of this repo, use it.
$here = Get-Location
if (Test-Path (Join-Path $here "cmd\agent\main.go")) {
    $ProjectDir = $here.Path
    Write-Info "Using local repo at $ProjectDir"
} elseif ($gitCmd) {
    Write-Info "Fetching AI agent..."
    if (Test-Path $CloneDir) { Remove-Item -Recurse -Force $CloneDir }
    & git clone --depth 1 $RepoURL $CloneDir
    if ($LASTEXITCODE -ne 0) { Write-Err "git clone failed" }
    $ProjectDir = $CloneDir
    Write-Ok "Source cloned to $CloneDir"
} else {
    Write-Err "Neither a local clone nor git is available. Clone the repo manually and re-run."
}

# ── Build ─────────────────────────────────────────────────────────────
Write-Info "Building AI agent (CGO_ENABLED=0)..."
if (-not (Test-Path $BinDir)) {
    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
}
Push-Location $ProjectDir
try {
    $env:CGO_ENABLED = "0"
    & go build -ldflags="-s -w" -o $AgentExe .\cmd\agent
    if ($LASTEXITCODE -ne 0) { Write-Err "go build failed" }
} finally {
    Pop-Location
}
Write-Ok "Binary installed at $AgentExe"

# ── Launcher ──────────────────────────────────────────────────────────
Write-Info "Installing go-agent.cmd launcher..."
$launcher = Join-Path $BinDir "go-agent.cmd"
@"
@echo off
set "AGENT_BIN=%LOCALAPPDATA%\agent\bin\ai-agent.exe"
set "AGENT_CONFIG=%APPDATA%\agent\config.yaml"
if exist "%AGENT_CONFIG%" (
  "%AGENT_BIN%" --config "%AGENT_CONFIG%" %*
) else (
  "%AGENT_BIN%" %*
)
"@ | Set-Content -Path $launcher -Encoding ASCII
Write-Ok "Launcher at $launcher"

# ── Config ────────────────────────────────────────────────────────────
Write-Info "Setting up config..."
if (-not (Test-Path $ConfigDir)) {
    New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null
}
$destCfg = Join-Path $ConfigDir "config.yaml"
$srcCfg  = Join-Path $ProjectDir "config.yaml"
if (-not (Test-Path $destCfg)) {
    if (Test-Path $srcCfg) {
        Copy-Item $srcCfg $destCfg
        Write-Ok "Default config created at $destCfg"
        Write-Warn "Edit $destCfg (or set env vars) to add API keys"
    }
} else {
    Write-Ok "Existing config found at $destCfg"
}

$sessDir = Join-Path $env:LOCALAPPDATA "agent\sessions"
if (-not (Test-Path $sessDir)) {
    New-Item -ItemType Directory -Path $sessDir -Force | Out-Null
}

# ── PATH ──────────────────────────────────────────────────────────────
Write-Info "Ensuring $BinDir is on user PATH..."
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (-not $userPath) { $userPath = "" }
$parts = $userPath -split ';' | Where-Object { $_ -ne "" }
if ($parts -notcontains $BinDir) {
    $newPath = if ($userPath.TrimEnd(';') -eq "") { $BinDir } else { "$($userPath.TrimEnd(';'));$BinDir" }
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    $env:Path = "$BinDir;$env:Path"
    Write-Ok "Added to user PATH"
} else {
    Write-Ok "Already on user PATH"
}

# ── Cleanup temp clone ────────────────────────────────────────────────
if ($ProjectDir -eq $CloneDir -and (Test-Path $CloneDir)) {
    Remove-Item -Recurse -Force $CloneDir -ErrorAction SilentlyContinue
}

Write-Host ""
Write-Host "=== Installation complete ===" -ForegroundColor Green
Write-Host ""
Write-Host "  Open a NEW terminal, then run:" -ForegroundColor White
Write-Host "    ai-agent                 Launch interactive TUI" -ForegroundColor Cyan
Write-Host "    ai-agent `"what is docker`"  Ask a question" -ForegroundColor Cyan
Write-Host "    go-agent                 Friendly alias" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Config: $ConfigDir\config.yaml" -ForegroundColor Yellow
Write-Host ""
Write-Host "  Set an API key (example):" -ForegroundColor White
Write-Host '    [System.Environment]::SetEnvironmentVariable("NVIDIA_API_KEY","nvapi-...","User")' -ForegroundColor Yellow
Write-Host "  Or edit the config file / create $ConfigDir\agent.env" -ForegroundColor Yellow
Write-Host ""
