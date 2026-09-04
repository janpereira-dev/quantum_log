[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][ValidateSet('codex', 'claude-code', 'opencode', 'copilot', 'copilot-vscode')][string]$AgentId,
    [Parameter(Mandatory = $true)][ValidatePattern('^[0-9A-Za-z][0-9A-Za-z._+-]{0,63}$')][string]$AgentVersion,
    [Parameter(Mandatory = $true)][string]$Output
)

$ErrorActionPreference = 'Stop'
$command = Get-Command qlog -CommandType Application -ErrorAction Stop
$qlogPath = $command.Path
$qlogItem = Get-Item -LiteralPath $qlogPath -Force
if (-not $qlogItem.PSIsContainer -and ($qlogItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -eq 0) {
    $qlogHash = (Get-FileHash -LiteralPath $qlogPath -Algorithm SHA256).Hash.ToLowerInvariant()
} else {
    throw 'qlog must resolve to a regular non-reparse executable file'
}

$boundaryOutput = @(& $qlogPath acceptance begin --agent $AgentId --agent-version $AgentVersion)
$beginCode = $LASTEXITCODE
if ($beginCode -ne 0) {
    exit $beginCode
}
if ($boundaryOutput.Count -ne 1 -or $boundaryOutput[0] -notmatch '^[0-9a-f]{64}$') {
    throw 'qlog returned an invalid acceptance boundary'
}
$boundaryId = $boundaryOutput[0]

Write-Output "Perform one normal authenticated $AgentId action now. Do not paste prompts, responses, paths, commands, environment values, or agent logs here."
[void](Read-Host 'Press Enter immediately after the action completes')
if ((Get-FileHash -LiteralPath $qlogPath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $qlogHash) {
    throw 'qlog changed after the boundary was created'
}

& $qlogPath acceptance run --output $Output --boundary $boundaryId
$runCode = $LASTEXITCODE
if ($runCode -ne 0) {
    exit $runCode
}
$package = Get-Item -LiteralPath $Output -Force -ErrorAction Stop
if ($package.PSIsContainer -or $package.Length -eq 0 -or ($package.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
    throw 'acceptance package is missing, empty, or unsafe'
}
if ((Get-FileHash -LiteralPath $qlogPath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $qlogHash) {
    throw 'qlog changed while packaging evidence'
}
& $qlogPath acceptance inspect --package $Output
$inspectCode = $LASTEXITCODE
if ($inspectCode -ne 0) {
    exit $inspectCode
}
Write-Output "Sanitized acceptance package structurally validated; external authenticity remains pending: $Output"
