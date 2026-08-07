[CmdletBinding()]
param(
    [string]$Qlog = "qlog",
    [string]$Output = (Join-Path (Get-Location) "qlog-external-acceptance.zip")
)

$ErrorActionPreference = 'Stop'
& $Qlog acceptance run --output $Output
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
Write-Output "Local acceptance package: $Output"
Write-Output "This package does not claim external verification. Review PENDING_EXTERNAL_E2E before release."
