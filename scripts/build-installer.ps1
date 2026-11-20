[CmdletBinding()]

param(
    [string]$ProductVersion = "0.1.0",
    [string]$QtRoot = $(if ($env:QT_ROOT) { $env:QT_ROOT } else { "G:\qt\6.12.0\mingw_64" }),
    [string]$OutputRoot = "",
    [string]$MingwRoot = "",
    [string]$PkgConfig = "",
    [ValidateRange(1, 64)][int]$Jobs = 2,
    [switch]$SetupOnly,
    [switch]$CheckOnly
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    $OutputRoot = Join-Path $repoRoot "artifacts"
}
$publishDir = Join-Path $OutputRoot "publish"
$installerDir = Join-Path $OutputRoot "installer"
$launcherDir = Join-Path $OutputRoot "installer\launcher"
$launcherPath = Join-Path $launcherDir "hesora-launcher.exe"
$exePath = Join-Path $publishDir "invoice-maker.exe"
$windeployqt = Join-Path $QtRoot "bin\windeployqt.exe"
$qmake = Join-Path $QtRoot "bin\qmake.exe"

function Fail([string]$Message) {
    throw "Installer build preflight failed: $Message"
}

if ($env:OS -ne "Windows_NT") {
    Fail "This script must run on Windows."
}

$goCommand = Get-Command go -ErrorAction SilentlyContinue
if (-not $goCommand) { Fail "Go was not found on PATH." }
$goVersion = (& $goCommand.Source version 2>&1 | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or $goVersion -notmatch '\bgo(\d+\.\d+\.\d+)\b') {
    Fail "Cannot determine the installed Go version: $goVersion"
}
$installedGo = [version]$Matches[1]
$goDirective = Select-String -LiteralPath (Join-Path $repoRoot 'go.mod') -Pattern '^go (\d+\.\d+(?:\.\d+)?)$' | Select-Object -First 1
if (-not $goDirective -or $installedGo -lt [version]$goDirective.Matches[0].Groups[1].Value) {
    Fail "Go must satisfy the minimum version declared in go.mod; found: $goVersion"
}

try {
    & (Join-Path $repoRoot "dev.ps1") -SetupOnly -QtRoot $QtRoot -MingwRoot $MingwRoot -PkgConfig $PkgConfig -Jobs $Jobs
} catch {
    Fail "Qt/MinGW environment setup failed: $($_.Exception.Message)"
}
$QtRoot = $env:QT_ROOT
$windeployqt = Join-Path $QtRoot "bin\windeployqt.exe"
$qmake = Join-Path $QtRoot "bin\qmake.exe"

if (-not (Test-Path $qmake) -or -not (Test-Path $windeployqt)) {
    Fail "Qt tools were not found under '$QtRoot'. Set -QtRoot or QT_ROOT to the Qt 6.12.0 MinGW kit."
}
$qtVersion = (& $qmake -query QT_VERSION 2>&1 | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or $qtVersion -ne "6.12.0") {
    Fail "Expected Qt 6.12.0; qmake reports '$qtVersion'."
}

if ($SetupOnly) {
    Write-Host "Installer Qt environment ready. No build or packaging was performed."
    return
}

$wixCommand = Get-Command wix -ErrorAction SilentlyContinue
if (-not $wixCommand) { Fail "WiX CLI was not found on PATH. Install/configure WiX Toolset 6.0.2." }
$wixVersion = (& wix --version 2>&1 | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or $wixVersion -notmatch '^6\.0\.2(\+|$)') {
    Fail "Expected WiX 6.0.2; found: $wixVersion"
}

$dotnetCommand = Get-Command dotnet -ErrorAction SilentlyContinue
if (-not $dotnetCommand) { Fail ".NET SDK is required to build the WiX SDK-style project." }

if ($ProductVersion -notmatch '^\d+\.\d+\.\d+$') {
    Fail "ProductVersion must have three numeric parts, for example 0.1.0."
}

if ($CheckOnly) {
    Write-Host "Installer preflight passed: $goVersion, Qt $qtVersion, WiX $wixVersion."
    return
}

$OutputRoot = [System.IO.Path]::GetFullPath($OutputRoot)
$publishDir = [System.IO.Path]::GetFullPath((Join-Path $OutputRoot "publish"))
$installerDir = Join-Path $OutputRoot "installer"
$exePath = Join-Path $publishDir "invoice-maker.exe"
if (-not $publishDir.StartsWith($OutputRoot.TrimEnd('\', '/') + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase) -or
    $repoRoot.StartsWith($publishDir + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase) -or $publishDir -eq $repoRoot) {
    Fail "Unsafe publish directory: $publishDir"
}
foreach ($directory in @($OutputRoot, $publishDir)) {
    if (Test-Path -LiteralPath $directory) {
        $entry = Get-Item -LiteralPath $directory
        if (-not $entry.PSIsContainer -or ($entry.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
            Fail "Output directory must be a regular directory, not a link or file: $directory"
        }
    }
}
$running = @(Get-Process -Name 'invoice-maker' -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $exePath })
if ($running.Count -gt 0) { Fail "Close the application running from '$exePath' before rebuilding its files." }

$ldflags = "-H=windowsgui"
if (-not [string]::IsNullOrWhiteSpace($env:HESORA_LICENSE_PUBLIC_KEY)) {
    if ($env:HESORA_LICENSE_PUBLIC_KEY -notmatch '^[A-Za-z0-9_+/=-]+$') {
        Fail "HESORA_LICENSE_PUBLIC_KEY must be a base64-encoded Ed25519 public key."
    }
    $ldflags += " -X main.hesoraLicensePublicKeyBase64=$($env:HESORA_LICENSE_PUBLIC_KEY)"
}

if (Test-Path -LiteralPath $publishDir) { Remove-Item -LiteralPath $publishDir -Recurse -Force }
New-Item -ItemType Directory -Path $publishDir -Force | Out-Null
New-Item -ItemType Directory -Path $installerDir -Force | Out-Null
New-Item -ItemType Directory -Path $launcherDir -Force | Out-Null

Push-Location $repoRoot
try {
    Write-Host "Building Windows GUI application with $goVersion (no console window)"
    & $goCommand.Source build "-p=$Jobs" -trimpath "-ldflags=$ldflags" -o $exePath ./cmd/invoice-maker
    if ($LASTEXITCODE -ne 0) { Fail "go build failed with exit code $LASTEXITCODE." }

    Write-Host "Building post-install launcher"
    & $goCommand.Source build -trimpath -ldflags="-H=windowsgui" -o $launcherPath ./cmd/installer-launcher
    if ($LASTEXITCODE -ne 0) { Fail "post-install launcher build failed with exit code $LASTEXITCODE." }

    Write-Host "Deploying Qt runtime from $QtRoot"
    & $windeployqt --release --compiler-runtime --no-translations --dir $publishDir $exePath
    if ($LASTEXITCODE -ne 0) { Fail "windeployqt failed with exit code $LASTEXITCODE." }

    # Bundle product assets, including the fixed application identity icon.
    $assetsSource = Join-Path $repoRoot "assets"
    $assetsDestination = Join-Path $publishDir "assets"
    if (-not (Test-Path (Join-Path $assetsSource "invoice-maker.ico"))) {
        Fail "Default application icon is missing under assets."
    }
    New-Item -ItemType Directory -Path $assetsDestination -Force | Out-Null
    Copy-Item -Path (Join-Path $assetsSource "*") -Destination $assetsDestination -Recurse -Force

    $publishIconPath = Join-Path $assetsDestination "invoice-maker.ico"
    foreach ($required in @(
        $exePath,
        (Join-Path $publishDir "Qt6Core.dll"),
        (Join-Path $publishDir "Qt6Widgets.dll"),
        (Join-Path $publishDir "Qt6Pdf.dll"),
        (Join-Path $publishDir "Qt6PdfWidgets.dll"),
        (Join-Path $publishDir "platforms\qwindows.dll"),
        $publishIconPath,
        (Join-Path $publishDir "assets\font\iransans\IRANSansX-Regular.ttf"),
        (Join-Path $publishDir "assets\font\IRANSharp\IRANSharpRegular.ttf")
    )) {
        if (-not (Test-Path $required)) {
            Fail "Required runtime or asset file was not deployed: $required"
        }
    }

    Write-Host "Building interactive MSI with WiX $wixVersion"
    & dotnet build (Join-Path $repoRoot "installer\InvoiceMaker.wixproj") -c Release "-p:ProductVersion=$ProductVersion" "-p:PublishDir=$publishDir" "-p:PublishIconPath=$publishIconPath" "-p:LaunchHelperPath=$launcherPath"
    if ($LASTEXITCODE -ne 0) { Fail "WiX/MSBuild failed with exit code $LASTEXITCODE." }

    $msi = Get-ChildItem -Path (Join-Path $repoRoot "installer") -Filter "HesoraSetup.msi" -Recurse -File |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
    if (-not $msi) { Fail "WiX build reported success but HesoraSetup.msi was not found." }

    $finalMsi = Join-Path $installerDir "HesoraSetup-$ProductVersion-x64.msi"
    Copy-Item -LiteralPath $msi.FullName -Destination $finalMsi -Force
    Write-Host "Installer created: $finalMsi"
    Write-Host ('Next: run scripts/test-installer.ps1 -MsiPath "' + $finalMsi + '" and complete the clean-VM checklist in docs/WINDOWS_INSTALLER.md.')
}
finally {
    Pop-Location
}
