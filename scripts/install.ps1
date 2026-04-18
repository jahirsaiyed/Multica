# Multica CLI installer for Windows
#
# Run from PowerShell (as Administrator for system-wide install):
#   irm https://raw.githubusercontent.com/jahirsaiyed/Multica/main/scripts/install.ps1 | iex
#
# Or with custom server/app URLs:
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/jahirsaiyed/Multica/main/scripts/install.ps1))) `
#       -ServerUrl "https://your-backend.com" -AppUrl "https://your-frontend.com"

param(
    [string]$ServerUrl = "",
    [string]$AppUrl = "",
    [string]$InstallDir = "$env:ProgramFiles\multica",
    [switch]$UserInstall  # Install to user directory without requiring Admin
)

$ErrorActionPreference = "Stop"
$RepoUrl = "https://github.com/jahirsaiyed/Multica"

function Write-Info  { Write-Host "==> $args" -ForegroundColor Cyan }
function Write-Ok    { Write-Host "v $args" -ForegroundColor Green }
function Write-Warn  { Write-Host "! $args" -ForegroundColor Yellow }
function Write-Fail  { Write-Host "x $args" -ForegroundColor Red; exit 1 }

function Get-Arch {
    $arch = (Get-CimInstance Win32_Processor).Architecture
    # 0 = x86, 9 = x86_64, 12 = ARM64
    if ($arch -eq 12) { return "arm64" }
    return "amd64"
}

function Get-LatestVersion {
    try {
        $response = Invoke-WebRequest -Uri "$RepoUrl/releases/latest" -MaximumRedirection 0 -ErrorAction SilentlyContinue
    } catch {
        $response = $_.Exception.Response
    }
    $location = $response.Headers["Location"]
    if (-not $location) {
        Write-Fail "Could not determine latest release. Check your network connection."
    }
    return ($location -split "/tag/")[1].Trim()
}

function Install-Multica {
    $arch = Get-Arch
    Write-Info "Detected architecture: $arch"

    $version = Get-LatestVersion
    Write-Info "Latest release: $version"

    $fileName = "multica_windows_$arch.zip"
    $downloadUrl = "$RepoUrl/releases/download/$version/$fileName"
    $tmpDir = Join-Path $env:TEMP "multica_install"
    $zipPath = Join-Path $tmpDir "multica.zip"

    New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null

    Write-Info "Downloading $downloadUrl ..."
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath

    Write-Info "Extracting..."
    Expand-Archive -Path $zipPath -DestinationPath $tmpDir -Force

    # User install: put in ~/.multica/bin instead of Program Files
    if ($UserInstall) {
        $script:InstallDir = Join-Path $env:USERPROFILE ".multica\bin"
    }

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $exeSrc = Join-Path $tmpDir "multica.exe"
    $exeDst = Join-Path $InstallDir "multica.exe"
    Copy-Item -Path $exeSrc -Destination $exeDst -Force

    Remove-Item -Recurse -Force $tmpDir

    Write-Ok "Multica CLI installed to $exeDst"
    return $exeDst
}

function Add-ToPath {
    param([string]$Dir)

    $scope = if ($UserInstall) { "User" } else { "Machine" }
    $current = [Environment]::GetEnvironmentVariable("Path", $scope)

    if ($current -notlike "*$Dir*") {
        [Environment]::SetEnvironmentVariable("Path", "$current;$Dir", $scope)
        Write-Ok "Added $Dir to PATH ($scope)"
    } else {
        Write-Ok "$Dir is already in PATH"
    }

    # Also update current session
    $env:Path = "$env:Path;$Dir"
}

function Configure-Cli {
    param([string]$ExePath)

    if ($ServerUrl -ne "") {
        Write-Info "Setting server_url = $ServerUrl"
        & $ExePath config set server_url $ServerUrl
    }
    if ($AppUrl -ne "") {
        Write-Info "Setting app_url = $AppUrl"
        & $ExePath config set app_url $AppUrl
    }
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
Write-Host ""
Write-Host "  Multica - Windows Installer" -ForegroundColor White
Write-Host ""

# Check if running as admin when doing system-wide install
if (-not $UserInstall) {
    $isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
        [Security.Principal.WindowsBuiltInRole]::Administrator
    )
    if (-not $isAdmin) {
        Write-Warn "Not running as Administrator. Switching to user install (~\.multica\bin)."
        Write-Warn "Re-run as Administrator for a system-wide install to $InstallDir."
        $UserInstall = $true
    }
}

$exePath = Install-Multica
Add-ToPath -Dir $InstallDir
Configure-Cli -Dir $exePath

Write-Host ""
Write-Host "  ===========================================" -ForegroundColor Green
Write-Host "  v Multica CLI installed!" -ForegroundColor Green
Write-Host "  ===========================================" -ForegroundColor Green
Write-Host ""
Write-Host "  Next steps (open a new terminal):" -ForegroundColor White
Write-Host ""

if ($ServerUrl -ne "" -or $AppUrl -ne "") {
    Write-Host "    multica login" -ForegroundColor Cyan
} else {
    Write-Host "    multica login --server-url https://multica.onrender.com --app-url https://multica-web-livid.vercel.app" -ForegroundColor Cyan
}

Write-Host "    multica daemon start" -ForegroundColor Cyan
Write-Host ""
