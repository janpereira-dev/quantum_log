//go:build windows

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func ensurePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		user, err := windows.GetCurrentProcessToken().GetTokenUser()
		if err != nil {
			return err
		}
		sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;" + user.User.Sid.String() + ")")
		if err != nil {
			return err
		}
		native, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		attributes := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
		if err := windows.CreateDirectory(native, &attributes); err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			return err
		}
		info, err = os.Lstat(path)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refuse non-directory or linked path %q", path)
	}
	return checkPrivatePermissions(path, info)
}

func checkPrivatePermissions(path string, _ os.FileInfo) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	allowed := func(sid *windows.SID) bool {
		if sid == nil {
			return false
		}
		text := sid.String()
		return text == user.User.Sid.String() || text == "S-1-5-18" || text == "S-1-5-32-544"
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	if sd == nil {
		return fmt.Errorf("missing security descriptor on %q", path)
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	if !allowed(owner) {
		return fmt.Errorf("untrusted owner on %q; protect the path explicitly before continuing", path)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if acl == nil {
		return fmt.Errorf("unrestricted access on %q; protect the path explicitly before continuing", path)
	}
	for index := uint32(0); index < uint32(acl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, index, &ace); err != nil {
			return err
		}
		if ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return fmt.Errorf("unsupported access rule on %q; review permissions explicitly before continuing", path)
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 && sid.String() == "S-1-3-0" {
			continue
		}
		if ace.Mask != 0 && !allowed(sid) {
			return fmt.Errorf("access granted to another identity on %q; restrict permissions explicitly before continuing", path)
		}
	}
	return nil
}
