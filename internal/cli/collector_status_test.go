package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectorStatusReportsPersistentLifecycleFields(t *testing.T) {
	status := CollectorStatus{Installed: true, Running: true, Reachable: true, Mode: "user_fallback", Listen: "127.0.0.1:4318", ServiceID: "dev.quantum-log.collector"}
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"installed":true`, `"running":true`, `"reachable":true`, `"mode":"user_fallback"`, `"service_id":"dev.quantum-log.collector"`} {
		if !bytes.Contains(encoded, []byte(want)) {
			t.Fatalf("status = %s", encoded)
		}
	}
}

func TestCollectorInstallIsIdempotent(t *testing.T) {
	manager := &fakeCollectorManager{}
	first, err := manager.Install(t.TempDir(), "127.0.0.1:4318")
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Install(t.TempDir(), "127.0.0.1:4318")
	if err != nil {
		t.Fatal(err)
	}
	if !first.Installed || !second.Installed || first.ServiceID != second.ServiceID {
		t.Fatalf("first=%#v second=%#v", first, second)
	}
}

func TestCollectorStatusResolvesDatabaseFromManagedHome(t *testing.T) {
	cliHome := t.TempDir()
	managedHome := t.TempDir()
	manager := &managedStatusCollectorManager{home: managedHome, listen: "127.0.0.1:4319"}

	status, err := collectorStatus(context.Background(), cliHome, defaultCollectorListen, false, false, manager)
	if err != nil {
		t.Fatalf("collectorStatus() error = %v", err)
	}
	if status.Home != managedHome || status.Database != filepath.Join(managedHome, "qlog.db") || status.Listen != manager.listen {
		t.Fatalf("status = %#v", status)
	}
}

func TestCollectorStatusExplicitHomeOverridesManagedHome(t *testing.T) {
	explicitHome := t.TempDir()
	manager := &managedStatusCollectorManager{home: t.TempDir(), listen: "127.0.0.1:4319"}

	status, err := collectorStatus(context.Background(), explicitHome, defaultCollectorListen, true, false, manager)
	if err != nil {
		t.Fatalf("collectorStatus() error = %v", err)
	}
	if status.Home != explicitHome || status.Database != filepath.Join(explicitHome, "qlog.db") || status.Listen != manager.listen {
		t.Fatalf("status = %#v", status)
	}
}

type managedStatusCollectorManager struct {
	fakeCollectorManager
	home   string
	listen string
}

func (m *managedStatusCollectorManager) ResolveManagedCollectorSettings(home, listen string, homeExplicit, listenExplicit bool) (string, string) {
	if !homeExplicit {
		home = m.home
	}
	if !listenExplicit {
		listen = m.listen
	}
	return home, listen
}

func (*fakeCollectorManager) Status(_ context.Context, listen string) (CollectorStatus, error) {
	return CollectorStatus{Listen: listen}, nil
}

func TestProbeCollectorHealthRejectsUnhealthyHTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	health := probeCollectorHealth(context.Background(), strings.TrimPrefix(server.URL, "http://"))
	if health.Reachable || health.Running {
		t.Fatalf("health = %#v, want unreachable and not running", health)
	}
}
