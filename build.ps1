# novram cross-platform build & packaging script for PowerShell
# Generates zero-dependency static binaries and release archives in ./dist

param (
    [string]$Version = "0.1.4"
)

$ErrorActionPreference = "Stop"
$DistDir = Join-Path $PSScriptRoot "dist"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host " novram Cross-Platform Build Automation (v$Version)" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

if (Test-Path $DistDir) {
    Remove-Item -Recurse -Force $DistDir
}
New-Item -ItemType Directory -Force -Path $DistDir | Out-Null

$env:CGO_ENABLED = "0"
$ldflags = "-s -w -X main.Version=$Version"

$targets = @(
    @{ OS = "windows"; Arch = "amd64"; Ext = ".exe"; Archive = "zip" },
    @{ OS = "linux";   Arch = "amd64"; Ext = "";     Archive = "tar.gz" },
    @{ OS = "linux";   Arch = "arm64"; Ext = "";     Archive = "tar.gz" },
    @{ OS = "darwin";  Arch = "arm64"; Ext = "";     Archive = "tar.gz" },
    @{ OS = "darwin";  Arch = "amd64"; Ext = "";     Archive = "tar.gz" }
)

$checksums = @()

foreach ($t in $targets) {
    $binName = "novram-$($t.OS)-$($t.Arch)$($t.Ext)"
    $binPath = Join-Path $DistDir $binName

    Write-Host "--> Compiling $($t.OS)/$($t.Arch) -> $binName..." -ForegroundColor Green
    $env:GOOS = $t.OS
    $env:GOARCH = $t.Arch

    go build -ldflags $ldflags -o $binPath ./cmd/novram

    Push-Location $DistDir
    try {
        if ($t.Archive -eq "zip") {
            $archiveName = "novram-$($t.OS)-$($t.Arch).zip"
            Write-Host "    Packaging $archiveName..." -ForegroundColor DarkGray
            Compress-Archive -Path $binName -DestinationPath $archiveName -Force
        } else {
            $archiveName = "novram-$($t.OS)-$($t.Arch).tar.gz"
            Write-Host "    Packaging $archiveName..." -ForegroundColor DarkGray
            tar -czf $archiveName $binName
        }

        # SHA256 Hash
        $hash = (Get-FileHash -Algorithm SHA256 $archiveName).Hash.ToLower()
        "$hash  $archiveName" | Out-File -Encoding utf8 "$archiveName.sha256"
        $checksums += "$hash  $archiveName"
    } finally {
        Pop-Location
    }
}

# Write master checksums.txt
$checksums | Out-File -Encoding utf8 (Join-Path $DistDir "checksums.txt")

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host " Build & packaging complete! Artifacts in: ./dist" -ForegroundColor Cyan
Get-ChildItem $DistDir | Select-Object Name, Length | Format-Table -AutoSize
Write-Host "==========================================================" -ForegroundColor Cyan
