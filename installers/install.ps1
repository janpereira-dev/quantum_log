# Install qlog from a verified GitHub Release. No administrator privileges required.
[CmdletBinding()]
param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$Arguments
)

$ErrorActionPreference = 'Stop'
$repository = if ($env:QLOG_RELEASE_REPOSITORY) { $env:QLOG_RELEASE_REPOSITORY } else { 'janpereira-dev/quantum_log' }
$releaseBase = if ($env:QLOG_RELEASE_BASE) { $env:QLOG_RELEASE_BASE } else { "https://github.com/$repository/releases/download" }
$version = $null
$channel = 'stable'
$installDir = if ($env:QLOG_INSTALL_DIR) { $env:QLOG_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\QUANTUM_LOG\bin' }
$modifyPath = $true
$dryRun = $false

function Show-Usage {
    @'
Usage: install.ps1 [options]

Options:
  --version VERSION       Install a fixed release version (for example, v1.2.3).
  --channel CHANNEL       stable (default) or latest. Both resolve GitHub's latest
                            non-prerelease release until a separate latest channel exists.
  --install-dir DIRECTORY Install qlog in DIRECTORY.
  --no-modify-path        Do not add the install directory to the user PATH.
  --dry-run               Print planned changes without downloading or writing files.
  --help                  Show this help.

Environment:
  QLOG_RELEASE_REPOSITORY GitHub owner/repository to query for releases.
  QLOG_RELEASE_BASE       HTTPS release-download base URL, without tag or filename.
'@ | Write-Output
}

function Fail([string]$Message) {
    throw "install.ps1: $Message"
}

for ($index = 0; $index -lt $Arguments.Count; $index++) {
    $argument = $Arguments[$index]
    switch -Regex ($argument) {
        '^--version$' {
            $index++
            if ($index -ge $Arguments.Count) { Fail '--version requires a value' }
            $version = $Arguments[$index]
            continue
        }
        '^--version=(.+)$' { $version = $Matches[1]; continue }
        '^--channel$' {
            $index++
            if ($index -ge $Arguments.Count) { Fail '--channel requires a value' }
            $channel = $Arguments[$index]
            continue
        }
        '^--channel=(.+)$' { $channel = $Matches[1]; continue }
        '^--install-dir$' {
            $index++
            if ($index -ge $Arguments.Count) { Fail '--install-dir requires a value' }
            $installDir = $Arguments[$index]
            continue
        }
        '^--install-dir=(.+)$' { $installDir = $Matches[1]; continue }
        '^--no-modify-path$' { $modifyPath = $false; continue }
        '^--dry-run$' { $dryRun = $true; continue }
        '^--help$|^-h$' { Show-Usage; exit 0 }
        default { Fail "unknown option: $argument" }
    }
}

if ($channel -notin @('stable', 'latest')) { Fail '--channel must be stable or latest' }
if (-not $releaseBase.StartsWith('https://', [StringComparison]::OrdinalIgnoreCase)) { Fail 'QLOG_RELEASE_BASE must use HTTPS' }
if ([string]::IsNullOrWhiteSpace($installDir)) { Fail '--install-dir cannot be empty' }

$os = 'windows'
switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
    'X64' { $arch = 'amd64' }
    'Arm64' { $arch = 'arm64' }
    default { Fail "unsupported architecture: $([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture)" }
}

if ($version) {
    if ($version -notmatch '^[0-9A-Za-z._-]+$') { Fail "invalid version: $version" }
    if ($version.StartsWith('v')) {
        $tag = $version
        $artifactVersion = $version.Substring(1)
    } else {
        $tag = "v$version"
        $artifactVersion = $version
    }
} elseif ($dryRun) {
    $tag = '<latest-release>'
    $artifactVersion = '<latest-release>'
} else {
    $apiUrl = "https://api.github.com/repos/$repository/releases/latest"
    try {
        $release = Invoke-RestMethod -Uri $apiUrl -Headers @{ 'User-Agent' = 'qlog-installer' }
        $tag = [string]$release.tag_name
    } catch {
        Fail "could not resolve latest release from ${apiUrl}: $($_.Exception.Message)"
    }
    if ([string]::IsNullOrWhiteSpace($tag)) { Fail "could not resolve a tag from $apiUrl" }
    if ($tag.StartsWith('v')) { $artifactVersion = $tag.Substring(1) } else { $artifactVersion = $tag; $tag = "v$tag" }
}

$archive = "qlog_${artifactVersion}_${os}_${arch}.zip"
$releaseUrl = "$releaseBase/$tag"
$checksumUrl = "$releaseUrl/checksums.txt"
$archiveUrl = "$releaseUrl/$archive"
Write-Output "platform: $os/$arch"
Write-Output "channel: $channel"
Write-Output "release: $tag"
Write-Output "manifest: $checksumUrl"
Write-Output "archive: $archiveUrl"
Write-Output "install dir: $installDir"

if ($dryRun) {
    Write-Output 'dry-run: no files downloaded or changed'
    exit 0
}

$workDir = Join-Path ([System.IO.Path]::GetTempPath()) ("qlog-install-" + [Guid]::NewGuid())
try {
    New-Item -ItemType Directory -Path $workDir | Out-Null
    $checksums = Join-Path $workDir 'checksums.txt'
    $archivePath = Join-Path $workDir $archive
    Invoke-WebRequest -Uri $checksumUrl -OutFile $checksums -Headers @{ 'User-Agent' = 'qlog-installer' }
    Invoke-WebRequest -Uri $archiveUrl -OutFile $archivePath -Headers @{ 'User-Agent' = 'qlog-installer' }
    $entry = Get-Content -LiteralPath $checksums | Where-Object { $_ -match "^([0-9A-Fa-f]{64})\s{2}$([Regex]::Escape($archive))$" } | Select-Object -First 1
    if (-not $entry) { Fail "checksum manifest has no SHA-256 entry for $archive" }
    $expected = ($entry -split '\s+')[0].ToLowerInvariant()
    $actual = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { Fail "SHA-256 verification failed for $archive" }

    $extractDir = Join-Path $workDir 'extract'
    Expand-Archive -LiteralPath $archivePath -DestinationPath $extractDir -Force
    $binaries = @(Get-ChildItem -LiteralPath $extractDir -Recurse -File -Filter 'qlog.exe')
    if ($binaries.Count -ne 1) { Fail 'release archive must contain exactly one qlog.exe' }
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    $target = Join-Path $installDir 'qlog.exe'
    $staged = Join-Path $installDir ('.qlog-' + [Guid]::NewGuid() + '.exe')
    Copy-Item -LiteralPath $binaries[0].FullName -Destination $staged -Force
    Move-Item -LiteralPath $staged -Destination $target -Force
    & $target '--version'
    if ($LASTEXITCODE -ne 0) { Fail "installed qlog failed its version check (exit $LASTEXITCODE)" }

    if ($modifyPath) {
        $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        $entries = @($userPath -split ';' | Where-Object { $_ })
        if ($entries -notcontains $installDir) {
            $backup = Join-Path $installDir 'qlog-user-path-backup.txt'
            Set-Content -LiteralPath $backup -Value $userPath -NoNewline
            [Environment]::SetEnvironmentVariable('Path', (($entries + $installDir) -join ';'), 'User')
            $env:Path = "$installDir;$env:Path"
            Write-Output "backed up user PATH to $backup"
            Write-Output "updated user PATH with $installDir"
        } else {
            Write-Output "user PATH already contains $installDir"
        }
    } else {
        Write-Output "PATH was not modified; add $installDir to PATH to run qlog by name"
    }
    Write-Output "installed qlog at $target"
    Write-Output 'next: qlog doctor'
} finally {
    if (Test-Path -LiteralPath $workDir) { Remove-Item -LiteralPath $workDir -Recurse -Force }
}
