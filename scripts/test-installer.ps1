[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$MsiPath
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if ($env:OS -ne "Windows_NT") {
    throw "Installer package inspection must run on Windows."
}
if (-not (Test-Path -LiteralPath $MsiPath -PathType Leaf)) {
    throw "MSI file does not exist: $MsiPath"
}

$resolvedMsi = (Resolve-Path -LiteralPath $MsiPath).Path
$installer = New-Object -ComObject WindowsInstaller.Installer
$database = $installer.OpenDatabase($resolvedMsi, 0)

function Read-MsiColumn([string]$Query, [int]$Column = 1) {
    $view = $database.OpenView($Query)
    $view.Execute()
    $values = [System.Collections.Generic.List[string]]::new()
    while ($record = $view.Fetch()) {
        $values.Add([string]$record.StringData($Column))
    }
    $view.Close()
    return $values
}

$fileNames = Read-MsiColumn "SELECT FileName FROM File"
$requiredNames = @("invoice-maker.exe", "invoice-maker.ico", "Qt6Core.dll", "Qt6Widgets.dll", "Qt6Pdf.dll", "Qt6PdfWidgets.dll", "qwindows.dll")
foreach ($required in $requiredNames) {
    $found = $false
    foreach ($fileName in $fileNames) {
        $longName = ($fileName -split '\|')[-1]
        if ($longName.Equals($required, [System.StringComparison]::OrdinalIgnoreCase)) {
            $found = $true
            break
        }
    }
    if (-not $found) {
        throw "MSI is missing required application/runtime file: $required"
    }
}

$dialogNames = Read-MsiColumn "SELECT Name FROM Dialog"
$requiredDialogs = @("WelcomeDlg", "LicenseAgreementDlg", "InstallDirDlg", "VerifyReadyDlg")
foreach ($required in $requiredDialogs) {
    if ($required -notin $dialogNames) {
        throw "MSI is missing expected interactive installer dialog: $required"
    }
}

$shortcutView = $database.OpenView("SELECT Name, Directory_ FROM Shortcut")
$shortcutView.Execute()
$shortcuts = [System.Collections.Generic.List[object]]::new()
while ($record = $shortcutView.Fetch()) {
    $shortcuts.Add([pscustomobject]@{
        Name = [string]$record.StringData(1)
        Directory = [string]$record.StringData(2)
    })
}
$shortcutView.Close()

$desktopShortcut = $shortcuts | Where-Object { $_.Directory -eq "DesktopFolder" } | Select-Object -First 1
if (-not $desktopShortcut) {
    throw "MSI does not contain a shortcut targeted to DesktopFolder."
}
if (-not ($shortcuts | Where-Object { $_.Directory -eq "ProgramMenuDir" })) {
    throw "MSI does not contain the expected Start Menu shortcut."
}

Write-Host "MSI package inspection passed: $resolvedMsi"
Write-Host "Verified payload entries: $($requiredNames -join ', ')"
Write-Host "Verified interactive dialogs: $($requiredDialogs -join ', ')"
Write-Host "Verified Desktop and Start Menu shortcuts."
Write-Host "This check does not replace install, upgrade, uninstall, or clean-VM testing."
