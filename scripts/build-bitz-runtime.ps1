param([switch]$SkipIfCurrent)
$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot
$target = Join-Path $projectRoot "internal\assistente\bitz\runtime\TinoBitz.exe"
$sources = @(
    (Join-Path $projectRoot "internal\assistente\bitz\scrapling\runner.py"),
    (Join-Path $projectRoot "internal\assistente\bitz\scrapling\requirements.txt"),
    $PSCommandPath
)
if ($SkipIfCurrent -and (Test-Path -LiteralPath $target)) {
    $targetTime = (Get-Item -LiteralPath $target).LastWriteTimeUtc
    $stale = $sources | Where-Object { (Get-Item -LiteralPath $_).LastWriteTimeUtc -gt $targetTime }
    if (-not $stale) { return }
}
$python = Join-Path $projectRoot "data\bitz-runtime\Scripts\python.exe"
if (-not (Test-Path -LiteralPath $python)) {
    python -m venv (Join-Path $projectRoot "data\bitz-runtime")
    if ($LASTEXITCODE -ne 0) { throw "Python 3.12+ necessário para construir o runtime Bitz" }
}
& $python -m pip install -r (Join-Path $projectRoot "internal\assistente\bitz\scrapling\requirements.txt")
if ($LASTEXITCODE -ne 0) { throw "instalação Scrapling falhou" }
& $python -m PyInstaller --noconfirm --clean --onefile --name TinoBitz `
    --collect-all scrapling --collect-all playwright --collect-all patchright `
    --collect-all browserforge --collect-all apify_fingerprint_datapoints `
    --distpath (Split-Path -Parent $target) `
    --workpath (Join-Path $projectRoot "tmp\bitz-build") `
    --specpath (Join-Path $projectRoot "tmp") `
    (Join-Path $projectRoot "internal\assistente\bitz\scrapling\runner.py")
if ($LASTEXITCODE -ne 0) { throw "empacotamento Scrapling falhou" }
