param(
    [Parameter(Mandatory = $true)][string]$CheckIn,
    [Parameter(Mandatory = $true)][string]$CheckOut,
    [string[]]$Sources = @("superluxo"),
    [switch]$ConfirmRealReservation
)

$ErrorActionPreference = "Stop"
if (-not $ConfirmRealReservation) {
    throw "Este teste cria uma pré-reserva real. Execute novamente com -ConfirmRealReservation."
}
if ($Sources.Count -lt 1 -or $Sources.Count -gt 6) {
    throw "Informe entre 1 e 6 categorias físicas."
}

$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location $projectRoot
try {
    $env:TINO_BITZ_LIVE_CREATE = "CONFIRMAR_PRE_RESERVA_REAL"
    $env:TINO_BITZ_CHECKIN = $CheckIn
    $env:TINO_BITZ_CHECKOUT = $CheckOut
    $env:TINO_BITZ_SOURCES = $Sources -join ","
    go test -count=1 -run TestLiveBitzCreateReservation -v ./internal/assistente/bitz
    if ($LASTEXITCODE -ne 0) { throw "teste real do Bitz falhou" }
} finally {
    Remove-Item Env:TINO_BITZ_LIVE_CREATE -ErrorAction SilentlyContinue
    Remove-Item Env:TINO_BITZ_CHECKIN -ErrorAction SilentlyContinue
    Remove-Item Env:TINO_BITZ_CHECKOUT -ErrorAction SilentlyContinue
    Remove-Item Env:TINO_BITZ_SOURCES -ErrorAction SilentlyContinue
    Pop-Location
}
