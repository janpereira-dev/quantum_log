package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/config"
	storelock "github.com/janpereira-dev/quantum_log/internal/storage/lock"
	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

func TestInitializeRejectsPurgeSentinelBeforeConfigurationCreation(t *testing.T) {
	home := filepath.Join(t.TempDir(), "ledger")
	paths, err := config.Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Database+".purge.pending", nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Initialize(context.Background(), home); err == nil || !strings.Contains(err.Error(), "purge is pending") {
		t.Fatalf("Initialize() error = %v", err)
	}
	if _, err := os.Stat(paths.ConfigFile); !os.IsNotExist(err) {
		t.Fatalf("Initialize created configuration despite the purge sentinel: %v", err)
	}
}

func TestInitializeRetainsQuiescenceLockThroughConfigurationAndStoreHandoff(t *testing.T) {
	home := filepath.Join(t.TempDir(), "ledger")
	paths, err := config.Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.Home, 0o700); err != nil {
		t.Fatal(err)
	}

	originalEnsure, originalOpen := ensureInitializedConfig, openInitializedStore
	t.Cleanup(func() {
		ensureInitializedConfig = originalEnsure
		openInitializedStore = originalOpen
	})
	ensureInitializedConfig = func(paths config.Paths) error {
		assertInitializationLockHeld(t, paths.Database)
		return config.Ensure(paths)
	}
	openInitializedStore = func(ctx context.Context, database string) (*storepkg.Store, error) {
		assertInitializationLockHeld(t, database)
		return storepkg.Open(ctx, database)
	}

	service, err := Initialize(context.Background(), home)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	assertInitializationLockHeld(t, paths.Database)
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	assertInitializationLockReleased(t, paths.Database)
}

func TestInitializeReleasesQuiescenceLockAfterStoreHandoffFailure(t *testing.T) {
	home := filepath.Join(t.TempDir(), "ledger")
	paths, err := config.Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.Home, 0o700); err != nil {
		t.Fatal(err)
	}

	originalEnsure, originalOpen := ensureInitializedConfig, openInitializedStore
	t.Cleanup(func() {
		ensureInitializedConfig = originalEnsure
		openInitializedStore = originalOpen
	})
	ensureInitializedConfig = config.Ensure
	openInitializedStore = func(_ context.Context, database string) (*storepkg.Store, error) {
		assertInitializationLockHeld(t, database)
		return nil, errors.New("simulated storage handoff failure")
	}

	if _, err := Initialize(context.Background(), home); err == nil || !strings.Contains(err.Error(), "simulated storage handoff failure") {
		t.Fatalf("Initialize() error = %v", err)
	}
	assertInitializationLockReleased(t, paths.Database)
}

func TestInitializeClosesStoreWhenInitializationGuardReleaseFails(t *testing.T) {
	home := filepath.Join(t.TempDir(), "ledger")
	paths, err := config.Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.Home, 0o700); err != nil {
		t.Fatal(err)
	}

	originalEnsure, originalOpen, originalGuard := ensureInitializedConfig, openInitializedStore, acquireInitializationGuard
	t.Cleanup(func() {
		ensureInitializedConfig = originalEnsure
		openInitializedStore = originalOpen
		acquireInitializationGuard = originalGuard
	})
	ensureInitializedConfig = config.Ensure
	openInitializedStore = storepkg.Open
	acquireInitializationGuard = func(string) (initializationGuard, error) {
		return failingInitializationGuard{}, nil
	}

	service, err := Initialize(context.Background(), home)
	if service != nil {
		t.Fatalf("Initialize() returned a service after guard release failed: %#v", service)
	}
	if err == nil || !strings.Contains(err.Error(), "release initialization quiescence lock") {
		t.Fatalf("Initialize() error = %v", err)
	}
	// A second writer can open only when Initialize closed the Store it had
	// already created before the guard release failure was returned.
	store, err := storepkg.Open(context.Background(), paths.Database)
	if err != nil {
		t.Fatalf("Initialize leaked its Store after guard release failure: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

type failingInitializationGuard struct{}

func (failingInitializationGuard) Close() error {
	return errors.New("simulated initialization guard release failure")
}

func assertInitializationLockHeld(t *testing.T, database string) {
	t.Helper()
	lock, err := storelock.AcquireExclusiveExisting(database + ".quiescence.lock")
	if err == nil {
		_ = lock.Close()
		t.Fatal("initialization released the quiescence lock before the store handoff")
	}
}

func assertInitializationLockReleased(t *testing.T, database string) {
	t.Helper()
	lock, err := storelock.AcquireExclusiveExisting(database + ".quiescence.lock")
	if err != nil {
		t.Fatalf("initialization retained the quiescence lock after failure/close: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
}
