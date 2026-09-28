param([string]$Version = "0.1.0")
$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot
$distDir = Join-Path $projectRoot "dist"
New-Item -ItemType Directory -Force -Path $distDir | Out-Null
Push-Location $projectRoot
try {
    $modernBuild = Join-Path $projectRoot "cmd\tino-modern\build"
    New-Item -ItemType Directory -Force -Path $modernBuild | Out-Null
    Copy-Item -Force (Join-Path $projectRoot "cmd\tino-ui\assets\tino-brand.png") (Join-Path $modernBuild "appicon.png")
    $wails = Join-Path (go env GOPATH) "bin\wails.exe"
    if (-not (Test-Path $wails)) {
        go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
        if ($LASTEXITCODE -ne 0) { throw "instalação do Wails falhou" }
    }
    Push-Location (Join-Path $projectRoot "cmd\tino-modern")
    try {
        & $wails build -clean -o Tino.exe -ldflags "-X main.version=$Version"
        if ($LASTEXITCODE -ne 0) { throw "build da interface moderna falhou" }
    } finally {
        Pop-Location
    }
    Copy-Item -Force (Join-Path $modernBuild "bin\Tino.exe") (Join-Path $distDir "Tino.exe")
    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    go build -trimpath -ldflags "-s -w -X main.version=$Version" -o (Join-Path $distDir "tino-cli-windows-amd64.exe") ./cmd/tino
	if ($LASTEXITCODE -ne 0) { throw "go build falhou" }
    Get-FileHash -Algorithm SHA256 (Join-Path $distDir "Tino.exe"), (Join-Path $distDir "tino-cli-windows-amd64.exe") | Format-Table -AutoSize
} finally {
    Pop-Location
}
