#requires -Version 7.0
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)] [string]$SourceFile,
    [string]$Binary = (Join-Path $PSScriptRoot '../dist/simple/qlog.exe'),
    [string]$OutputDirectory = (Join-Path $PSScriptRoot '../dist/simple-verification')
)

$ErrorActionPreference = 'Stop'
$binaryPath = (Resolve-Path -LiteralPath $Binary).Path
$sourcePath = (Resolve-Path -LiteralPath $SourceFile).Path
$outputRoot = [System.IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Path $outputRoot -Force | Out-Null
# A fresh directory ensures every run measures a new isolated ledger.
$ledger = Join-Path $outputRoot ('ledger-' + [guid]::NewGuid().ToString('N'))
$sourceBefore = (Get-FileHash -LiteralPath $sourcePath -Algorithm SHA256).Hash
# Independently check the final native total, without copying content to evidence.
$nativeTotal = $null
$nativeReset = $false
foreach ($line in [System.IO.File]::ReadLines($sourcePath)) {
    $record = $line | ConvertFrom-Json
    if ($record.type -eq 'event_msg' -and $record.payload.type -eq 'token_count' -and
        $null -ne $record.payload.info.total_token_usage.total_tokens) {
        $value = [long]$record.payload.info.total_token_usage.total_tokens
        if ($null -ne $nativeTotal -and $value -lt $nativeTotal) { $nativeReset = $true }
        $nativeTotal = $value
    }
}
if ($null -eq $nativeTotal -or $nativeReset) { throw 'Choose a finished session with monotonic native usage totals.' }

function Invoke-MeasuredQlog {
    param([string[]]$Arguments)
    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $process.StartInfo.FileName = $binaryPath
    $process.StartInfo.UseShellExecute = $false
    $process.StartInfo.CreateNoWindow = $true
    $process.StartInfo.RedirectStandardOutput = $true
    $process.StartInfo.RedirectStandardError = $true
    foreach ($argument in $Arguments) { $process.StartInfo.ArgumentList.Add($argument) }
    $watch = [System.Diagnostics.Stopwatch]::StartNew()
    try {
        if (-not $process.Start()) { throw 'Could not start the simple executable.' }
        $stdout = $process.StandardOutput.ReadToEndAsync()
        $stderr = $process.StandardError.ReadToEndAsync()
        $peak = 0L
        while (-not $process.HasExited) {
            $process.Refresh()
            try { $peak = [math]::Max($peak, $process.PeakWorkingSet64) } catch { }
            if ($watch.Elapsed.TotalSeconds -gt 60) {
                $process.Kill()
                throw 'Simple verification timed out after 60 seconds.'
            }
            Start-Sleep -Milliseconds 5
        }
        $process.WaitForExit()
        $watch.Stop()
        $text = $stdout.GetAwaiter().GetResult()
        $errors = $stderr.GetAwaiter().GetResult()
        if ($process.ExitCode -ne 0) { throw "qlog exited with $($process.ExitCode): $errors" }
        return [pscustomobject]@{
            elapsed_ms = [math]::Round($watch.Elapsed.TotalMilliseconds, 3)
            observed_peak_working_set_bytes = $peak
            exit_code = $process.ExitCode
            output = $text
        }
    }
    finally { $process.Dispose() }
}

$import = Invoke-MeasuredQlog @('--home', $ledger, 'import', '--json', $sourcePath)
$replay = Invoke-MeasuredQlog @('--home', $ledger, 'import', '--json', $sourcePath)
$database = Join-Path $ledger 'qlog.db'
$databaseBefore = (Get-FileHash -LiteralPath $database -Algorithm SHA256).Hash
$report = Invoke-MeasuredQlog @('--home', $ledger, 'report', '--json')
$export = Invoke-MeasuredQlog @('--home', $ledger, 'export')
$databaseAfter = (Get-FileHash -LiteralPath $database -Algorithm SHA256).Hash
$sourceAfter = (Get-FileHash -LiteralPath $sourcePath -Algorithm SHA256).Hash
$importResult = $import.output | ConvertFrom-Json
$replayResult = $replay.output | ConvertFrom-Json
$summary = $report.output | ConvertFrom-Json
if ($sourceBefore -ne $sourceAfter) { throw 'Source changed during verification; choose a finished session.' }
if ($databaseBefore -ne $databaseAfter) { throw 'Report or export changed the ledger.' }
if ($importResult.samples -lt 1 -or $replayResult.samples -ne 0 -or $summary.usage_samples -ne $importResult.samples) {
    throw 'Native evidence import/replay invariants failed.'
}
if ($summary.total_tokens -ne $nativeTotal) { throw 'Imported total does not match the native source total.' }
$running = @(Get-Process -Name 'qlog' -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $binaryPath })
$evidence = [ordered]@{
    executed_at_utc = [DateTime]::UtcNow.ToString('o')
    platform = [System.Runtime.InteropServices.RuntimeInformation]::OSDescription
    logical_processors = [Environment]::ProcessorCount
    binary_sha256 = (Get-FileHash -LiteralPath $binaryPath -Algorithm SHA256).Hash
    binary_bytes = (Get-Item -LiteralPath $binaryPath).Length
    source_sha256 = $sourceBefore
    source_bytes = (Get-Item -LiteralPath $sourcePath).Length
    native_reported_total_tokens = $nativeTotal
    imported_total_matches_source = ($summary.total_tokens -eq $nativeTotal)
    source_unchanged = ($sourceBefore -eq $sourceAfter)
    report_export_preserved_database = ($databaseBefore -eq $databaseAfter)
    import = [ordered]@{result = $importResult; elapsed_ms = $import.elapsed_ms; observed_peak_working_set_bytes = $import.observed_peak_working_set_bytes}
    replay = [ordered]@{result = $replayResult; elapsed_ms = $replay.elapsed_ms; observed_peak_working_set_bytes = $replay.observed_peak_working_set_bytes}
    report = [ordered]@{usage_samples = $summary.usage_samples; total_tokens = $summary.total_tokens; cost_quality = $summary.cost_quality; elapsed_ms = $report.elapsed_ms; observed_peak_working_set_bytes = $report.observed_peak_working_set_bytes}
    export_elapsed_ms = $export.elapsed_ms
    remaining_binary_processes = $running.Count
    limitations = @('One source file on one Windows machine.', 'Peak working set is observed by polling; short-lived peaks may be missed.', 'Wall time includes process startup and polling overhead.', 'Usage samples are not exact model-call counts.', 'No claim of complete agent coverage, historical speedup, or production certification.')
}
$evidence | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $outputRoot 'evidence.json') -Encoding utf8NoBOM
$evidence | ConvertTo-Json -Depth 8
