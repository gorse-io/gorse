#Requires -Version 5.1

$ErrorActionPreference = "Stop"

$Repo = if ($env:GORSE_REPO) { $env:GORSE_REPO } else { "gorse-io/gorse" }
$Version = if ($env:GORSE_CLI_VERSION) { $env:GORSE_CLI_VERSION } else { "latest" }
$DefaultInstallDir = if ($env:LOCALAPPDATA) { Join-Path $env:LOCALAPPDATA "Programs\gorse\bin" } else { Join-Path ([IO.Path]::GetTempPath()) "gorse\bin" }
$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { $DefaultInstallDir }
$BinaryName = if ($env:BINARY_NAME) { $env:BINARY_NAME } else { "gorse-cli.exe" }
if (-not $BinaryName.EndsWith(".exe", [StringComparison]::OrdinalIgnoreCase)) {
    $BinaryName = "${BinaryName}.exe"
}

function Write-Log {
    param([string]$Message)
    Write-Host $Message
}

function Fail {
    param([string]$Message)
    Write-Error "error: $Message"
    exit 1
}

function Normalize-Arch {
    $arch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
    if (-not $arch) {
        Fail "unable to detect processor architecture"
    }
    switch ($arch.ToLowerInvariant()) {
        "amd64" { return "amd64" }
        "x86_64" { return "amd64" }
        "arm64" { return "arm64" }
        "aarch64" { return "arm64" }
        default { Fail "unsupported architecture: $arch" }
    }
}

function Download {
    param(
        [string]$Url,
        [string]$Output
    )

    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    } catch {
        # Ignore on PowerShell runtimes where TLS configuration is not supported.
    }

    Invoke-WebRequest -Uri $Url -OutFile $Output -UseBasicParsing
}

function Install-Binary {
    param(
        [string]$Source,
        [string]$Destination
    )

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item -Force -Path $Source -Destination $Destination
}

function ConvertTo-NormalizedPath {
    param([string]$Directory)

    $expandedDirectory = [Environment]::ExpandEnvironmentVariables($Directory.Trim().Trim('"'))
    return [IO.Path]::GetFullPath($expandedDirectory).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
}

function Test-PathEntry {
    param(
        [AllowNull()]
        [string]$PathValue,
        [string]$Directory
    )

    $fullDirectory = ConvertTo-NormalizedPath $Directory
    $entries = ($PathValue -split [IO.Path]::PathSeparator) | Where-Object { $_ }
    foreach ($entry in $entries) {
        try {
            $fullEntry = ConvertTo-NormalizedPath $entry
            if ([string]::Equals($fullEntry, $fullDirectory, [StringComparison]::OrdinalIgnoreCase)) {
                return $true
            }
        } catch {
            # Ignore malformed PATH entries.
        }
    }
    return $false
}

function Add-InstallDirToPath {
    $userPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::User)
    $machinePath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine)

    if (-not (Test-PathEntry $userPath $InstallDir) -and -not (Test-PathEntry $machinePath $InstallDir)) {
        $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) {
            $InstallDir
        } else {
            $userPath.TrimEnd([IO.Path]::PathSeparator) + [IO.Path]::PathSeparator + $InstallDir
        }
        [Environment]::SetEnvironmentVariable("Path", $newUserPath, [EnvironmentVariableTarget]::User)
        Write-Log "Added ${InstallDir} to the user PATH."
    }

    # Environment variable changes are process-local. Update this PowerShell
    # process as well so direct script invocations can use the CLI immediately.
    if (-not (Test-PathEntry $env:Path $InstallDir)) {
        $env:Path = if ([string]::IsNullOrWhiteSpace($env:Path)) {
            $InstallDir
        } else {
            $env:Path.TrimEnd([IO.Path]::PathSeparator) + [IO.Path]::PathSeparator + $InstallDir
        }
    }
}

function Main {
    if ([System.Environment]::OSVersion.Platform -ne [System.PlatformID]::Win32NT) {
        Fail "unsupported operating system: $([System.Environment]::OSVersion.Platform)"
    }

    $arch = Normalize-Arch
    $asset = "gorse-cli_windows_${arch}.exe"
    if ($Version -eq "latest") {
        $url = "https://github.com/${Repo}/releases/latest/download/${asset}"
    } else {
        $url = "https://github.com/${Repo}/releases/download/${Version}/${asset}"
    }

    $tmpDir = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName())
    New-Item -ItemType Directory -Path $tmpDir | Out-Null
    try {
        $tmpFile = Join-Path $tmpDir $asset
        Write-Log "Downloading ${asset} from ${Repo} (${Version})..."
        Download $url $tmpFile

        $destination = Join-Path $InstallDir $BinaryName
        Install-Binary $tmpFile $destination
        Write-Log "Installed ${BinaryName} to ${destination}"
        Add-InstallDirToPath
        Write-Log "Run '${BinaryName} --version' to verify the installation. Open a new terminal if necessary."
    } finally {
        Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
    }
}

Main
