#Requires -Version 5.1
<#
.SYNOPSIS
    Installs Gecko programming language compiler on Windows.

.DESCRIPTION
    Downloads and installs the latest Gecko release, including the compiler
    binary and standard library.

.PARAMETER InstallDir
    Installation directory. Defaults to $env:USERPROFILE\.gecko

.PARAMETER NoPath
    Skip adding Gecko to the PATH environment variable.

.EXAMPLE
    iwr -useb https://raw.githubusercontent.com/neutrino2211/gecko/main/scripts/install.ps1 | iex

.EXAMPLE
    .\install.ps1 -InstallDir "C:\gecko"
#>

param(
    [string]$InstallDir = "$env:USERPROFILE\.gecko",
    [switch]$NoPath
)

$ErrorActionPreference = "Stop"

$Repo = "neutrino2211/gecko"
$Platform = "windows-amd64"

function Write-Info {
    param([string]$Message)
    Write-Host "[INFO] " -ForegroundColor Blue -NoNewline
    Write-Host $Message
}

function Write-Success {
    param([string]$Message)
    Write-Host "[OK] " -ForegroundColor Green -NoNewline
    Write-Host $Message
}

function Write-Warn {
    param([string]$Message)
    Write-Host "[WARN] " -ForegroundColor Yellow -NoNewline
    Write-Host $Message
}

function Write-Error-Message {
    param([string]$Message)
    Write-Host "[ERROR] " -ForegroundColor Red -NoNewline
    Write-Host $Message
    exit 1
}

function Get-LatestRelease {
    try {
        $releases = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases" -UseBasicParsing
        if ($releases.Count -gt 0) {
            return $releases[0].tag_name
        }
        return $null
    }
    catch {
        return $null
    }
}

function Install-Gecko {
    Write-Host ""
    Write-Host "  Gecko Installer for Windows" -ForegroundColor Cyan
    Write-Host "  ===========================" -ForegroundColor Cyan
    Write-Host ""

    # Get latest release
    Write-Info "Fetching latest release..."
    $Tag = Get-LatestRelease
    if (-not $Tag) {
        Write-Error-Message "Failed to fetch latest release. Check your internet connection."
    }
    Write-Info "Latest release: $Tag"

    # Download URL
    $DownloadUrl = "https://github.com/$Repo/releases/download/$Tag/gecko-$Platform.zip"
    Write-Info "Downloading from: $DownloadUrl"

    # Create temp directory
    $TempDir = Join-Path $env:TEMP "gecko-install-$(Get-Random)"
    New-Item -ItemType Directory -Path $TempDir -Force | Out-Null

    try {
        # Download
        $ZipPath = Join-Path $TempDir "gecko.zip"
        Write-Info "Downloading..."

        $ProgressPreference = 'SilentlyContinue'
        Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath -UseBasicParsing
        $ProgressPreference = 'Continue'

        # Extract
        Write-Info "Extracting..."
        $ExtractPath = Join-Path $TempDir "extracted"
        Expand-Archive -Path $ZipPath -DestinationPath $ExtractPath -Force

        # Create install directory
        Write-Info "Installing to $InstallDir..."
        if (-not (Test-Path $InstallDir)) {
            New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        }

        # Copy binary
        $BinarySource = Join-Path $ExtractPath "gecko.exe"
        if (-not (Test-Path $BinarySource)) {
            # Try finding it in subdirectories
            $BinarySource = Get-ChildItem -Path $ExtractPath -Filter "gecko.exe" -Recurse | Select-Object -First 1 -ExpandProperty FullName
        }

        if ($BinarySource -and (Test-Path $BinarySource)) {
            Copy-Item -Path $BinarySource -Destination (Join-Path $InstallDir "gecko.exe") -Force
            Write-Success "Binary installed"
        }
        else {
            Write-Error-Message "gecko.exe not found in archive"
        }

        # Copy stdlib
        $StdlibSource = Join-Path $ExtractPath "stdlib"
        if (-not (Test-Path $StdlibSource)) {
            $StdlibSource = Get-ChildItem -Path $ExtractPath -Filter "stdlib" -Directory -Recurse | Select-Object -First 1 -ExpandProperty FullName
        }

        if ($StdlibSource -and (Test-Path $StdlibSource)) {
            $StdlibDest = Join-Path $InstallDir "stdlib"
            if (Test-Path $StdlibDest) {
                Remove-Item -Path $StdlibDest -Recurse -Force
            }
            Copy-Item -Path $StdlibSource -Destination $StdlibDest -Recurse -Force
            Write-Success "Standard library installed"
        }
        else {
            Write-Warn "stdlib not found in archive"
        }

        # Add to PATH
        if (-not $NoPath) {
            $CurrentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
            if ($CurrentPath -notlike "*$InstallDir*") {
                Write-Info "Adding to PATH..."
                [Environment]::SetEnvironmentVariable("PATH", "$CurrentPath;$InstallDir", "User")
                Write-Success "Added $InstallDir to PATH"
                Write-Warn "Restart your terminal for PATH changes to take effect"
            }
            else {
                Write-Info "$InstallDir is already in PATH"
            }
        }

        # Set GECKO_HOME
        $CurrentGeckoHome = [Environment]::GetEnvironmentVariable("GECKO_HOME", "User")
        if (-not $CurrentGeckoHome) {
            Write-Info "Setting GECKO_HOME environment variable..."
            [Environment]::SetEnvironmentVariable("GECKO_HOME", $InstallDir, "User")
            Write-Success "GECKO_HOME set to $InstallDir"
        }

        Write-Host ""
        Write-Success "Gecko installed successfully!"
        Write-Host ""
        Write-Info "Version: $Tag"
        Write-Info "Binary:  $InstallDir\gecko.exe"
        Write-Info "Stdlib:  $InstallDir\stdlib"
        Write-Host ""
        Write-Host "Restart your terminal, then run 'gecko --help' to get started." -ForegroundColor Cyan
        Write-Host ""
    }
    finally {
        # Cleanup
        if (Test-Path $TempDir) {
            Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

# Run installation
Install-Gecko
