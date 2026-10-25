package distribution_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSimpleBuildUsesAndReleasesTemporaryDriveForBracketPaths(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows build path workaround")
	}
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "build-simple.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	forEachPowerShell(t, func(t *testing.T, shell string) {
		for _, fail := range []bool{false, true} {
			name := "success"
			if fail {
				name = "failure"
			}
			t.Run(name, func(t *testing.T) {
				root := filepath.Join(t.TempDir(), "[bracket] project")
				mustWrite(t, root, "scripts/build-simple.ps1", string(script))
				mustWrite(t, root, "go.mod", "module example.test/simple-build\n\ntoolchain go1.99.1\n")
				mustWrite(t, root, "LICENSE", "fixture license\n")
				mustWrite(t, root, "probe.ps1", simpleBuildProbe)
				command := exec.Command(shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(root, "probe.ps1"))
				command.Dir = t.TempDir()
				command.Env = append(os.Environ(), "QLOG_TEST_BUILD_ROOT="+root, "QLOG_TEST_BUILD_FAIL="+name)
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("build probe failed: %v\n%s", err, output)
				}
				data, err := os.ReadFile(filepath.Join(root, "result.json"))
				if err != nil {
					t.Fatal(err)
				}
				var result struct {
					BuildLocation, BuildToolchain, BuildError                                                          string
					Failed, LocationRestored, ToolchainRestored, DrivesRestored, OwnedDriveObserved, RetriedAllocation bool
				}
				if err := json.Unmarshal(data, &result); err != nil {
					t.Fatal(err)
				}
				if result.BuildLocation == "" || strings.ContainsAny(result.BuildLocation, "[]") || result.BuildToolchain != "go1.99.1" || result.Failed != fail || !result.LocationRestored || !result.ToolchainRestored || !result.DrivesRestored || !result.OwnedDriveObserved || !result.RetriedAllocation {
					t.Fatalf("temporary build drive contract: %+v", result)
				}
			})
		}
	})
}

const simpleBuildProbe = `
$ErrorActionPreference = 'Stop'
# A Go parent can pass PowerShell 7's module paths to Windows PowerShell 5.1.
$env:PSModulePath = Join-Path $PSHOME 'Modules'
$root = $env:QLOG_TEST_BUILD_ROOT
$initialLocation = (Get-Location).Path
$env:GOTOOLCHAIN = 'original-toolchain'
$global:buildLocation = ''
$global:buildToolchain = ''
$global:ownedDriveObserved = $false
$global:rejectedAllocation = $false
function subst.exe {
    if ($args.Count -eq 2 -and $args[1] -ne '/D' -and -not $global:rejectedAllocation) {
        $global:rejectedAllocation = $true
        $global:LASTEXITCODE = 1
        return
    }
    & (Join-Path $env:SystemRoot 'System32/subst.exe') @args
    $global:LASTEXITCODE = $LASTEXITCODE
}
function go {
    $global:buildLocation = (Get-Location).Path
    $global:buildToolchain = $env:GOTOOLCHAIN
    $global:ownedDriveObserved = @((& subst.exe) | Where-Object { $_ -match '^([A-Z]:)\\: => (.+)$' -and $Matches[2] -eq $root }).Count -eq 1
    if ($env:QLOG_TEST_BUILD_FAIL -eq 'failure') { $global:LASTEXITCODE = 1; return }
    $output = $args[[Array]::IndexOf($args, '-o') + 1]
    [IO.File]::WriteAllText($output, 'fixture executable')
    $global:LASTEXITCODE = 0
}
$failed = $false
$buildError = ''
try { & (Join-Path $root 'scripts/build-simple.ps1') }
catch { $failed = $true; $buildError = $_.ToString() + ' at ' + $_.ScriptStackTrace }
$result = @{
    BuildLocation = $global:buildLocation
    BuildToolchain = $global:buildToolchain
    BuildError = $buildError
    Failed = $failed
    LocationRestored = (Get-Location).Path -eq $initialLocation
    ToolchainRestored = $env:GOTOOLCHAIN -eq 'original-toolchain'
    DrivesRestored = @((& subst.exe) | Where-Object { $_ -match '^([A-Z]:)\\: => (.+)$' -and $Matches[2] -eq $root }).Count -eq 0
    OwnedDriveObserved = $global:ownedDriveObserved
    RetriedAllocation = $global:rejectedAllocation
}
$json = $result | ConvertTo-Json -Compress
[IO.File]::WriteAllText((Join-Path $root 'result.json'), $json)
# Remove only a leaked mapping that points at this disposable fixture.
foreach ($mapping in (& subst.exe)) {
    if ($mapping -match '^([A-Z]:)\\: => (.+)$' -and $Matches[2] -eq $root) {
        & subst.exe $Matches[1] /D
    }
}
`
