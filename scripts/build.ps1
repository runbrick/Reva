param(
    [ValidateSet("amd64", "arm64")]
    [string]$Arch = "amd64"
)

$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") {
    throw "Run build.ps1 on Windows; use build.sh on macOS."
}

$previousGOOS = $env:GOOS
$previousGOARCH = $env:GOARCH
Push-Location (Join-Path $PSScriptRoot "..")
try {
    $env:GOOS = "windows"
    $env:GOARCH = $Arch
    $outputDir = "dist/windows-$Arch"
    New-Item -ItemType Directory -Force -Path $outputDir | Out-Null

    # Keep a bare stripped binary as a baseline for the resource overhead.
    & go build -trimpath -ldflags "-s -w -buildid= -H=windowsgui" -o "$outputDir/reqo-stripped.exe" ./cmd/reqo
    if ($LASTEXITCODE -ne 0) { throw "Stripped baseline build failed." }
    & go tool viva build --target windows --arch $Arch -o "$outputDir/reqo.exe"
    if ($LASTEXITCODE -ne 0) { throw "Release build failed." }
    & go tool viva package --target windows --arch $Arch --format nsis,zip --no-sign --program "$outputDir/reqo.exe" -o $outputDir
    if ($LASTEXITCODE -ne 0) { throw "Packaging failed." }

    $program = Get-Item -LiteralPath "$outputDir/reqo.exe"
    if ($program.VersionInfo.ProductName -ne "Reqo" -or [string]::IsNullOrWhiteSpace($program.VersionInfo.ProductVersion)) {
        throw "The release executable is missing the expected version resources."
    }
    $version = $program.VersionInfo.ProductVersion
    $packages = @(
        Get-Item -LiteralPath "$outputDir/reqo-$version-windows-$Arch-setup.exe"
        Get-Item -LiteralPath "$outputDir/reqo-$version-windows-$Arch.zip"
    )
    if (@($packages | Where-Object Length -eq 0).Count -gt 0) {
        throw "Windows packaging produced an empty artifact."
    }
    foreach ($artifact in @(
        Get-Item -LiteralPath "$outputDir/reqo-stripped.exe"
        $program
        $packages
    )) {
        Write-Host ("{0}: {1:N0} bytes ({2:N2} MB)" -f $artifact.FullName, $artifact.Length, ($artifact.Length / 1000000))
    }
}
finally {
    $env:GOOS = $previousGOOS
    $env:GOARCH = $previousGOARCH
    Pop-Location
}
