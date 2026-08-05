//go:build !windows

package cli

func recordCollectorFallbackProcess(_, _, _, _ string) error { return nil }
