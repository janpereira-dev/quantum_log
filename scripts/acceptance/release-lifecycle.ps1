[CmdletBinding()]
param([switch]$ContractOnly)

$ErrorActionPreference = 'Stop'
$passLine = 'PASS contract: explicit versions and isolated home'
$tempBase = if ($env:TEMP) { $env:TEMP } elseif ($env:TMP) { $env:TMP } else { [System.IO.Path]::GetTempPath() }
$runRoot = Join-Path $tempBase ("qlog-release-lifecycle-{0}" -f [guid]::NewGuid())
$fromVersion = $env:QLOG_FROM_VERSION
$toVersion = $env:QLOG_TO_VERSION
$releaseBase = $env:QLOG_RELEASE_BASE
if ([string]::IsNullOrWhiteSpace($fromVersion)) { throw 'QLOG_FROM_VERSION is required' }
if ([string]::IsNullOrWhiteSpace($toVersion)) { throw 'QLOG_TO_VERSION is required' }
if ($fromVersion -eq $toVersion) { throw 'QLOG_FROM_VERSION and QLOG_TO_VERSION must differ' }
if ($ContractOnly -and ($fromVersion -notmatch '^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$' -or $toVersion -notmatch '^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$')) { throw 'release versions must be immutable tags (vMAJOR.MINOR.PATCH[-suffix])' }
if ([string]::IsNullOrWhiteSpace($releaseBase)) { throw 'QLOG_RELEASE_BASE is required' }
if (-not $releaseBase.StartsWith('https://', [StringComparison]::OrdinalIgnoreCase)) { throw 'QLOG_RELEASE_BASE must use HTTPS' }

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

$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$installer = if ($env:QLOG_INSTALLER_PS1) { $env:QLOG_INSTALLER_PS1 } else { Join-Path $repoRoot 'installers/install.ps1' }
$uninstaller = if ($env:QLOG_UNINSTALLER_PS1) { $env:QLOG_UNINSTALLER_PS1 } else { Join-Path $repoRoot 'installers/uninstall.ps1' }
$evidenceDir = if ($env:QLOG_EVIDENCE_DIR) { $env:QLOG_EVIDENCE_DIR } else { Join-Path $tempBase ("qlog-release-evidence-{0}" -f [guid]::NewGuid()) }
$installDir = Join-Path $runRoot 'bin'
$originalEnvironment = @{}
foreach ($name in @('HOME', 'USERPROFILE', 'APPDATA', 'LOCALAPPDATA', 'XDG_CONFIG_HOME', 'XDG_DATA_HOME')) { $originalEnvironment[$name] = [Environment]::GetEnvironmentVariable($name) }
$isolatedUserHome = Join-Path $runRoot 'user-home'
$env:HOME = $isolatedUserHome
$env:USERPROFILE = $isolatedUserHome
$env:APPDATA = Join-Path $isolatedUserHome 'AppData/Roaming'
$env:LOCALAPPDATA = Join-Path $isolatedUserHome 'AppData/Local'
$env:XDG_CONFIG_HOME = Join-Path $isolatedUserHome '.config'
$env:XDG_DATA_HOME = Join-Path $isolatedUserHome '.local/share'
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
    $jsonEscapedRoot = $runRoot.Replace('\', '\\')
    $sanitized = $Text.Replace($jsonEscapedRoot, '<TEMP>').Replace($runRoot, '<TEMP>')
    # Go can render the disposable root with a short 8.3 user component or a
    # different separator. Redact the lifecycle anchor without dropping the
    # diagnostic text or its useful suffix.
    $sanitized = [regex]::Replace($sanitized, '(?i)[A-Z]:[\\/][^\s"'']*[\\/]qlog-release-lifecycle[^\s"'']*', '<TEMP>')
    $sanitized = [regex]::Replace($sanitized, '(?i)/[^\s"'']*/qlog-release-lifecycle[^\s"'']*', '<TEMP>')
    $sanitized | Set-Content -LiteralPath $Path -Encoding UTF8
}

function Get-SHA256([string]$Path) {
    if (Get-Command Get-FileHash -ErrorAction SilentlyContinue) {
        return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash
    }
    $stream = [System.IO.File]::OpenRead($Path)
    try {
        $sha256 = [System.Security.Cryptography.SHA256]::Create()
        try { return ([BitConverter]::ToString($sha256.ComputeHash($stream))).Replace('-', '') }
        finally { $sha256.Dispose() }
    } finally { $stream.Dispose() }
}

function Assert-Version([string]$Text, [string]$Requested, [string]$Stage) {
    $match = [regex]::Match($Text, '^qlog ([^ ]+) \(commit [^\r\n]+\)$')
    $normalized = $Requested -replace '^v', ''
    if (-not $match.Success -or $match.Groups[1].Value -cne $normalized) {
        throw "installed $Stage version does not match requested version"
    }
}

try {
    New-Item -ItemType Directory -Path $runRoot, $evidenceDir -Force | Out-Null
    Set-Content -LiteralPath $commands -Value '' -Encoding UTF8
    Invoke-Recorded 'install-from' { Invoke-Installer -InstallerArguments @('--version', $fromVersion, '--install-dir', $installDir, '--no-modify-path', '--no-bootstrap') }
    $qlog = Join-Path $installDir 'qlog.exe'
    $beforeVersionText = (& $qlog --version | Out-String).Trim()
    $beforeVersionText | Set-Content -LiteralPath (Join-Path $evidenceDir 'before-version.txt') -Encoding UTF8
    Assert-Version $beforeVersionText $fromVersion 'source'
    '{"source":"release-lifecycle","source_version":"1","session_id":"release-lifecycle-sentinel","event_type":"lifecycle.sentinel","occurred_at":"2026-01-01T00:00:00Z","payload":{"capture_quality":"lifecycle_only","sentinel":"qlog-release-lifecycle-v1"}}' | Set-Content -LiteralPath $fixture -Encoding ASCII
    Invoke-Recorded 'init' { $arguments = @('--home', $env:QLOG_HOME, 'init'); & $qlog @arguments | Out-Null }
    Invoke-Recorded 'ingest' { $arguments = @('--home', $env:QLOG_HOME, 'ingest', 'file', $fixture); & $qlog @arguments | Out-Null }
    if (-not (Test-Path -LiteralPath $ledger -PathType Leaf)) { throw 'qlog.db was not created' }
    Get-SHA256 $ledger | Set-Content -LiteralPath (Join-Path $evidenceDir 'ledger-before.sha256') -Encoding ASCII

    Invoke-Recorded 'install-to' { Invoke-Installer -InstallerArguments @('--version', $toVersion, '--install-dir', $installDir, '--no-modify-path', '--no-bootstrap') }
    $afterVersionText = (& $qlog --version | Out-String).Trim()
    $afterVersionText | Set-Content -LiteralPath (Join-Path $evidenceDir 'after-version.txt') -Encoding UTF8
    Assert-Version $afterVersionText $toVersion 'target'
    Invoke-Recorded 'migrate' { $arguments = @('--home', $env:QLOG_HOME, 'migrate'); & $qlog @arguments | Out-Null }
    Get-SHA256 $ledger | Set-Content -LiteralPath (Join-Path $evidenceDir 'ledger-after-migration.sha256') -Encoding ASCII
    $verifyMigration = & $qlog --home $env:QLOG_HOME verify 2>&1 | Out-String
    $verifyMigrationCode = $LASTEXITCODE
    Add-Content -LiteralPath $commands -Value "verify-migration`t$verifyMigrationCode" -Encoding UTF8
    Write-Sanitized $verifyMigration (Join-Path $evidenceDir 'verify-migration.txt')
    if ($verifyMigrationCode -ne 0) { throw 'verify after migration failed' }
    $arguments = @('--home', $env:QLOG_HOME, 'doctor', '--json')
    $doctor = & $qlog @arguments 2>&1 | Out-String
    $doctorCode = $LASTEXITCODE
    Add-Content -LiteralPath $commands -Value "doctor`t$doctorCode" -Encoding UTF8
    Write-Sanitized $doctor (Join-Path $evidenceDir 'doctor.json')
    if ($doctorCode -ne 0) { throw 'doctor failed' }
    $arguments = @('--home', $env:QLOG_HOME, 'verify')
    $verify = & $qlog @arguments 2>&1 | Out-String
    $verifyCode = $LASTEXITCODE
    Add-Content -LiteralPath $commands -Value "verify`t$verifyCode" -Encoding UTF8
    Write-Sanitized $verify (Join-Path $evidenceDir 'verify.txt')
    if ($verifyCode -ne 0) { throw 'verify failed' }
    $beforeHash = (Get-Content -LiteralPath (Join-Path $evidenceDir 'ledger-before.sha256') -Raw).Trim()
    $upgradeHash = Get-SHA256 $ledger
    $upgradeHash | Set-Content -LiteralPath (Join-Path $evidenceDir 'ledger-after-upgrade.sha256') -Encoding ASCII
    $migrationHash = (Get-Content -LiteralPath (Join-Path $evidenceDir 'ledger-after-migration.sha256') -Raw).Trim()
    if ($migrationHash -ne $upgradeHash) { throw 'ledger hash changed during upgrade diagnostics' }

    Invoke-Recorded 'uninstall' { $arguments = @('--install-dir', $installDir, '--no-modify-path'); & $uninstaller @arguments | Out-Null }
    if (-not (Test-Path -LiteralPath $ledger -PathType Leaf)) { throw 'qlog.db was removed by uninstall' }
    $uninstallHash = Get-SHA256 $ledger
    $uninstallHash | Set-Content -LiteralPath (Join-Path $evidenceDir 'ledger-after-uninstall.sha256') -Encoding ASCII
    if ($migrationHash -ne $uninstallHash) { throw 'ledger hash changed during uninstall' }

    Invoke-Recorded 'reinstall-to' { Invoke-Installer -InstallerArguments @('--version', $toVersion, '--install-dir', $installDir, '--no-modify-path', '--no-bootstrap') }
    $arguments = @('--home', $env:QLOG_HOME, 'verify')
    $verifyReinstall = & $qlog @arguments 2>&1 | Out-String
    $verifyReinstallCode = $LASTEXITCODE
    Add-Content -LiteralPath $commands -Value "verify-reinstall`t$verifyReinstallCode" -Encoding UTF8
    Write-Sanitized $verifyReinstall (Join-Path $evidenceDir 'verify-reinstall.txt')
    if ($verifyReinstallCode -ne 0) { throw 'verify after reinstall failed' }
    $reinstallHash = Get-SHA256 $ledger
    $reinstallHash | Set-Content -LiteralPath (Join-Path $evidenceDir 'ledger-after-reinstall.sha256') -Encoding ASCII
    if ($migrationHash -ne $reinstallHash) { throw 'ledger hash changed during reinstall' }
    Write-Output "PASS lifecycle: $fromVersion -> $toVersion"
    Write-Output 'evidence: <TEMP>'
} finally {
    foreach ($name in $originalEnvironment.Keys) { [Environment]::SetEnvironmentVariable($name, $originalEnvironment[$name]) }
    if (Test-Path -LiteralPath $runRoot) { Remove-Item -LiteralPath $runRoot -Recurse -Force }
}
