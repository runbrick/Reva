$ErrorActionPreference = "Stop"
$nsisPaths = @(
    "${env:ProgramFiles(x86)}\NSIS\makensis.exe"
    "$env:ProgramFiles\NSIS\makensis.exe"
)

for ($attempt = 1; $attempt -le 3; $attempt++) {
    & choco install nsis --no-progress -y
    $installed = $LASTEXITCODE -eq 0
    $makensis = Get-Command makensis -ErrorAction SilentlyContinue
    foreach ($path in $nsisPaths) {
        if (Test-Path -LiteralPath $path -PathType Leaf) {
            $makensis = $path
            break
        }
    }
    if ($installed -and $makensis) { return }
    if ($attempt -lt 3) { Start-Sleep -Seconds (5 * $attempt) }
}

throw "NSIS installation did not provide makensis after three attempts."
