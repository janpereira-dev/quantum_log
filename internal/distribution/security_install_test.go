package distribution

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func posixPath(path string) string {
	path = filepath.ToSlash(path)
	if runtime.GOOS == "windows" && len(path) > 2 && path[1] == ':' {
		return "/" + strings.ToLower(path[:1]) + path[2:]
	}
	return path
}

func TestShellInstallerAuthenticatesBeforeInstallAndQuotesProfile(t *testing.T) {
	shell := "sh"
	if runtime.GOOS == "windows" {
		shell = filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe")
		if _, err := os.Stat(shell); err != nil {
			t.Skip("Git Bash unavailable")
		}
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, reject := range []bool{true, false} {
		t.Run(fmt.Sprintf("signature_rejected=%t", reject), func(t *testing.T) {
			stage := t.TempDir()
			archive := filepath.Join(stage, "qlog_9.8.7_linux_amd64.tar.gz")
			file, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			gzipWriter := gzip.NewWriter(file)
			tarWriter := tar.NewWriter(gzipWriter)
			binary := []byte("#!/bin/sh\nexit 0\n")
			if err := tarWriter.WriteHeader(&tar.Header{Name: "qlog", Mode: 0o755, Size: int64(len(binary))}); err != nil {
				t.Fatal(err)
			}
			_, _ = tarWriter.Write(binary)
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			_ = file.Close()
			contents, _ := os.ReadFile(archive)
			_ = os.WriteFile(filepath.Join(stage, "checksums.txt"), []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(contents), filepath.Base(archive))), 0o600)
			_ = os.WriteFile(filepath.Join(stage, "checksums.txt.sigstore.json"), []byte("fixture verifier input"), 0o600)
			bin := filepath.Join(stage, "tools")
			_ = os.Mkdir(bin, 0o700)
			stubs := map[string]string{
				"uname":  "case \"$1\" in -s) echo Linux;; -m) echo x86_64;; esac\n",
				"curl":   "url=''; out=''; while [ \"$#\" -gt 0 ]; do case \"$1\" in https:*) url=$1;; --output) shift; out=$1;; esac; shift; done\ncp \"$QLOG_FIXTURES/${url##*/}\" \"$out\"\n",
				"cosign": "printf '%s\\n' \"$*\" > \"$QLOG_FIXTURES/verifier-call\"\nexit \"$QLOG_VERIFY_EXIT\"\n",
			}
			for name, code := range stubs {
				if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nset -eu\n"+code), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			install := filepath.Join(stage, "literal$(touch PROFILE_EXECUTED)`touch OTHER_MARKER`'dir")
			_ = os.Mkdir(install, 0o700)
			original := filepath.Join(install, "qlog")
			_ = os.WriteFile(original, []byte("prior installation"), 0o700)
			profile := filepath.Join(stage, "profile")
			command := exec.Command(shell, filepath.Join(root, "installers", "install.sh"), "--version", "v9.8.7", "--no-bootstrap")
			if runtime.GOOS == "windows" {
				command = exec.Command(shell, "--noprofile", "--norc", "-c", `export PATH="$QLOG_TOOLS:/usr/bin:/bin"; exec sh "$QLOG_INSTALL_SCRIPT" --version v9.8.7 --no-bootstrap`)
			}
			verifyExit := "0"
			if reject {
				verifyExit = "1"
			}
			command.Dir = stage
			command.Env = append(os.Environ(), "QLOG_TOOLS="+posixPath(bin), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "QLOG_FIXTURES="+posixPath(stage), "QLOG_INSTALL_SCRIPT="+posixPath(filepath.Join(root, "installers", "install.sh")), "QLOG_INSTALL_DIR="+posixPath(install), "QLOG_PROFILE="+posixPath(profile), "QLOG_VERIFY_EXIT="+verifyExit, "QLOG_RELEASE_BASE=https://example.invalid/fixtures")
			output, installErr := command.CombinedOutput()
			if reject {
				prior, _ := os.ReadFile(original)
				if installErr == nil || string(prior) != "prior installation" {
					t.Fatalf("invalid signature changed installed binary: err=%v contents=%q output=%s", installErr, prior, output)
				}
				return
			}
			if installErr != nil {
				t.Fatalf("fixture gate installation failed: %v %s", installErr, output)
			}
			call, err := os.ReadFile(filepath.Join(stage, "verifier-call"))
			if err != nil || !strings.Contains(string(call), "--certificate-identity https://github.com/janpereira-dev/quantum_log/.github/workflows/release.yml@refs/tags/v9.8.7") || !strings.Contains(string(call), "--certificate-oidc-issuer https://token.actions.githubusercontent.com") {
				t.Fatalf("workflow identity was not verified: %s %v", call, err)
			}
			load := exec.Command(shell, "-c", `. "$QLOG_PROFILE"; [ "${PATH%%:*}" = "$QLOG_INSTALL_DIR" ]`)
			load.Dir, load.Env = stage, command.Env
			if output, err := load.CombinedOutput(); err != nil {
				t.Fatalf("generated profile did not retain literal directory: %v %s", err, output)
			}
			for _, marker := range []string{"PROFILE_EXECUTED", "OTHER_MARKER"} {
				if _, err := os.Stat(filepath.Join(stage, marker)); !os.IsNotExist(err) {
					t.Fatalf("directory name was executed: %s", marker)
				}
			}
		})
	}
}
