package distribution

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Fake downloads/cosign exercise ordering and immutable inputs, not real cryptography.
func TestPowerShellInstallerSignatureGatePreservesExistingInstallation(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows PowerShell fixture")
	}
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skip("PowerShell unavailable")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"missing verifier", "rejected signature", "accepted fixture"} {
		t.Run(scenario, func(t *testing.T) {
			stage := t.TempDir()
			tools := filepath.Join(stage, "tools")
			if err := os.Mkdir(tools, 0700); err != nil {
				t.Fatal(err)
			}
			archiveName := "qlog_9.8.7_windows_" + runtime.GOARCH + ".zip"
			archiveBytes := []byte("untrusted archive must not extract")
			manifest := []byte(fmt.Sprintf("%x  %s\r\nsecond line\r\n", sha256.Sum256(archiveBytes), archiveName))
			for name, data := range map[string][]byte{"checksums.txt": manifest, "checksums.txt.sigstore.json": []byte("synthetic fixture bundle"), archiveName: archiveBytes} {
				if err := os.WriteFile(filepath.Join(stage, name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario != "missing verifier" {
				stub := "@echo off\r\necho %*>\"%QLOG_COSIGN_CALLS%\"\r\ncopy /b \"%~8\" \"%QLOG_VERIFIED_MANIFEST%\" >nul\r\ncopy /b \"%~3\" \"%QLOG_VERIFIED_BUNDLE%\" >nul\r\nexit /b %QLOG_VERIFY_EXIT%\r\n"
				if err := os.WriteFile(filepath.Join(tools, "cosign.cmd"), []byte(stub), 0600); err != nil {
					t.Fatal(err)
				}
			}
			installDir := filepath.Join(stage, "installed")
			if err := os.Mkdir(installDir, 0700); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(installDir, "qlog.exe")
			if err := os.WriteFile(target, []byte("original installed binary"), 0600); err != nil {
				t.Fatal(err)
			}
			userData := filepath.Join(stage, "user-data")
			if err := os.Mkdir(userData, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(userData, "private.db"), []byte("original private data"), 0600); err != nil {
				t.Fatal(err)
			}
			wrapper := filepath.Join(stage, "run.ps1")
			script := `$ErrorActionPreference = 'Stop'
function Get-FileHash {
 param($LiteralPath,$Algorithm)
 $sha = [Security.Cryptography.SHA256]::Create()
 try { return [pscustomobject]@{Hash=[BitConverter]::ToString($sha.ComputeHash([IO.File]::ReadAllBytes($LiteralPath))).Replace('-','')} } finally { $sha.Dispose() }
}
function Invoke-WebRequest {
 param($Uri,$OutFile,$Headers)
 $name = ($Uri -split '/')[-1]
 Add-Content -LiteralPath $env:QLOG_DOWNLOADS -Value $name
 Copy-Item -LiteralPath (Join-Path $env:QLOG_FIXTURES $name) -Destination $OutFile
}
function Expand-Archive { throw 'fixture authenticated extraction reached' }
& $env:QLOG_INSTALL_SCRIPT '--version' 'v9.8.7' '--no-bootstrap' '--no-modify-path' '--install-dir' $env:QLOG_INSTALL_DIR
`
			if err := os.WriteFile(wrapper, []byte(script), 0600); err != nil {
				t.Fatal(err)
			}
			verifyExit := "0"
			if scenario == "rejected signature" {
				verifyExit = "1"
			}
			command := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", wrapper)
			command.Env = append(os.Environ(), "PATH="+tools, "QLOG_FIXTURES="+stage, "QLOG_DOWNLOADS="+filepath.Join(stage, "downloads"), "QLOG_INSTALL_SCRIPT="+filepath.Join(root, "installers", "install.ps1"), "QLOG_INSTALL_DIR="+installDir, "QLOG_RELEASE_BASE=https://fixtures.invalid/releases", "QLOG_HOME="+userData, "QLOG_COSIGN_CALLS="+filepath.Join(stage, "verifier-call"), "QLOG_VERIFIED_MANIFEST="+filepath.Join(stage, "verified-manifest"), "QLOG_VERIFIED_BUNDLE="+filepath.Join(stage, "verified-bundle"), "QLOG_VERIFY_EXIT="+verifyExit)
			output, runErr := command.CombinedOutput()
			want := "cosign is required"
			if scenario == "rejected signature" {
				want = "signature or workflow identity is invalid"
			}
			if scenario == "accepted fixture" {
				want = "fixture authenticated extraction reached"
			}
			if runErr == nil || !strings.Contains(string(output), want) {
				t.Fatalf("gate did not enforce %q: %v %s", scenario, runErr, output)
			}
			for file, wantData := range map[string]string{target: "original installed binary", filepath.Join(userData, "private.db"): "original private data"} {
				actual, err := os.ReadFile(file)
				if err != nil || string(actual) != wantData {
					t.Fatalf("signature gate changed existing data: %s %v", filepath.Base(file), err)
				}
			}
			if scenario != "missing verifier" {
				calls, err := os.ReadFile(filepath.Join(stage, "verifier-call"))
				if err != nil {
					t.Fatal(err)
				}
				for _, arg := range []string{"verify-blob --bundle", "--certificate-oidc-issuer https://token.actions.githubusercontent.com", "--certificate-identity https://github.com/janpereira-dev/quantum_log/.github/workflows/release.yml@refs/tags/v9.8.7"} {
					if !strings.Contains(string(calls), arg) {
						t.Fatalf("verifier missing %q: %s", arg, calls)
					}
				}
				for file, wantBytes := range map[string][]byte{"verified-manifest": manifest, "verified-bundle": []byte("synthetic fixture bundle")} {
					actual, err := os.ReadFile(filepath.Join(stage, file))
					if err != nil || string(actual) != string(wantBytes) {
						t.Fatalf("cosign received modified %s: %v", file, err)
					}
				}
				if strings.Contains(string(calls), "--insecure") || strings.Contains(string(calls), "--ignore") {
					t.Fatal("verifier bypassed verification")
				}
			}
		})
	}
}
