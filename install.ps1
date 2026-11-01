# novram automated installation script for Windows
$ErrorActionPreference = 'Stop'

$Repo = "PixelEzz/NoVram"
$Asset = "novram-windows-amd64.zip"
$Url = "https://github.com/$Repo/releases/latest/download/$Asset"
$InstallDir = "$env:LOCALAPPDATA\Programs\novram"
$BinPath = "$InstallDir\novram.exe"

Write-Host "==> Downloading novram for Windows (amd64)..." -ForegroundColor Cyan
$TempZip = Join-Path $env:TEMP $Asset
Invoke-WebRequest -Uri $Url -OutFile $TempZip

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

Write-Host "==> Extracting to $InstallDir..." -ForegroundColor Cyan
Expand-Archive -Path $TempZip -DestinationPath $InstallDir -Force
if (Test-Path "$InstallDir\novram-windows-amd64.exe") {
    Copy-Item "$InstallDir\novram-windows-amd64.exe" $BinPath -Force
}
Remove-Item $TempZip -Force

# Add to user PATH if not present
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Host "==> Adding $InstallDir to User PATH..." -ForegroundColor Cyan
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
}

Write-Host "==> novram installed successfully!" -ForegroundColor Green
Write-Host "==> Run 'novram doctor' in a new terminal to inspect hardware." -ForegroundColor Green
