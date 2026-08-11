//go:build !windows

package adapters

type unavailableUserEnvironmentStore struct{}

func newCopilotCLIUserEnvironmentStore() copilotCLIUserEnvironmentStore {
	return unavailableUserEnvironmentStore{}
}

func (unavailableUserEnvironmentStore) Get(string) (string, bool, error) { return "", false, nil }
func (unavailableUserEnvironmentStore) Set(string, string) error         { return nil }
func (unavailableUserEnvironmentStore) Delete(string) error              { return nil }

func notifyCopilotCLIUserEnvironment() error { return nil }
