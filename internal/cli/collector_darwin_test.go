//go:build darwin

package cli

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestDarwinCollectorServiceDefinition(t *testing.T) {
	definition := darwinCollectorLaunchAgentDefinition("/Users/alice/bin/qlog", "/Users/alice/.qlog", "127.0.0.1:4318")
	for _, want := range []string{
		"<key>RunAtLoad</key>",
		"<key>KeepAlive</key>",
		"/Users/alice/bin/qlog",
		"collector",
		"127.0.0.1:4318",
		"/Users/alice/.qlog/collector/collector.log",
	} {
		if !strings.Contains(definition, want) {
			t.Fatalf("launch agent definition missing %q: %s", want, definition)
		}
	}
}

func TestDarwinCollectorRejectsTransientExecutable(t *testing.T) {
	for _, executable := range []string{
		"/var/folders/tmp/go-build1234/b001/exe/qlog",
		"/var/folders/tmp/cli.test",
	} {
		if err := validateCollectorExecutable(executable); err == nil {
			t.Fatalf("validateCollectorExecutable(%q) error = nil", executable)
		}
	}
}

func TestDarwinCollectorStartReplacesLoadedLaunchAgent(t *testing.T) {
	var calls [][]string
	resetDarwinCollectorStartSeams(t)
	runDarwinLaunchctl = func(args ...string) error {
		calls = append(calls, append([]string(nil), args...))
		return nil
	}

	if _, err := (darwinCollectorManager{}).Start(t.TempDir(), "127.0.0.1:4318"); err != nil {
		t.Fatal(err)
	}
	service := darwinCollectorDomain() + "/" + darwinCollectorLabel
	want := [][]string{
		{"print", service},
		{"bootout", service},
		{"bootstrap", darwinCollectorDomain(), darwinCollectorPlistPath()},
		{"kickstart", "-k", service},
	}
	if !slices.EqualFunc(calls, want, func(left, right []string) bool { return slices.Equal(left, right) }) {
		t.Fatalf("launchctl calls = %q, want %q", calls, want)
	}
}

func TestDarwinCollectorStartBootstrapsWhenJobIsNotLoaded(t *testing.T) {
	var calls [][]string
	resetDarwinCollectorStartSeams(t)
	runDarwinLaunchctl = func(args ...string) error {
		calls = append(calls, append([]string(nil), args...))
		if args[0] == "print" {
			return errors.New("not loaded")
		}
		return nil
	}

	if _, err := (darwinCollectorManager{}).Start(t.TempDir(), "127.0.0.1:4318"); err != nil {
		t.Fatal(err)
	}
	service := darwinCollectorDomain() + "/" + darwinCollectorLabel
	want := [][]string{
		{"print", service},
		{"bootstrap", darwinCollectorDomain(), darwinCollectorPlistPath()},
		{"kickstart", "-k", service},
	}
	if !slices.EqualFunc(calls, want, func(left, right []string) bool { return slices.Equal(left, right) }) {
		t.Fatalf("launchctl calls = %q, want %q", calls, want)
	}
}

func TestDarwinCollectorStartReturnsUnexpectedBootoutFailure(t *testing.T) {
	var calls [][]string
	resetDarwinCollectorStartSeams(t)
	bootoutErr := errors.New("bootout failed")
	runDarwinLaunchctl = func(args ...string) error {
		calls = append(calls, append([]string(nil), args...))
		if args[0] == "bootout" {
			return bootoutErr
		}
		return nil
	}

	if _, err := (darwinCollectorManager{}).Start(t.TempDir(), "127.0.0.1:4318"); !errors.Is(err, bootoutErr) {
		t.Fatalf("Start() error = %v, want %v", err, bootoutErr)
	}
	service := darwinCollectorDomain() + "/" + darwinCollectorLabel
	want := [][]string{{"print", service}, {"bootout", service}}
	if !slices.EqualFunc(calls, want, func(left, right []string) bool { return slices.Equal(left, right) }) {
		t.Fatalf("launchctl calls = %q, want %q", calls, want)
	}
}

func resetDarwinCollectorStartSeams(t *testing.T) {
	t.Helper()
	previousLaunchctl := runDarwinLaunchctl
	previousInstall := installDarwinCollector
	previousStatus := statusDarwinCollector
	installDarwinCollector = func(string, string) (CollectorStatus, error) { return CollectorStatus{}, nil }
	statusDarwinCollector = func(context.Context, string) (CollectorStatus, error) {
		return CollectorStatus{Message: "healthy"}, nil
	}
	t.Cleanup(func() {
		runDarwinLaunchctl = previousLaunchctl
		installDarwinCollector = previousInstall
		statusDarwinCollector = previousStatus
	})
}
