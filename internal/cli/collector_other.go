//go:build !windows

package cli

import "errors"

type unsupportedCollectorManager struct{}

func newCollectorManager() collectorManager { return unsupportedCollectorManager{} }

func (unsupportedCollectorManager) Install(_, _ string) (string, error) {
	return "", errManagedCollectorUnsupported
}

func (unsupportedCollectorManager) Start(_, _ string) (string, error) {
	return "", errManagedCollectorUnsupported
}

func (unsupportedCollectorManager) Stop() (string, error) {
	return "", errManagedCollectorUnsupported
}

func (unsupportedCollectorManager) Restart(_, _ string) (string, error) {
	return "", errManagedCollectorUnsupported
}

func (unsupportedCollectorManager) Logs() (string, error) {
	return "", errManagedCollectorUnsupported
}

func (unsupportedCollectorManager) Uninstall() (string, error) {
	return "", errManagedCollectorUnsupported
}

var errManagedCollectorUnsupported = errors.New("managed collector lifecycle is currently implemented for Windows only; use qlog collector serve")
