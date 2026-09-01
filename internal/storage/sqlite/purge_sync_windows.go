//go:build windows

package sqlite

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func syncLifecycleDirectory(root string) error {
	path, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return fmt.Errorf("encode qlog lifecycle control directory: %w", err)
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return fmt.Errorf("open qlog lifecycle control directory: %w", err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	if err := windows.FlushFileBuffers(handle); err != nil {
		return fmt.Errorf("sync qlog lifecycle control directory: %w", err)
	}
	return nil
}
