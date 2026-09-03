[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][ValidateSet('codex', 'claude-code', 'opencode', 'copilot', 'copilot-vscode')][string]$AgentId,
    [Parameter(Mandatory = $true)][ValidatePattern('^[A-Za-z0-9._/+:-]+$')][string]$AgentVersion,
    [Parameter(Mandatory = $true)][ValidatePattern('^[A-Za-z0-9._/+:-]+$')][string]$CandidateTag,
    [Parameter(Mandatory = $true)][ValidatePattern('^[0-9a-fA-F]{40}$')][string]$CandidateCommit,
    [Parameter(Mandatory = $true)][string]$Output,
    [ValidateSet('PASS', 'FAIL', 'PENDING_EXTERNAL_E2E')][string]$PrivacyStatus = 'PENDING_EXTERNAL_E2E',
    [ValidateSet('PASS', 'FAIL', 'PENDING_EXTERNAL_E2E')][string]$ReplayStatus = 'PENDING_EXTERNAL_E2E',
    [string]$Qlog = 'qlog'
)

$ErrorActionPreference = 'Stop'
$startedAt = [DateTimeOffset]::UtcNow
Write-Output "UTC evidence window started: $($startedAt.ToString('o'))"
Write-Output "Perform one normal authenticated $AgentId action now. Do not paste prompts, responses, paths, commands, environment values, or agent logs here."
[void](Read-Host 'Press Enter immediately after the action completes')
$endedAt = [DateTimeOffset]::UtcNow
$evidence = [ordered]@{
    schema_version = 'qlog.acceptance.real-agent/v1'
    candidate_tag = $CandidateTag
    candidate_commit = $CandidateCommit
    platform = "windows/$([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant())"
    agent_id = $AgentId
    agent_version = $AgentVersion
    started_at = $startedAt.ToString('o')
    ended_at = $endedAt.ToString('o')
    source_evidence = $true
    ledger_status = 'PASS'
    privacy_status = $PrivacyStatus
    replay_status = $ReplayStatus
    status = 'PENDING_EXTERNAL_E2E'
} | ConvertTo-Json -Compress

& $Qlog acceptance run --output $Output --real-agent-evidence $evidence
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
Write-Output "Sanitized acceptance package: $Output"
Write-Output 'PASS is derived only when qlog finds matching ledger source evidence and every supplied gate is PASS.'
