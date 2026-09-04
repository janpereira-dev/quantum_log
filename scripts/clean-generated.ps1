[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'Medium')]
param(
    [Parameter()]
    [string]$RepositoryRoot = (Join-Path $PSScriptRoot '..'),

    [Parameter()]
    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'

$root = [System.IO.Path]::GetFullPath($RepositoryRoot)
$goModPath = Join-Path $root 'go.mod'
$gitIgnorePath = Join-Path $root '.gitignore'
$projectMarkerPath = Join-Path $root 'QUANTUM_LOG_MASTER_PROMPT.md'
if (-not (Test-Path -LiteralPath $goModPath -PathType Leaf) -or
    -not (Test-Path -LiteralPath $gitIgnorePath -PathType Leaf) -or
    -not (Test-Path -LiteralPath $projectMarkerPath -PathType Leaf)) {
    throw "RepositoryRoot is not a Quantum Log checkout: $root"
}

$modulePath = $null
foreach ($line in @(Get-Content -LiteralPath $goModPath)) {
    $moduleMatch = [System.Text.RegularExpressions.Regex]::Match(
        $line,
        '^\s*module[ \t]+([^\s]+)[ \t]*(?://.*)?$'
    )
    if ($moduleMatch.Success) {
        $modulePath = $moduleMatch.Groups[1].Value
        break
    }
}
if ($modulePath -cne 'github.com/janpereira-dev/quantum_log') {
    throw "RepositoryRoot is not a Quantum Log checkout: $root"
}

$ledgerRoot = [System.IO.Path]::GetFullPath((Join-Path $root '.qlog'))
$rootPrefix = $root.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
$ledgerPrefix = $ledgerRoot.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar

function Assert-SafeCleanupTarget {
    param([Parameter(Mandatory = $true)][string]$Path)

    $resolvedTarget = [System.IO.Path]::GetFullPath($Path)
    if (-not $resolvedTarget.StartsWith($rootPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to clean path outside repository: $resolvedTarget"
    }
    if ($resolvedTarget.Equals($ledgerRoot, [System.StringComparison]::OrdinalIgnoreCase) -or
        $resolvedTarget.StartsWith($ledgerPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to clean ledger path: $resolvedTarget"
    }
    return $resolvedTarget
}

function Remove-GeneratedTarget {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter()][switch]$Recurse
    )

    $target = Assert-SafeCleanupTarget -Path $Path
    if (-not (Test-Path -LiteralPath $target)) {
        return
    }

    $item = Get-Item -LiteralPath $target -Force
    if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Refusing to clean reparse point: $target"
    }

    if ($PSCmdlet.ShouldProcess($target, 'Remove generated output')) {
        if ($Recurse) {
            Remove-Item -LiteralPath $target -Recurse -Force -WhatIf:$DryRun
        }
        else {
            Remove-Item -LiteralPath $target -Force -WhatIf:$DryRun
        }
    }
}

foreach ($relative in @('dist', 'coverage.out', 'qlog-external-acceptance.zip')) {
    Remove-GeneratedTarget -Path (Join-Path $root $relative) -Recurse:($relative -eq 'dist')
}

foreach ($testBinary in @(Get-ChildItem -LiteralPath $root -File -Filter '*.test')) {
    Remove-GeneratedTarget -Path $testBinary.FullName
}

Write-Output "preserved: $ledgerRoot"
