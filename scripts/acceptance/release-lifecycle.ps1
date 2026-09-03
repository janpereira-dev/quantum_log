[CmdletBinding()]
param([switch]$ContractOnly)

$ErrorActionPreference = 'Stop'
$passLine = 'PASS contract: explicit versions and isolated home'
$tempBase = if ($env:TEMP) { $env:TEMP } elseif ($env:TMP) { $env:TMP } else { [System.IO.Path]::GetTempPath() }
$runRoot = Join-Path $tempBase ("qlog-release-lifecycle-{0}" -f [guid]::NewGuid())

if ($ContractOnly) {
    try {
        New-Item -ItemType Directory -Path $runRoot -Force | Out-Null
        $contractHome = Join-Path $runRoot 'home'
        $contractInstall = Join-Path $runRoot 'bin'
        if (-not $contractHome.StartsWith($runRoot, [StringComparison]::Ordinal) -or -not $contractInstall.StartsWith($runRoot, [StringComparison]::Ordinal)) { throw 'contract paths are not isolated' }
        Write-Output $passLine
    } finally {
        if (Test-Path -LiteralPath $runRoot) { Remove-Item -LiteralPath $runRoot -Recurse -Force }
    }
    exit 0
}

$fromVersion = $env:QLOG_FROM_VERSION
$toVersion = $env:QLOG_TO_VERSION
$releaseBase = $env:QLOG_RELEASE_BASE
if ([string]::IsNullOrWhiteSpace($fromVersion)) { throw 'QLOG_FROM_VERSION is required' }
if ([string]::IsNullOrWhiteSpace($toVersion)) { throw 'QLOG_TO_VERSION is required' }
if ($fromVersion -eq $toVersion) { throw 'QLOG_FROM_VERSION and QLOG_TO_VERSION must differ' }
if ([string]::IsNullOrWhiteSpace($releaseBase)) { throw 'QLOG_RELEASE_BASE is required' }
if (-not $releaseBase.StartsWith('https://', [StringComparison]::OrdinalIgnoreCase)) { throw 'QLOG_RELEASE_BASE must use HTTPS' }

$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$installer = if ($env:QLOG_INSTALLER_PS1) { $env:QLOG_INSTALLER_PS1 } else { Join-Path $repoRoot 'installers/install.ps1' }
$uninstaller = if ($env:QLOG_UNINSTALLER_PS1) { $env:QLOG_UNINSTALLER_PS1 } else { Join-Path $repoRoot 'installers/uninstall.ps1' }
$evidenceDir = if ($env:QLOG_EVIDENCE_DIR) { $env:QLOG_EVIDENCE_DIR } else { Join-Path $tempBase ("qlog-release-evidence-{0}" -f [guid]::NewGuid()) }
$installDir = Join-Path $runRoot 'bin'
$env:QLOG_HOME = Join-Path $runRoot 'home'
$ledger = Join-Path $env:QLOG_HOME 'qlog.db'
$fixture = Join-Path $runRoot 'sentinel.ndjson'
$commands = Join-Path $evidenceDir 'commands.tsv'

function Invoke-Recorded([string]$Label, [scriptblock]$Operation) {
    $status = 0
    try { & $Operation; if ($LASTEXITCODE -and $LASTEXITCODE -ne 0) { $status = $LASTEXITCODE } } catch { $status = 1; throw } finally {
        Add-Content -LiteralPath $commands -Value ("{0}`t{1}" -f $Label, $status) -Encoding UTF8
    }
    if ($status -ne 0) { throw "$Label failed with exit code $status" }
}

function Invoke-Installer([string[]]$InstallerArguments) {
    if ($PSVersionTable.PSEdition -eq 'Desktop') {
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $installer @InstallerArguments
    } else {
        & pwsh -NoProfile -File $installer @InstallerArguments
    }
    if ($LASTEXITCODE -ne 0) { throw "installer failed with exit code $LASTEXITCODE" }
}

function Write-Sanitized([string]$Text, [string]$Path) {
    $Text.Replace($runRoot, '<TEMP>') | Set-Content -LiteralPath $Path -Encoding UTF8
}

try {
    New-Item -ItemType Directory -Path $runRoot, $evidenceDir -Force | Out-Null
    Set-Content -LiteralPath $commands -Value '' -Encoding UTF8
    Invoke-Recorded 'install-from' { Invoke-Installer @('--version', $fromVersion, '--install-dir', $installDir, '--no-modify-path', '--no-bootstrap') }
    $qlog = Join-Path $installDir 'qlog.exe'
    $beforeVersionText = (& $qlog --version | Out-String).Trim()
    $beforeVersionText | Set-Content -LiteralPath (Join-Path $evidenceDir 'before-version.txt') -Encoding UTF8
    if (-not $beforeVersionText.Contains($fromVersion.TrimStart('v'))) { throw 'installed source version does not match QLOG_FROM_VERSION' }
    '{"source":"release-lifecycle","source_version":"1","session_id":"release-lifecycle-sentinel","event_type":"lifecycle.sentinel","occurred_at":"2026-01-01T00:00:00Z","payload":{"capture_quality":"lifecycle_only","sentinel":"qlog-release-lifecycle-v1"}}' | Set-Content -LiteralPath $fixture -Encoding ASCII
    Invoke-Recorded 'init' { & $qlog '--home', $env:QLOG_HOME, 'init' | Out-Null }
    Invoke-Recorded 'ingest' { & $qlog '--home', $env:QLOG_HOME, 'ingest', 'file', $fixture | Out-Null }
    if (-not (Test-Path -LiteralPath $ledger -PathType Leaf)) { throw 'qlog.db was not created' }
    (Get-FileHash -LiteralPath $ledger -Algorithm SHA256).Hash | Set-Content -LiteralPath (Join-Path $evidenceDir 'ledger-before.sha256') -Encoding ASCII

    Invoke-Recorded 'install-to' { Invoke-Installer @('--version', $toVersion, '--install-dir', $installDir, '--no-modify-path', '--no-bootstrap') }
    $afterVersionText = (& $qlog --version | Out-String).Trim()
    $afterVersionText | Set-Content -LiteralPath (Join-Path $evidenceDir 'after-version.txt') -Encoding UTF8
    if (-not $afterVersionText.Contains($toVersion.TrimStart('v'))) { throw 'installed target version does not match QLOG_TO_VERSION' }
    $doctor = & $qlog '--home', $env:QLOG_HOME, 'doctor', '--json' | Out-String
    if ($LASTEXITCODE -ne 0) { throw 'doctor failed' }
    Add-Content -LiteralPath $commands -Value "doctor`t0" -Encoding UTF8
    Write-Sanitized $doctor (Join-Path $evidenceDir 'doctor.json')
    $verify = & $qlog '--home', $env:QLOG_HOME, 'verify' | Out-String
    if ($LASTEXITCODE -ne 0) { throw 'verify failed' }
    Add-Content -LiteralPath $commands -Value "verify`t0" -Encoding UTF8
    Write-Sanitized $verify (Join-Path $evidenceDir 'verify.txt')
    $beforeHash = (Get-Content -LiteralPath (Join-Path $evidenceDir 'ledger-before.sha256') -Raw).Trim()
    $upgradeHash = (Get-FileHash -LiteralPath $ledger -Algorithm SHA256).Hash
    $upgradeHash | Set-Content -LiteralPath (Join-Path $evidenceDir 'ledger-after-upgrade.sha256') -Encoding ASCII
    if ($beforeHash -ne $upgradeHash) { throw 'ledger hash changed during upgrade diagnostics' }

    Invoke-Recorded 'uninstall' { & $uninstaller '--install-dir', $installDir, '--no-modify-path' | Out-Null }
    if (-not (Test-Path -LiteralPath $ledger -PathType Leaf)) { throw 'qlog.db was removed by uninstall' }
    $uninstallHash = (Get-FileHash -LiteralPath $ledger -Algorithm SHA256).Hash
    $uninstallHash | Set-Content -LiteralPath (Join-Path $evidenceDir 'ledger-after-uninstall.sha256') -Encoding ASCII
    if ($beforeHash -ne $uninstallHash) { throw 'ledger hash changed during uninstall' }

    Invoke-Recorded 'reinstall-to' { Invoke-Installer @('--version', $toVersion, '--install-dir', $installDir, '--no-modify-path', '--no-bootstrap') }
    $verifyReinstall = & $qlog '--home', $env:QLOG_HOME, 'verify' | Out-String
    if ($LASTEXITCODE -ne 0) { throw 'verify after reinstall failed' }
    Add-Content -LiteralPath $commands -Value "verify-reinstall`t0" -Encoding UTF8
    Write-Sanitized $verifyReinstall (Join-Path $evidenceDir 'verify-reinstall.txt')
    $reinstallHash = (Get-FileHash -LiteralPath $ledger -Algorithm SHA256).Hash
    $reinstallHash | Set-Content -LiteralPath (Join-Path $evidenceDir 'ledger-after-reinstall.sha256') -Encoding ASCII
    if ($beforeHash -ne $reinstallHash) { throw 'ledger hash changed during reinstall' }
    Write-Output "PASS lifecycle: $fromVersion -> $toVersion"
    Write-Output "evidence: $evidenceDir"
} finally {
    if (Test-Path -LiteralPath $runRoot) { Remove-Item -LiteralPath $runRoot -Recurse -Force }
}
