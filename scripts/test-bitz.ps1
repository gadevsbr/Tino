param(
    [Parameter(Mandatory = $true)][string]$CheckIn,
    [Parameter(Mandatory = $true)][string]$CheckOut,
    [string[]]$Sources = @("triploDeluxe")
)

$ErrorActionPreference = "Stop"
if ($Sources.Count -lt 1 -or $Sources.Count -gt 6) { throw "Informe entre 1 e 6 categorias físicas." }
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location $projectRoot
try {
    $env:TINO_BITZ_LIVE_PROBE = "1"
    $env:TINO_BITZ_CHECKIN = $CheckIn
    $env:TINO_BITZ_CHECKOUT = $CheckOut
    $env:TINO_BITZ_SOURCES = $Sources -join ","
    go test -count=1 -run TestLiveBitzProbe -v ./internal/assistente/bitz
    if ($LASTEXITCODE -ne 0) { throw "teste real sem salvamento falhou" }
} finally {
    Remove-Item Env:TINO_BITZ_LIVE_PROBE -ErrorAction SilentlyContinue
    Remove-Item Env:TINO_BITZ_CHECKIN -ErrorAction SilentlyContinue
    Remove-Item Env:TINO_BITZ_CHECKOUT -ErrorAction SilentlyContinue
    Remove-Item Env:TINO_BITZ_SOURCES -ErrorAction SilentlyContinue
    Pop-Location
}
