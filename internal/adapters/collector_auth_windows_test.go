//go:build windows

package adapters

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestAuthenticatedAdaptersRefuseSharedWindowsSettingsBeforeWriting(t *testing.T) {
	for _, id := range []string{"codex", "claude-code", "copilot-vscode"} {
		t.Run(id, func(t *testing.T) {
			configRoot := t.TempDir()
			t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configRoot)
			var path string
			original := []byte(`{"user_setting":"preserve"}`)
			switch id {
			case "codex":
				path = newCodexAdapter().configPath()
				original = []byte("model = \"preserve\"\n")
			case "claude-code":
				path = newClaudeCodeAdapter().settingsPath()
			case "copilot-vscode":
				path = newVSCodeCopilotAdapter().settingsPath()
			}
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			user, err := windows.GetCurrentProcessToken().GetTokenUser()
			if err != nil {
				t.Fatal(err)
			}
			sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;" + user.User.Sid.String() + ")(A;OICI;GR;;;WD)")
			if err != nil {
				t.Fatal(err)
			}
			acl, _, err := sd.DACL()
			if err != nil {
				t.Fatal(err)
			}
			if err := windows.SetNamedSecurityInfo(filepath.Dir(path), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
				t.Fatal(err)
			}
			adapter, _ := Default().Get(id)
			_, err = adapter.Install(context.Background(), InstallOptions{Home: t.TempDir()})
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "permission") {
				t.Fatalf("expected actionable permission refusal, got %v", err)
			}
			if data := mustReadFile(t, path); string(data) != string(original) {
				t.Fatalf("original settings changed: %s", data)
			}
			if err := filepath.WalkDir(configRoot, func(found string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !entry.IsDir() && found != path {
					t.Errorf("failed setup left state or credential file: %s", filepath.Base(found))
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
