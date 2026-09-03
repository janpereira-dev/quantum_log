package distribution_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCleanGeneratedRemovesOnlyAllowlistedOutputsAndPreservesLedger(t *testing.T) {
	forEachPowerShell(t, func(t *testing.T, shell string) {
		root := newRepositoryFixture(t)
		ledgerBefore := snapshotLedger(t, root)

		mustWrite(t, root, "dist/archive.zip", "generated archive")
		mustWrite(t, root, "coverage.out", "generated coverage")
		mustWrite(t, root, "qlog-external-acceptance.zip", "generated acceptance package")
		mustWrite(t, root, "quantum-log.test", "generated test binary")
		mustWrite(t, root, "nested/must-remain.test", "not a root output")
		mustWrite(t, root, "release.zip", "not allowlisted")

		output, err := runCleaner(shell, root, false)
		if err != nil {
			t.Fatalf("cleanup failed: %v\n%s", err, output)
		}

		for _, name := range []string{"dist", "coverage.out", "qlog-external-acceptance.zip", "quantum-log.test"} {
			assertMissing(t, root, name)
		}
		for _, name := range []string{"nested/must-remain.test", "release.zip"} {
			assertFileContent(t, root, name, map[string]string{
				"nested/must-remain.test": "not a root output",
				"release.zip":             "not allowlisted",
			}[name])
		}
		assertLedgerUnchanged(t, root, ledgerBefore)
		if !strings.Contains(output, "preserved:") || !strings.Contains(output, filepath.Join(root, ".qlog")) {
			t.Fatalf("cleanup did not report ledger preservation:\n%s", output)
		}
	})
}

func TestCleanGeneratedDryRunLeavesEveryFileUnchanged(t *testing.T) {
	forEachPowerShell(t, func(t *testing.T, shell string) {
		root := newRepositoryFixture(t)
		files := map[string]string{
			"dist/archive.zip":             "generated archive",
			"coverage.out":                 "generated coverage",
			"qlog-external-acceptance.zip": "generated acceptance package",
			"quantum-log.test":             "generated test binary",
		}
		for name, value := range files {
			mustWrite(t, root, name, value)
		}
		ledgerBefore := snapshotLedger(t, root)

		output, err := runCleaner(shell, root, true)
		if err != nil {
			t.Fatalf("dry run failed: %v\n%s", err, output)
		}

		for name, value := range files {
			assertFileContent(t, root, name, value)
		}
		assertLedgerUnchanged(t, root, ledgerBefore)
	})
}

func TestCleanGeneratedRejectsInvalidRepositoryRoot(t *testing.T) {
	forEachPowerShell(t, func(t *testing.T, shell string) {
		root := t.TempDir()
		mustWrite(t, root, "coverage.out", "must remain")

		output, err := runCleaner(shell, root, false)
		if err == nil {
			t.Fatalf("cleanup accepted a directory without Quantum Log repository markers:\n%s", output)
		}
		assertFileContent(t, root, "coverage.out", "must remain")
		if !strings.Contains(output, "RepositoryRoot is not a Quantum Log checkout") {
			t.Fatalf("unexpected invalid-root error:\n%s", output)
		}
	})
}

func TestCleanGeneratedRejectsUnrelatedGoRepository(t *testing.T) {
	forEachPowerShell(t, func(t *testing.T, shell string) {
		root := newRepositoryFixture(t)
		mustWrite(t, root, "go.mod", "module example.com/not-quantum-log\n")
		mustWrite(t, root, "dist/archive.zip", "must remain")
		mustWrite(t, root, "coverage.out", "must remain")
		ledgerBefore := snapshotLedger(t, root)

		output, err := runCleaner(shell, root, false)
		if err == nil {
			t.Fatalf("cleanup accepted an unrelated Go repository:\n%s", output)
		}
		assertFileContent(t, root, "dist/archive.zip", "must remain")
		assertFileContent(t, root, "coverage.out", "must remain")
		assertLedgerUnchanged(t, root, ledgerBefore)
		if !strings.Contains(output, "RepositoryRoot is not a Quantum Log checkout") {
			t.Fatalf("unexpected unrelated-repository error:\n%s", output)
		}
	})
}

func TestCleanGeneratedRejectsMissingQuantumLogProjectMarker(t *testing.T) {
	forEachPowerShell(t, func(t *testing.T, shell string) {
		root := newRepositoryFixture(t)
		marker := filepath.Join(root, "QUANTUM_LOG_MASTER_PROMPT.md")
		if err := os.Remove(marker); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, root, "coverage.out", "must remain")

		output, err := runCleaner(shell, root, false)
		if err == nil {
			t.Fatalf("cleanup accepted a repository without the Quantum Log project marker:\n%s", output)
		}
		assertFileContent(t, root, "coverage.out", "must remain")
		if !strings.Contains(output, "RepositoryRoot is not a Quantum Log checkout") {
			t.Fatalf("unexpected missing-marker error:\n%s", output)
		}
	})
}

func forEachPowerShell(t *testing.T, test func(t *testing.T, shell string)) {
	t.Helper()
	available := 0
	for _, name := range []string{"powershell.exe", "pwsh.exe"} {
		shell, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		available++
		t.Run(strings.TrimSuffix(name, ".exe"), func(t *testing.T) {
			test(t, shell)
		})
	}
	if available == 0 {
		t.Skip("PowerShell is unavailable")
	}
}

func runCleaner(shell, root string, dryRun bool) (string, error) {
	_, filename, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(filename), "..", "..", "scripts", "clean-generated.ps1")
	args := []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script, "-RepositoryRoot", root}
	if dryRun {
		args = append(args, "-DryRun")
	}
	cmd := exec.Command(shell, args...)
	cmd.Env = append(os.Environ(), "POWERSHELL_TELEMETRY_OPTOUT=1")
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func newRepositoryFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, root, "go.mod", "module github.com/janpereira-dev/quantum_log\n")
	mustWrite(t, root, ".gitignore", "coverage.out\n")
	mustWrite(t, root, "QUANTUM_LOG_MASTER_PROMPT.md", "# Quantum Log\n")
	mustWrite(t, root, ".qlog/qlog.db", "ledger-sentinel\x00\x01")
	mustWrite(t, root, ".qlog/qlog.db-wal", "wal-sentinel\x02\x03")
	mustWrite(t, root, ".qlog/qlog.db-shm", "shm-sentinel\x04\x05")
	mustWrite(t, root, ".qlog/operator-note.txt", "operator-owned")
	return root
}

func snapshotLedger(t *testing.T, root string) map[string][]byte {
	t.Helper()
	want := make(map[string][]byte)
	err := filepath.Walk(filepath.Join(root, ".qlog"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		want[relative] = contents
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return want
}

func assertLedgerUnchanged(t *testing.T, root string, before map[string][]byte) {
	t.Helper()
	after := snapshotLedger(t, root)
	if len(after) != len(before) {
		t.Fatalf("ledger file count changed: before=%d after=%d", len(before), len(after))
	}
	for name, want := range before {
		got, ok := after[name]
		if !ok {
			t.Errorf("ledger file removed: %s", name)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("ledger file changed byte-for-byte: %s", name)
		}
	}
}

func mustWrite(t *testing.T, root, name, value string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertMissing(t *testing.T, root, name string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("generated path remains: %s (%v)", path, err)
	}
}

func assertFileContent(t *testing.T, root, name, want string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s changed: got %q, want %q", name, got, want)
	}
}
