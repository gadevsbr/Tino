param([string]$Version = "0.1.0")
$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot
$distDir = Join-Path $projectRoot "dist"
New-Item -ItemType Directory -Force -Path $distDir | Out-Null
Push-Location $projectRoot
try {
    go run ./tools/icon ./cmd/tino-ui/assets/tino-brand.png ./cmd/tino-ui/assets/tino.ico
    if ($LASTEXITCODE -ne 0) { throw "geração do ícone falhou" }
    go run github.com/akavel/rsrc@v0.10.2 -manifest ./cmd/tino-ui/tino.manifest -ico ./cmd/tino-ui/assets/tino.ico -arch amd64 -o ./cmd/tino-ui/rsrc_windows_amd64.syso
    if ($LASTEXITCODE -ne 0) { throw "incorporação dos recursos Windows falhou" }
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
