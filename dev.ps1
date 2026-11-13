<#
.SYNOPSIS
Configure Qt, check, build and launch Hesora without an installer.
.EXAMPLE
.\dev.ps1
.EXAMPLE
.\dev.ps1 -SkipChecks
.EXAMPLE
.\dev.ps1 -NoRun
.EXAMPLE
. .\dev.ps1 -SetupOnly
#>
[CmdletBinding()]
param(
    [string]$QtRoot = $(if ($env:QT_ROOT) { $env:QT_ROOT } else { 'G:\qt\6.12.0\mingw_64' }),
    [string]$MingwRoot = '',
    [string]$PkgConfig = '',
    [ValidateRange(1, 64)][int]$Jobs = 2,
    [switch]$SetupOnly,
    [switch]$SkipChecks,
    [switch]$NoRun,
    [switch]$Restart
)

# Environment changes apply to this PowerShell session and its child processes.
# No setx, global Go settings, installer, or machine-wide PATH changes are used.
$ErrorActionPreference = 'Stop'
$repoRoot = $PSScriptRoot

function Invoke-HesoraTool {
    param([string]$File, [string[]]$ToolArguments)
    & $File @ToolArguments
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed (exit $LASTEXITCODE): $File $($ToolArguments -join ' ')"
    }
}

function Get-HesoraExecutable {
    param([string]$File)
    if (-not (Test-Path -LiteralPath $File -PathType Leaf)) {
        throw "Required tool not found: $File"
    }
    return (Resolve-Path -LiteralPath $File).ProviderPath
}

if ($env:OS -ne 'Windows_NT') { throw 'This script requires Windows x64.' }
$QtRoot = (Resolve-Path -LiteralPath $QtRoot -ErrorAction Stop).ProviderPath
$qtBin = Join-Path $QtRoot 'bin'
$qmake = Get-HesoraExecutable (Join-Path $qtBin 'qmake.exe')
$windeployqt = Get-HesoraExecutable (Join-Path $qtBin 'windeployqt.exe')
$pcDirectory = Join-Path $QtRoot 'lib\pkgconfig'
foreach ($module in @('Qt6Widgets', 'Qt6Pdf', 'Qt6PdfWidgets')) {
    $null = Get-HesoraExecutable (Join-Path $pcDirectory "$module.pc")
}

if ([string]::IsNullOrWhiteSpace($MingwRoot)) {
    $qtInstallation = Split-Path -Parent (Split-Path -Parent $QtRoot)
    $MingwRoot = Join-Path $qtInstallation 'Tools\mingw1310_64'
}
$MingwRoot = (Resolve-Path -LiteralPath $MingwRoot -ErrorAction Stop).ProviderPath
$mingwBin = Join-Path $MingwRoot 'bin'
$gcc = Get-HesoraExecutable (Join-Path $mingwBin 'gcc.exe')
$gxx = Get-HesoraExecutable (Join-Path $mingwBin 'g++.exe')

if ([string]::IsNullOrWhiteSpace($PkgConfig)) {
    $found = Get-Command pkg-config.exe -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($found) { $PkgConfig = $found.Source }
    else {
        foreach ($candidate in @(
            (Join-Path $mingwBin 'pkg-config.exe'),
            'C:\msys64\mingw64\bin\pkg-config.exe',
            'C:\msys64\ucrt64\bin\pkg-config.exe',
            'C:\msys64\usr\bin\pkg-config.exe'
        )) {
            if (Test-Path -LiteralPath $candidate -PathType Leaf) { $PkgConfig = $candidate; break }
        }
    }
}
if ([string]::IsNullOrWhiteSpace($PkgConfig)) {
    throw 'pkg-config.exe was not found. Supply its path with -PkgConfig.'
}
$PkgConfig = Get-HesoraExecutable $PkgConfig
$go = Get-Command go.exe -CommandType Application -ErrorAction Stop | Select-Object -First 1
$pathEntries = @((Split-Path -Parent $go.Source), $qtBin, $mingwBin) + @($env:PATH -split ';')
$env:PATH = ($pathEntries | Where-Object { $_ } | Select-Object -Unique) -join ';'
$env:QT_ROOT = $QtRoot
$env:PKG_CONFIG_PATH = (@($pcDirectory) + @($env:PKG_CONFIG_PATH -split ';') | Where-Object { $_ } | Select-Object -Unique) -join ';'
$env:PKG_CONFIG = $PkgConfig
$env:CGO_ENABLED = '1'
$env:CC = 'gcc'
$env:CXX = 'g++'
$env:GOMAXPROCS = [string]$Jobs
$env:QT_PLUGIN_PATH = Join-Path $QtRoot 'plugins'
$env:QT_QPA_PLATFORM_PLUGIN_PATH = Join-Path $QtRoot 'plugins\platforms'

Push-Location -LiteralPath $repoRoot
try {
    $qtVersion = (& $qmake -query QT_VERSION | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $qtVersion -notmatch '^6\.') { throw 'A working Qt 6 MinGW kit is required.' }
    $target = (& $gcc -dumpmachine | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $target -notmatch '^x86_64-.*mingw') { throw "Expected MinGW x64; found: $target" }
    $goTarget = @(& $go.Source env GOOS GOARCH)
    if ($LASTEXITCODE -ne 0 -or $goTarget.Count -ne 2 -or $goTarget[0] -ne 'windows' -or $goTarget[1] -ne 'amd64') {
        throw 'Go must target windows/amd64. Check GOOS and GOARCH in your shell.'
    }
    Invoke-HesoraTool $PkgConfig @('--exists', 'Qt6Widgets', 'Qt6Pdf', 'Qt6PdfWidgets')
    $pcVersion = (& $PkgConfig --modversion Qt6Widgets | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $pcVersion -ne $qtVersion) { throw 'pkg-config resolved a different Qt version from the selected kit.' }
    Write-Host "Qt $qtVersion : $QtRoot"
    Write-Host "MinGW        : $MingwRoot"
    Write-Host "pkg-config   : $PkgConfig"
    if ($SetupOnly) {
        Write-Host 'Environment ready. You can now run go test ./... or go vet ./... in this shell.'
        return
    }

    $output = Join-Path $repoRoot 'build\dev\bin'
    $exe = Join-Path $output 'invoice-maker.exe'
    # Only inspect the executable produced by this script, never installed copies.
    $running = @(Get-Process -Name 'invoice-maker' -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $exe })
    foreach ($process in $running) {
        if (-not $Restart) { throw 'The development app is running. Close it first, or rerun with -Restart to request a normal window close.' }
        Write-Host "Closing development window (PID $($process.Id))..."
        if (-not $process.CloseMainWindow() -or -not $process.WaitForExit(10000)) {
            throw 'The app did not close normally. Close its window before rebuilding; it will not be force-killed.'
        }
    }

    if (-not $SkipChecks) {
        Write-Host 'Running tests...'
        Invoke-HesoraTool $go.Source @('test', "-p=$Jobs", './...')
        Write-Host 'Running vet...'
        Invoke-HesoraTool $go.Source @('vet', "-p=$Jobs", './...')
    }
    $null = New-Item -ItemType Directory -Path $output -Force
    $ldflags = '-H=windowsgui'
    if (-not [string]::IsNullOrWhiteSpace($env:HESORA_LICENSE_PUBLIC_KEY)) {
        if ($env:HESORA_LICENSE_PUBLIC_KEY -notmatch '^[A-Za-z0-9_+/=-]+$') { throw 'HESORA_LICENSE_PUBLIC_KEY must be a Base64 Ed25519 public key.' }
        $ldflags += " -X main.hesoraLicensePublicKeyBase64=$($env:HESORA_LICENSE_PUBLIC_KEY)"
    }
    # Keep Go's build cache; -a and -trimpath would make normal iterations slower.
    Write-Host 'Building application...'
    Invoke-HesoraTool $go.Source @('build', "-p=$Jobs", "-ldflags=$ldflags", '-o', $exe, './cmd/invoice-maker')
    Write-Host 'Deploying Qt runtime...'
    Invoke-HesoraTool $windeployqt @('--release', '--compiler-runtime', '--no-translations', '--verbose', '0', '--dir', $output, $exe)

    $sourceAssets = Join-Path $repoRoot 'assets'
    foreach ($asset in Get-ChildItem -LiteralPath $sourceAssets -File -Recurse) {
        $relative = $asset.FullName.Substring($sourceAssets.Length).TrimStart('\', '/')
        $destination = Join-Path (Join-Path $output 'assets') $relative
        $existing = Get-Item -LiteralPath $destination -ErrorAction SilentlyContinue
        if (-not $existing -or $existing.Length -ne $asset.Length -or $existing.LastWriteTimeUtc -ne $asset.LastWriteTimeUtc) {
            $null = New-Item -ItemType Directory -Path (Split-Path -Parent $destination) -Force
            Copy-Item -LiteralPath $asset.FullName -Destination $destination -Force
        }
    }
    foreach ($required in @('Qt6Core.dll', 'Qt6Widgets.dll', 'Qt6Pdf.dll', 'Qt6PdfWidgets.dll', 'platforms\qwindows.dll', 'assets\font\iransans\IRANSansX-Regular.ttf')) {
        if (-not (Test-Path -LiteralPath (Join-Path $output $required) -PathType Leaf)) { throw "Runtime deployment is incomplete: $required" }
    }
    Write-Host "Build ready: $exe"
    if (-not $NoRun) {
        $app = Start-Process -FilePath $exe -WorkingDirectory $output -WindowStyle Normal -PassThru
        Write-Host "Hesora started (PID $($app.Id))."
    }
} finally {
    Pop-Location
}
