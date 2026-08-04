[CmdletBinding()]
param(
    [string]$ArtifactPath,
    [string]$EvidenceRoot,
    [switch]$PostAgent
)

$ErrorActionPreference = 'Stop'
$ExpectedVersion = 'v0.3.2-rc.1'
$ExpectedCLIStandardVersion = '0.3.2-rc.1'
$ExpectedArchive = 'qlog_0.3.2-rc.1_windows_amd64.zip'
$ExpectedSHA256 = '1e343d21c71ef78fd32a7694c8a30d7031e30596e6bfded2991d917501ee14e3'
$script:Results = @()
$script:CommandNumber = 0
$script:ForegroundCollector = $null

function Stop-WithFailure([int]$Code, [string]$Message) {
    Write-Error "P0-09 failure [$Code]: $Message"
    exit $Code
}

function Get-SHA256([string]$Path) {
    $stream = [System.IO.File]::OpenRead($Path)
    try {
        $sha = [System.Security.Cryptography.SHA256]::Create()
        try {
            return ([System.BitConverter]::ToString($sha.ComputeHash($stream))).Replace('-', '').ToLowerInvariant()
        } finally {
            $sha.Dispose()
        }
    } finally {
        $stream.Dispose()
    }
}

function Write-Transcript([string]$Name, [string[]]$Lines) {
    $path = Join-Path $script:Logs "$Name.log"
    Set-Content -LiteralPath $path -Value $Lines -Encoding utf8
    return $path
}

function Invoke-Qlog([string]$Name, [string[]]$Arguments, [switch]$AllowFailure) {
    $script:CommandNumber++
    $logName = ('{0:D2}-{1}' -f $script:CommandNumber, $Name)
    $started = Get-Date
    $savedErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $output = @(& $script:Qlog @Arguments 2>&1 | ForEach-Object { $_.ToString() })
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $savedErrorActionPreference
    }
    $log = Write-Transcript $logName (@("COMMAND: $script:Qlog $($Arguments -join ' ')", "STARTED: $started", "EXIT: $exitCode", '') + $output)
    $record = [pscustomobject]@{
        name = $Name
        arguments = $Arguments
        exit_code = $exitCode
        log = $log
    }
    $script:Results += $record
    if ($exitCode -ne 0 -and -not $AllowFailure) {
        Stop-WithFailure 20 "$Name failed with exit $exitCode. See $log"
    }
    return [pscustomobject]@{ ExitCode = $exitCode; Output = ($output -join [Environment]::NewLine); Log = $log }
}

function Start-ForegroundCollector {
    $stdout = Join-Path $script:Logs 'collector-foreground.stdout.log'
    $stderr = Join-Path $script:Logs 'collector-foreground.stderr.log'
    $collectorLog = Join-Path $script:Evidence 'collector.log'
    $arguments = "--home `"$script:QlogHome`" collector serve --log-file `"$collectorLog`""
    $script:ForegroundCollector = Start-Process -FilePath $script:Qlog -ArgumentList $arguments -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    Start-Sleep -Seconds 2
    return $collectorLog
}

function Write-RealAgentGuide([string]$CollectorMode) {
    $guide = @"
P0-09 real-agent continuation

Evidence root: $script:Root
Candidate executable: $script:Qlog
QLOG_HOME: $script:QlogHome
Adapter config home: $script:AdapterConfigHome
Collector mode: $CollectorMode

Do not inject OTLP, qlog JSON events, or synthetic telemetry.
In a separate terminal, use one installed detected agent normally against this clean project:
  $script:ProjectRoot

Then run:
  powershell -NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`" -EvidenceRoot `"$script:Root`" -PostAgent

Post-agent run automatically records adapter verification, project and daily usage,
export, append-only ledger verification, doctor, managed restart result, and collector status.
"@
    Set-Content -LiteralPath (Join-Path $script:Evidence 'NEXT-REAL-AGENT-ACTION.txt') -Value $guide -Encoding utf8
}

if (-not $ArtifactPath) {
    $ArtifactPath = Join-Path $PSScriptRoot "..\dist\$ExpectedArchive"
}
try {
    $ArtifactPath = (Resolve-Path -LiteralPath $ArtifactPath).Path
} catch {
    Stop-WithFailure 10 "candidate artifact does not exist: $ArtifactPath"
}
if ((Split-Path -Leaf $ArtifactPath) -ne $ExpectedArchive) {
    Stop-WithFailure 10 "candidate artifact must be $ExpectedArchive, got $(Split-Path -Leaf $ArtifactPath)"
}
if ((Get-SHA256 $ArtifactPath) -ne $ExpectedSHA256) {
    Stop-WithFailure 10 "candidate checksum does not match generated v0.3.2-rc.1 checksum"
}

if ($EvidenceRoot) {
    try {
        $script:Root = (Resolve-Path -LiteralPath $EvidenceRoot).Path
    } catch {
        Stop-WithFailure 12 "existing evidence root does not exist: $EvidenceRoot"
    }
} else {
    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
    $script:Root = Join-Path ([System.IO.Path]::GetTempPath()) "qlog-p0-09-$stamp"
    New-Item -ItemType Directory -Path $script:Root -Force | Out-Null
}
$script:Logs = Join-Path $script:Root 'logs'
$script:Evidence = Join-Path $script:Root 'evidence'
$script:InstallRoot = Join-Path $script:Root 'install'
$script:QlogHome = Join-Path $script:Root 'qlog-home'
$script:AdapterConfigHome = Join-Path $script:Root 'adapter-config'
$script:ProjectRoot = Join-Path $script:Root 'clean-project'
@($script:Logs, $script:Evidence, $script:InstallRoot, $script:QlogHome, $script:AdapterConfigHome, $script:ProjectRoot) | ForEach-Object {
    New-Item -ItemType Directory -Path $_ -Force | Out-Null
}

$env:QLOG_HOME = $script:QlogHome
$env:QLOG_ADAPTER_CONFIG_HOME = $script:AdapterConfigHome

try {
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $archive = [System.IO.Compression.ZipFile]::OpenRead($ArtifactPath)
    try {
        $entries = @($archive.Entries | Where-Object { $_.FullName -match '(^|/)qlog\.exe$' })
        if ($entries.Count -ne 1) {
            Stop-WithFailure 12 "candidate archive must contain exactly one qlog.exe; found $($entries.Count)"
        }
    } finally {
        $archive.Dispose()
    }
    Expand-Archive -LiteralPath $ArtifactPath -DestinationPath $script:InstallRoot -Force
} catch {
    Stop-WithFailure 12 "candidate archive cannot be extracted: $($_.Exception.Message)"
}

$qlogFiles = @(Get-ChildItem -LiteralPath $script:InstallRoot -Recurse -File -Filter qlog.exe)
if ($qlogFiles.Count -ne 1) {
    Stop-WithFailure 12 "extracted candidate must contain exactly one qlog.exe; found $($qlogFiles.Count)"
}
$script:Qlog = $qlogFiles[0].FullName

try {
    $version = Invoke-Qlog 'version' @('--version')
    if ($version.Output -notmatch [regex]::Escape("qlog $ExpectedCLIStandardVersion")) {
        Stop-WithFailure 11 "candidate version mismatch. Expected qlog $ExpectedCLIStandardVersion. See $($version.Log)"
    }

    Set-Location $script:ProjectRoot
    Invoke-Qlog 'init' @('--home', $script:QlogHome, 'init') | Out-Null
    Invoke-Qlog 'project-register' @('--home', $script:QlogHome, 'project', 'register', '--path', $script:ProjectRoot, '--name', 'P0-09 Smoke', '--slug', 'p0-09-smoke') | Out-Null
    Invoke-Qlog 'adapter-detect-before' @('--home', $script:QlogHome, 'adapter', 'detect', '--json') | Out-Null

    $setupFirst = Invoke-Qlog 'setup-first' @('--home', $script:QlogHome, 'setup', '--yes', '--json') -AllowFailure
    $setupSecond = Invoke-Qlog 'setup-second' @('--home', $script:QlogHome, 'setup', '--yes', '--json') -AllowFailure
    $policyDenied = $setupFirst.Output -match '(?i)task scheduler.*(access|acceso).*denied|task scheduler.*denegado'
    $policyDenied = $policyDenied -and $setupSecond.Output -match '(?i)task scheduler.*(access|acceso).*denied|task scheduler.*denegado'
    if ($policyDenied) {
        $setupState = 'BLOCKED_EXTERNAL_POLICY'
    } elseif ($setupFirst.ExitCode -ne 0 -or $setupSecond.ExitCode -ne 0) {
        Stop-WithFailure 20 "setup failed without exact Task Scheduler policy diagnosis. See $($setupFirst.Log) and $($setupSecond.Log)"
    } elseif ($setupSecond.Output -notmatch 'unchanged') {
        Stop-WithFailure 21 "second setup did not report unchanged idempotent state. See $($setupSecond.Log)"
    } else {
        $setupState = 'IDEMPOTENT'
    }

    Invoke-Qlog 'adapter-detect-after' @('--home', $script:QlogHome, 'adapter', 'detect', '--json') | Out-Null
    Invoke-Qlog 'adapter-status' @('--home', $script:QlogHome, 'adapter', 'status', '--json') | Out-Null
    $collectorStatus = Invoke-Qlog 'collector-status-managed' @('--home', $script:QlogHome, 'collector', 'status', '--json') -AllowFailure
    $collectorMode = 'managed'
    if ($policyDenied) {
        $collectorMode = 'foreground-policy-fallback'
        Start-ForegroundCollector | Out-Null
        $foregroundStatus = Invoke-Qlog 'collector-status-foreground' @('--home', $script:QlogHome, 'collector', 'status', '--json') -AllowFailure
        if ($foregroundStatus.ExitCode -ne 0 -or $foregroundStatus.Output -notmatch '"reachable"\s*:\s*true') {
            Stop-WithFailure 22 "Task Scheduler policy denied managed collector and foreground fallback is not reachable. See $($foregroundStatus.Log)"
        }
    } elseif ($collectorStatus.ExitCode -ne 0 -or $collectorStatus.Output -notmatch '"reachable"\s*:\s*true') {
        Stop-WithFailure 22 "collector is not enabled and no exact Task Scheduler policy diagnosis was recorded. See $($collectorStatus.Log)"
    }

    # No agent action occurs in this harness. These expected failures prove empty-ledger behavior only.
    foreach ($adapter in @('claude-code', 'codex', 'copilot', 'copilot-vscode', 'opencode')) {
        Invoke-Qlog "adapter-verify-negative-$adapter" @('--home', $script:QlogHome, 'adapter', 'verify', $adapter, '--project', 'p0-09-smoke', '--since', '5m', '--json') -AllowFailure | Out-Null
    }
    Invoke-Qlog 'usage-project-negative' @('--home', $script:QlogHome, 'usage', 'project', 'p0-09-smoke', '--json') | Out-Null
    Invoke-Qlog 'usage-today-negative' @('--home', $script:QlogHome, 'usage', 'today', '--json') | Out-Null
    $today = Get-Date -Format 'yyyy-MM-dd'
    $tomorrow = (Get-Date).AddDays(1).ToString('yyyy-MM-dd')
    Invoke-Qlog 'export-negative' @('--home', $script:QlogHome, 'export', '--format', 'json', '--from', $today, '--to', $tomorrow) | Out-Null

    if ($PostAgent) {
        Invoke-Qlog 'collector-restart-post-agent' @('--home', $script:QlogHome, 'collector', 'restart', '--json') -AllowFailure | Out-Null
        Invoke-Qlog 'collector-status-post-agent' @('--home', $script:QlogHome, 'collector', 'status', '--json') -AllowFailure | Out-Null
        Invoke-Qlog 'adapter-status-post-agent' @('--home', $script:QlogHome, 'adapter', 'status', '--json') | Out-Null
        foreach ($adapter in @('claude-code', 'codex', 'copilot', 'copilot-vscode', 'opencode')) {
            Invoke-Qlog "adapter-verify-post-agent-$adapter" @('--home', $script:QlogHome, 'adapter', 'verify', $adapter, '--project', 'p0-09-smoke', '--since', '1h', '--json') -AllowFailure | Out-Null
        }
        Invoke-Qlog 'usage-project-post-agent' @('--home', $script:QlogHome, 'usage', 'project', 'p0-09-smoke', '--json') | Out-Null
        Invoke-Qlog 'usage-today-post-agent' @('--home', $script:QlogHome, 'usage', 'today', '--json') | Out-Null
        Invoke-Qlog 'export-post-agent' @('--home', $script:QlogHome, 'export', '--format', 'json', '--from', $today, '--to', $tomorrow) | Out-Null
    }

    Invoke-Qlog 'verify' @('--home', $script:QlogHome, 'verify') | Out-Null
    Invoke-Qlog 'doctor' @('--home', $script:QlogHome, 'doctor', '--json') | Out-Null
    Write-RealAgentGuide $collectorMode
    [pscustomobject]@{
        status = if ($policyDenied) { 'BLOCKED_EXTERNAL_POLICY' } else { 'PASS_PRE_AGENT' }
        candidate_version = $ExpectedVersion
        candidate_archive = $ArtifactPath
        candidate_sha256 = $ExpectedSHA256
        qlog = $script:Qlog
        qlog_home = $script:QlogHome
        adapter_config_home = $script:AdapterConfigHome
        project = 'p0-09-smoke'
        collector_mode = $collectorMode
        setup_state = $setupState
        post_agent = [bool]$PostAgent
        commands = $script:Results
    } | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $script:Evidence 'summary.json') -Encoding utf8
    if ($policyDenied) {
        Write-Output "P0-09 BLOCKED_EXTERNAL_POLICY [23]. Evidence: $script:Root"
        exit 23
    }
    Write-Output "P0-09 PASS_PRE_AGENT. Evidence: $script:Root"
} finally {
    if ($script:ForegroundCollector -and -not $script:ForegroundCollector.HasExited) {
        Stop-Process -Id $script:ForegroundCollector.Id -Force
    }
}
