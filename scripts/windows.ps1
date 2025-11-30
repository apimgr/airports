# Airports API Server - Windows Installation Script
# Organization: apimgr
# Project: airports

#Requires -RunAsAdministrator

$ErrorActionPreference = "Stop"

# Configuration
$Project = "airports"
$Org = "apimgr"
$Repo = "https://github.com/$Org/$Project"
$BinaryName = "$Project.exe"
$ServiceName = $Project
$ServiceDisplayName = "Airports API Server"

# Directories
$ProgramData = $env:ProgramData
$ConfigDir = Join-Path $ProgramData "$Org\$Project"
$DataDir = Join-Path $ProgramData "$Org\$Project\data"
$LogsDir = Join-Path $ProgramData "$Org\$Project\logs"
$InstallDir = Join-Path $ProgramData "$Org\$Project\bin"

function Write-Info { param([string]$Message) Write-Host "[INFO] $Message" -ForegroundColor Green }
function Write-Warn { param([string]$Message) Write-Host "[WARN] $Message" -ForegroundColor Yellow }
function Write-Err { param([string]$Message) Write-Host "[ERROR] $Message" -ForegroundColor Red }

# Detect architecture
function Get-Arch {
    if ([Environment]::Is64BitOperatingSystem) {
        if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
            return "arm64"
        }
        return "amd64"
    }
    Write-Err "32-bit systems are not supported"
    exit 1
}

# Download latest release
function Download-Binary {
    $arch = Get-Arch
    $url = "$Repo/releases/latest/download/$Project-windows-$arch.exe"

    Write-Info "Downloading $Project for windows/$arch..."

    $tempPath = Join-Path $env:TEMP $BinaryName
    Invoke-WebRequest -Uri $url -OutFile $tempPath -UseBasicParsing

    return $tempPath
}

# Create directories
function Create-Directories {
    Write-Info "Creating directories..."

    @($ConfigDir, $DataDir, $LogsDir, $InstallDir) | ForEach-Object {
        if (-not (Test-Path $_)) {
            New-Item -ItemType Directory -Path $_ -Force | Out-Null
        }
    }
}

# Install binary
function Install-Binary {
    param([string]$TempPath)

    Write-Info "Installing $BinaryName to $InstallDir..."
    $destPath = Join-Path $InstallDir $BinaryName
    Move-Item -Path $TempPath -Destination $destPath -Force

    # Add to PATH if not already there
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    if ($currentPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$currentPath;$InstallDir", "Machine")
        Write-Info "Added $InstallDir to system PATH"
    }
}

# Install Windows Service
function Install-Service {
    Write-Info "Installing Windows service..."

    $binaryPath = Join-Path $InstallDir $BinaryName

    # Remove existing service if present
    $existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($existing) {
        Write-Info "Removing existing service..."
        Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
        sc.exe delete $ServiceName | Out-Null
        Start-Sleep -Seconds 2
    }

    # Create service using sc.exe
    $serviceArgs = "binPath= `"$binaryPath`" start= auto DisplayName= `"$ServiceDisplayName`""
    sc.exe create $ServiceName $serviceArgs | Out-Null

    # Set description
    sc.exe description $ServiceName "Global airport location information API with GeoIP integration" | Out-Null

    # Set recovery options (restart on failure)
    sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/10000/restart/30000 | Out-Null

    # Set environment variables for the service
    $envKey = "HKLM:\SYSTEM\CurrentControlSet\Services\$ServiceName"
    Set-ItemProperty -Path $envKey -Name "Environment" -Value @(
        "CONFIG_DIR=$ConfigDir",
        "DATA_DIR=$DataDir",
        "LOGS_DIR=$LogsDir"
    ) -Type MultiString

    Write-Info "Service installed successfully"
    Write-Info "Start with: Start-Service $ServiceName"
}

# Main installation
function Install-Airports {
    Write-Info "Installing $Project..."

    Create-Directories
    $tempPath = Download-Binary
    Install-Binary -TempPath $tempPath
    Install-Service

    Write-Info "Installation complete!"
    Write-Info "Configuration: $ConfigDir\server.yaml"
    Write-Info "Data: $DataDir"
    Write-Info "Logs: $LogsDir"

    # Show version
    $binaryPath = Join-Path $InstallDir $BinaryName
    & $binaryPath --version
}

# Uninstall
function Uninstall-Airports {
    Write-Info "Uninstalling $Project..."

    # Stop and remove service
    $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($service) {
        Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
        sc.exe delete $ServiceName | Out-Null
        Write-Info "Service removed"
    }

    # Remove binary directory
    if (Test-Path $InstallDir) {
        Remove-Item -Path $InstallDir -Recurse -Force
        Write-Info "Binary removed"
    }

    # Remove from PATH
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    $newPath = ($currentPath -split ';' | Where-Object { $_ -ne $InstallDir }) -join ';'
    [Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")

    Write-Info "Uninstallation complete"
    Write-Info "Config/data directories preserved: $ConfigDir"
}

# Handle arguments
$action = if ($args.Count -gt 0) { $args[0] } else { "install" }

switch ($action.ToLower()) {
    "install"   { Install-Airports }
    "uninstall" { Uninstall-Airports }
    default     { Write-Host "Usage: .\windows.ps1 [install|uninstall]"; exit 1 }
}
