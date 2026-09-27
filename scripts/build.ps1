param([string]$Version = "0.1.0")
$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot
$distDir = Join-Path $projectRoot "dist"
New-Item -ItemType Directory -Force -Path $distDir | Out-Null
Push-Location $projectRoot
try {
    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    go build -trimpath -ldflags "-s -w -H windowsgui -X main.version=$Version" -o (Join-Path $distDir "Tino.exe") ./cmd/tino-ui
    if ($LASTEXITCODE -ne 0) { throw "build da interface gráfica falhou" }
    go build -trimpath -ldflags "-s -w -X main.version=$Version" -o (Join-Path $distDir "tino-cli-windows-amd64.exe") ./cmd/tino
	if ($LASTEXITCODE -ne 0) { throw "go build falhou" }
    Get-FileHash -Algorithm SHA256 (Join-Path $distDir "Tino.exe"), (Join-Path $distDir "tino-cli-windows-amd64.exe") | Format-Table -AutoSize
} finally {
    Pop-Location
}
