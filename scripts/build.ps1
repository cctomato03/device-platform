# Build script: single executable with embedded web assets (Windows / PowerShell 5.1+ & 7+).
# NOTE: this file is ASCII-only on purpose, so Windows PowerShell 5.1 reads it correctly
#       regardless of code page settings.
# Usage: pwsh scripts/build.ps1 [-SkipWeb] [-OutDir bin]
param(
    [switch]$SkipWeb,
    [string]$OutDir = "bin"
)

$ErrorActionPreference = "Continue"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

# Resolve version info from git when available; the repo may not be a git working copy.
$prevEap = $ErrorActionPreference
$ErrorActionPreference = "SilentlyContinue"
$version = (git describe --tags --always --dirty 2>&1 | Out-String).Trim()
$commit = (git rev-parse --short HEAD 2>&1 | Out-String).Trim()
$ErrorActionPreference = $prevEap
if (-not $version -or $version -like "*fatal*") { $version = "dev" }
if (-not $commit -or $commit -like "*fatal*") { $commit = "none" }
$buildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags = "-s -w -X main.Version=$version -X main.Commit=$commit -X main.BuildTime=$buildTime"

if (-not $SkipWeb) {
    Write-Host "==> [1/3] building frontend (vite + pnpm)" -ForegroundColor Cyan
    Push-Location web
    if (-not (Test-Path node_modules)) { pnpm install; if ($LASTEXITCODE -ne 0) { throw "pnpm install failed" } }
    pnpm build
    if ($LASTEXITCODE -ne 0) { throw "frontend build failed" }
    Pop-Location
} else {
    Write-Host "==> [1/3] skipped frontend build (-SkipWeb)" -ForegroundColor Yellow
}

# Always re-sync the embed directory from web/dist so the binary never embeds a stale bundle.
if (-not (Test-Path web/dist/index.html)) {
    throw "web/dist/index.html not found - run 'cd web && pnpm build' first (or drop -SkipWeb)"
}
Write-Host "==> [2/3] syncing web/dist -> internal/webui/dist (go:embed target)" -ForegroundColor Cyan
if (Test-Path internal/webui/dist) { Remove-Item -Recurse -Force internal/webui/dist }
New-Item -ItemType Directory -Force -Path internal/webui/dist | Out-Null
Copy-Item -Recurse -Force web/dist/* internal/webui/dist/

Write-Host "==> [3/3] compiling backend (-tags embed)" -ForegroundColor Cyan
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$exe = Join-Path $OutDir "device-platform-server.exe"
go build -tags embed -trimpath -ldflags "$ldflags" -o $exe ./cmd/server
if ($LASTEXITCODE -ne 0) { throw "backend build failed" }

$size = [math]::Round((Get-Item $exe).Length / 1MB, 1)
Write-Host "==> done: $exe ($size MB) version=$version commit=$commit" -ForegroundColor Green
Write-Host "    run: $exe -c configs/config.yaml" -ForegroundColor Green
