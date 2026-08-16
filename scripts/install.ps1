#Requires -Version 5.1
<#
.SYNOPSIS
  Install the Go Terminal AI Agent on Windows 11.

.DESCRIPTION
  Builds (or downloads a pre-built) ai-agent.exe, installs it to
  %LOCALAPPDATA%\agent\bin, copies a default config to
  %APPDATA%\agent\config.yaml, and optionally adds the bin directory
  to the user PATH.

  Usage (from a cloned repo):
    powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1

  One-liner (from the internet once published):
    irm https://raw.githubusercontent.com/yesk993-ops/go-terminal-agent/master/scripts/install.ps1 | iex

.PARAMETER SkipBuild
  Skip the Go build step and only install an existing .\agent.exe / .\ai-agent.exe.

.PARAMETER AddToPath
  Add the install bin directory to the user PATH (default: $true).
#>
[CmdletBinding()]
param(
    [switch]$SkipBuild,
    [bool]$AddToPath = $true
)

$ErrorActionPreference = "Stop"

function Write-Info($msg)  { Write-Host "==> $msg" -ForegroundColor Cyan }
function Write-Ok($msg)    { Write-Host "  OK  $msg" -ForegroundColor Green }
function Write-Warn($msg)  { Write-Host " WARN  $msg" -ForegroundColor Yellow }
function Write-Err($msg)   { Write-Host "FAIL  $msg" -ForegroundColor Red; exit 1 }

# ── Paths ────────────────────────────────────────────────────────────
$BinDir    = Join-Path $env:LOCALAPPDATA "agent\bin"
$ConfigDir = Join-Path $env:APPDATA "agent"
$AgentExe  = Join-Path $BinDir "ai-agent.exe"
$GoCmdExe  = Join-Path $BinDir "go-agent.cmd"   # launcher that won't shadow real `go`

# Resolve repo root (script lives in scripts\)
$ScriptDir  = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectDir = Split-Path -Parent $ScriptDir

Write-Host ""
Write-Host "========================================" -ForegroundColor Magenta
Write-Host "  Go Terminal AI Agent — Windows Setup" -ForegroundColor Magenta
Write-Host "========================================" -ForegroundColor Magenta
Write-Host ""

# ── Preconditions ────────────────────────────────────────────────────
function Test-GoInstalled {
    $go = Get-Command go -ErrorAction SilentlyContinue
    if ($go) {
        Write-Info "Found Go: $(& go version)"
        return $true
    }
    return $false
}

# ── Build ────────────────────────────────────────────────────────────
function Build-Agent {
    Write-Info "Building AI agent (CGO_ENABLED=0, windows/amd64)..."
    if (-not (Test-Path $BinDir)) {
        New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
    }

    Push-Location $ProjectDir
    try {
        $env:CGO_ENABLED = "0"
        & go build -ldflags="-s -w" -o $AgentExe .\cmd\agent
        if ($LASTEXITCODE -ne 0) {
            Write-Err "go build failed (exit $LASTEXITCODE)"
        }
    } finally {
        Pop-Location
    }
    Write-Ok "Binary installed at $AgentExe"
}

function Install-ExistingBinary {
    $candidates = @(
        (Join-Path $ProjectDir "ai-agent.exe"),
        (Join-Path $ProjectDir "agent.exe"),
        (Join-Path $ProjectDir "agent-windows-amd64.exe")
    )
    $src = $candidates | Where-Object { Test-Path $_ } | Select-Object -First 1
    if (-not $src) {
        Write-Err "No pre-built binary found and -SkipBuild was set. Run without -SkipBuild or build first with: go build -o ai-agent.exe .\cmd\agent"
    }
    if (-not (Test-Path $BinDir)) {
        New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
    }
    Copy-Item -Force $src $AgentExe
    Write-Ok "Copied $src -> $AgentExe"
}

# ── Launcher (go-agent.cmd) ──────────────────────────────────────────
# We deliberately do NOT install a `go.cmd` that shadows the real Go compiler.
# On Windows the AI assistant is invoked as `ai-agent` or the friendly alias
# `go-agent`. Users who want a `go` alias can create one themselves.
function Install-Launcher {
    Write-Info "Installing go-agent.cmd launcher..."
    $configFile = Join-Path $ConfigDir "config.yaml"
    $content = @"
@echo off
REM Go Terminal AI Agent launcher
REM Usage: go-agent                 (interactive TUI)
REM        go-agent "your prompt"   (one-shot CLI)
set "AGENT_BIN=%LOCALAPPDATA%\agent\bin\ai-agent.exe"
set "AGENT_CONFIG=%APPDATA%\agent\config.yaml"
if exist "%AGENT_CONFIG%" (
  "%AGENT_BIN%" --config "%AGENT_CONFIG%" %*
) else (
  "%AGENT_BIN%" %*
)
"@
    Set-Content -Path $GoCmdExe -Value $content -Encoding ASCII
    Write-Ok "Launcher installed at $GoCmdExe"
}

# ── Config ────────────────────────────────────────────────────────────
function Setup-Config {
    Write-Info "Setting up config..."
    if (-not (Test-Path $ConfigDir)) {
        New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null
    }
    $dest = Join-Path $ConfigDir "config.yaml"
    $src  = Join-Path $ProjectDir "config.yaml"
    if (-not (Test-Path $dest)) {
        if (Test-Path $src) {
            Copy-Item $src $dest
            Write-Ok "Default config created at $dest"
            Write-Warn "Edit $dest to add API keys (or set env vars)"
        } else {
            Write-Warn "No config.yaml found in project; create one at $dest"
        }
    } else {
        Write-Ok "Existing config found at $dest"
    }

    # Create sessions directory
    $sessDir = Join-Path $env:LOCALAPPDATA "agent\sessions"
    if (-not (Test-Path $sessDir)) {
        New-Item -ItemType Directory -Path $sessDir -Force | Out-Null
    }
}

# ── PATH ──────────────────────────────────────────────────────────────
function Add-BinToPath {
    if (-not $AddToPath) { return }
    Write-Info "Ensuring $BinDir is on user PATH..."
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (-not $userPath) { $userPath = "" }
    $parts = $userPath -split ';' | Where-Object { $_ -ne "" }
    if ($parts -contains $BinDir) {
        Write-Ok "Already on user PATH"
        return
    }
    $newPath = if ($userPath.TrimEnd(';') -eq "") { $BinDir } else { "$($userPath.TrimEnd(';'));$BinDir" }
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    # Also update current session so the user can run immediately.
    $env:Path = "$BinDir;$env:Path"
    Write-Ok "Added to user PATH (restart terminals to pick up permanently)"
}

# ── Main ──────────────────────────────────────────────────────────────
if (-not $SkipBuild) {
    if (-not (Test-GoInstalled)) {
        Write-Err "Go not found. Install Go 1.22+ from https://go.dev/dl/ then re-run this script."
    }
    Build-Agent
} else {
    Install-ExistingBinary
}

Install-Launcher
Setup-Config
Add-BinToPath

Write-Host ""
Write-Host "=== Installation complete ===" -ForegroundColor Green
Write-Host ""
Write-Host "  Commands:" -ForegroundColor White
Write-Host "    ai-agent                 Launch interactive TUI" -ForegroundColor Cyan
Write-Host "    ai-agent `"what is docker`"  Ask a question" -ForegroundColor Cyan
Write-Host "    go-agent                 Same as ai-agent (friendly alias)" -ForegroundColor Cyan
Write-Host "    ai-agent --list-providers" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Config file:" -ForegroundColor White
Write-Host "    $ConfigDir\config.yaml" -ForegroundColor Yellow
Write-Host ""
Write-Host "  Set at least one API key (PowerShell):" -ForegroundColor White
Write-Host '    $env:NVIDIA_API_KEY = "nvapi-..."     # free, reliable' -ForegroundColor Yellow
Write-Host '    $env:GROQ_API_KEY    = "gsk_..."      # free tier' -ForegroundColor Yellow
Write-Host '    $env:OPENAI_API_KEY  = "sk-..."       # paid' -ForegroundColor Yellow
Write-Host '    $env:GEMINI_API_KEY  = "AIza..."      # free tier' -ForegroundColor Yellow
Write-Host '    $env:OPENROUTER_API_KEY = "sk-or-..." # paid' -ForegroundColor Yellow
Write-Host ""
Write-Host "  To persist keys across sessions, either:" -ForegroundColor White
Write-Host "    1. Edit $ConfigDir\config.yaml" -ForegroundColor Yellow
Write-Host "    2. Create $ConfigDir\agent.env with KEY=value lines" -ForegroundColor Yellow
Write-Host "    3. Set User environment variables in Windows Settings" -ForegroundColor Yellow
Write-Host ""
Write-Host "  Tip: open a NEW PowerShell/Terminal window so PATH updates apply." -ForegroundColor White
Write-Host ""
