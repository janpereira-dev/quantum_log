# Remove a qlog binary installed by install.ps1. Local data is preserved.
[CmdletBinding()]
param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$Arguments
)

$ErrorActionPreference = 'Stop'
$installDir = if ($env:QLOG_INSTALL_DIR) { $env:QLOG_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\QUANTUM_LOG\bin' }
$modifyPath = $true
$dryRun = $false
$purgeData = $false

function Show-Usage {
    @'
Usage: uninstall.ps1 [options]

Options:
  --install-dir DIRECTORY Remove qlog from DIRECTORY.
  --no-modify-path        Leave the user PATH unchanged.
  --purge-data            Also delete the local QUANTUM_LOG ledger after cleanup.
  --dry-run               Print planned changes without writing files.
  --help                  Show this help.

By default, local QUANTUM_LOG ledger data is preserved.
'@ | Write-Output
}

function Fail([string]$Message) { throw "uninstall.ps1: $Message" }

for ($index = 0; $index -lt $Arguments.Count; $index++) {
    $argument = $Arguments[$index]
    switch -Regex ($argument) {
        '^--install-dir$' {
            $index++
            if ($index -ge $Arguments.Count) { Fail '--install-dir requires a value' }
            $installDir = $Arguments[$index]
            continue
        }
        '^--install-dir=(.+)$' { $installDir = $Matches[1]; continue }
        '^--no-modify-path$' { $modifyPath = $false; continue }
        '^--purge-data$' { $purgeData = $true; continue }
        '^--dry-run$' { $dryRun = $true; continue }
        '^--help$|^-h$' { Show-Usage; exit 0 }
        default { Fail "unknown option: $argument" }
    }
}

if ([string]::IsNullOrWhiteSpace($installDir)) { Fail '--install-dir cannot be empty' }
$target = Join-Path $installDir 'qlog.exe'
Write-Output "binary: $target"
if ($modifyPath) { Write-Output "user PATH: $installDir" }
if ($dryRun) {
	Write-Output 'dry-run: no files changed; qlog-owned setup and local data are preserved'
    exit 0
}

if (Test-Path -LiteralPath $target) {
	$cleanupArguments = @('uninstall', '--json')
	if ($purgeData) { $cleanupArguments += '--purge-data' }
	& $target @cleanupArguments
	if ($LASTEXITCODE -ne 0) {
		Fail "qlog-owned cleanup failed; retained $target so cleanup can be retried"
	}
    Remove-Item -LiteralPath $target -Force
    Write-Output "removed $target"
} else {
	if ($purgeData) { Fail '--purge-data requires the installed qlog binary to remove qlog-owned setup safely' }
    Write-Output "qlog is not present at $target"
}

if ($modifyPath) {
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $entries = @($userPath -split ';' | Where-Object { $_ -and $_ -ne $installDir })
    [Environment]::SetEnvironmentVariable('Path', ($entries -join ';'), 'User')
    Write-Output "removed $installDir from user PATH when present"
}

$backup = Join-Path $installDir 'qlog-user-path-backup.txt'
if (Test-Path -LiteralPath $backup) { Remove-Item -LiteralPath $backup -Force }
if (Test-Path -LiteralPath $installDir) {
    $remaining = @(Get-ChildItem -LiteralPath $installDir -Force)
    if ($remaining.Count -eq 0) { Remove-Item -LiteralPath $installDir -Force }
}
if ($purgeData) {
	Write-Output 'uninstalled qlog and removed local QUANTUM_LOG data'
} else {
	Write-Output 'uninstalled qlog; local QUANTUM_LOG data was preserved'
}
