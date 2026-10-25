[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$projectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$sourceRoot = $projectRoot
$outputRoot = Join-Path $projectRoot 'dist/simple'
New-Item -ItemType Directory -Path $outputRoot -Force | Out-Null
$toolchainLine = Get-Content -LiteralPath (Join-Path $projectRoot 'go.mod') |
    Where-Object { $_ -match '^toolchain\s+(go[0-9]+\.[0-9]+\.[0-9]+)$' } |
    Select-Object -First 1
if (-not $toolchainLine) { throw 'The repository must specify a Go toolchain.' }
$pinnedToolchain = ($toolchainLine -split '\s+')[1]
$previousToolchain = $env:GOTOOLCHAIN
$temporaryDrive = $null
Push-Location -LiteralPath $projectRoot
try {
    if ($env:OS -eq 'Windows_NT' -and $projectRoot -match '\[|\]') {
        # Go embed and Compress-Archive cannot resolve bracketed checkout paths.
        foreach ($letter in 90..68) {
            $drive = '{0}:' -f [char]$letter
            if ("$drive\" -in [System.IO.Directory]::GetLogicalDrives() -or (Get-PSDrive -Name ([char]$letter) -ErrorAction SilentlyContinue)) { continue }
            & subst.exe $drive $projectRoot *> $null
            if ($LASTEXITCODE -eq 0) { $temporaryDrive = $drive; break }
        }
        if (-not $temporaryDrive) { throw 'Could not allocate a temporary build drive.' }
        $projectRoot = "$temporaryDrive\"
        $outputRoot = Join-Path $projectRoot 'dist/simple'
        Set-Location -LiteralPath $projectRoot
    }
    $env:GOTOOLCHAIN = $pinnedToolchain
    & go build -trimpath -ldflags '-s -w' -o (Join-Path $outputRoot 'qlog.exe') ./cmd/qlog
    if ($LASTEXITCODE -ne 0) { throw 'Simple executable build failed.' }

    @'
QUANTUM LOG — SIMPLE LOCAL VERSION

Open PowerShell in this folder and run:

  .\qlog.exe import
  .\qlog.exe report
  .\qlog.exe export > usage.csv

Import reads available local Codex logs on demand (newest 100 files modified
within 7 days). To read an older session:

  .\qlog.exe import C:\path\to\session.jsonl

The executable exits after each command. It installs no service, startup entry,
hook, watcher, or agent configuration. Reports use a separate local ledger in
%LOCALAPPDATA%\QUANTUM_LOG\simple unless QLOG_HOME or --home overrides it.

Only usage and selected metadata are stored. Prompts, responses, and tool content
are excluded. Tokens are agent-reported; usage samples are not exact model-call
counts. Costs and subscription charges are unavailable. Reimporting unchanged
evidence does not add its tokens again.

This is a local source build, not a published release or a replacement for any
older installed collector. The older project and its data remain preserved.
Use .\qlog.exe --help for filters and limits.
'@ | Set-Content -LiteralPath (Join-Path $outputRoot 'README.txt') -Encoding utf8
    Copy-Item -LiteralPath (Join-Path $projectRoot 'LICENSE') -Destination $outputRoot
    $package = Join-Path $projectRoot 'dist/quantum-log-simple-windows.zip'
    Compress-Archive -LiteralPath @((Join-Path $outputRoot 'qlog.exe'), (Join-Path $outputRoot 'README.txt'), (Join-Path $outputRoot 'LICENSE')) -DestinationPath $package -Force
    $hash = (Get-FileHash -LiteralPath $package -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  quantum-log-simple-windows.zip" | Set-Content -LiteralPath (Join-Path $projectRoot 'dist/simple-checksums.txt') -Encoding ascii
}
finally {
    $env:GOTOOLCHAIN = $previousToolchain
    Pop-Location
    if ($temporaryDrive) {
        & subst.exe $temporaryDrive /D
        if ($LASTEXITCODE -ne 0) { throw 'Could not remove the temporary build drive.' }
    }
}
Write-Output "Executable: $(Join-Path $sourceRoot 'dist/simple/qlog.exe')"
Write-Output "Portable package: $(Join-Path $sourceRoot 'dist/quantum-log-simple-windows.zip')"
