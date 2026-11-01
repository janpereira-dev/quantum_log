//go:build windows

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func setSyntheticHomeACL(t *testing.T, path string, broad bool) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sddl := "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;" + user.User.Sid.String() + ")"
	if broad {
		sddl += "(A;OICI;GR;;;WD)"
	}
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}

func hasSyntheticEveryoneGrant(t *testing.T, path string) bool {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		t.Fatalf("missing DACL %v", err)
	}
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			t.Fatal(err)
		}
		if ace.Header.AceType == windows.ACCESS_ALLOWED_ACE_TYPE && (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String() == "S-1-1-0" {
			return true
		}
	}
	return false
}

func TestEnsureProtectsNewWindowsHomeFromBroadParentACL(t *testing.T) {
	parent := t.TempDir()
	setSyntheticHomeACL(t, parent, true)
	paths, err := Resolve(filepath.Join(parent, "new-home"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Ensure(paths); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{paths.Home, paths.ConfigFile, paths.PromptHashKey} {
		if hasSyntheticEveryoneGrant(t, path) {
			t.Errorf("inherited Everyone grant retained on %s", filepath.Base(path))
		}
	}
}

func TestEnsureRejectsBroadExistingWindowsHomeWithoutChangingACLOrData(t *testing.T) {
	home := t.TempDir()
	setSyntheticHomeACL(t, home, true)
	original := []byte("synthetic original data")
	existing := filepath.Join(home, "qlog.db")
	if err := os.WriteFile(existing, original, 0600); err != nil {
		t.Fatal(err)
	}
	paths, err := Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	err = Ensure(paths)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "permission") {
		t.Fatalf("want explicit permission error, got %v", err)
	}
	if !hasSyntheticEveryoneGrant(t, home) {
		t.Fatal("existing ACL was silently changed")
	}
	data, err := os.ReadFile(existing)
	if err != nil || string(data) != string(original) {
		t.Fatalf("existing data changed: %q %v", data, err)
	}
	for _, path := range []string{paths.ConfigFile, paths.PromptHashKey} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("private file created despite unsafe existing home: %s %v", filepath.Base(path), err)
		}
	}
}

func TestEnsureRejectsBroadExistingSecretWithinPrivateWindowsHome(t *testing.T) {
	home := t.TempDir()
	setSyntheticHomeACL(t, home, false)
	paths, err := Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	original := []byte("synthetic-key-that-must-be-retained-unchanged")
	if err := os.WriteFile(paths.PromptHashKey, original, 0600); err != nil {
		t.Fatal(err)
	}
	setSyntheticHomeACL(t, paths.PromptHashKey, true)
	if err := Ensure(paths); err == nil {
		t.Fatal("broad existing secret accepted")
	}
	data, err := os.ReadFile(paths.PromptHashKey)
	if err != nil || string(data) != string(original) {
		t.Fatalf("existing key changed: %q %v", data, err)
	}
}

func TestEnsureRejectsBroadInheritOnlyWindowsGrantBeforeCreatingSecrets(t *testing.T) {
	home := t.TempDir()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;" + user.User.Sid.String() + ")(A;OIIO;GR;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(home, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	paths, err := Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := Ensure(paths); err == nil {
		t.Fatal("inherit-only Everyone rule would leak future secrets")
	}
	if _, err := os.Stat(paths.PromptHashKey); !os.IsNotExist(err) {
		t.Fatalf("secret created despite broad future-file grant: %v", err)
	}
}

func TestTLSRejectsBroadNestedIdentityWithoutChangingFilesOrACL(t *testing.T) {
	for _, relative := range []string{"collector-tls", filepath.Join("collector-tls", "cert.pem"), filepath.Join("collector-tls", "key.pem")} {
		t.Run(relative, func(t *testing.T) {
			paths, _ := Resolve(t.TempDir())
			if err := EnsureCollectorTLS(paths, "127.0.0.1:4318"); err != nil {
				t.Fatal(err)
			}
			certBefore, _ := os.ReadFile(CollectorCertificatePath(paths))
			keyBefore, _ := os.ReadFile(collectorPrivateKeyPath(paths))
			target := filepath.Join(paths.Home, relative)
			setSyntheticHomeACL(t, target, true)
			if err := EnsureCollectorTLS(paths, "127.0.0.1:4318"); err == nil {
				t.Fatal("broad TLS identity accepted")
			}
			if _, err := CollectorClientTLS(paths); err == nil {
				t.Fatal("client trusted broadly writable identity")
			}
			if !hasSyntheticEveryoneGrant(t, target) {
				t.Fatal("existing ACL was silently changed")
			}
			certAfter, _ := os.ReadFile(CollectorCertificatePath(paths))
			keyAfter, _ := os.ReadFile(collectorPrivateKeyPath(paths))
			if string(certBefore) != string(certAfter) || string(keyBefore) != string(keyAfter) {
				t.Fatal("existing TLS identity changed")
			}
		})
	}
}
