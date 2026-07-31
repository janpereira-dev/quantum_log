package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestCollectorStatusReportsPersistentLifecycleFields(t *testing.T) {
	status := CollectorStatus{Installed: true, Running: true, Reachable: true, Listen: "127.0.0.1:4318", ServiceID: "dev.quantum-log.collector"}
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"installed":true`, `"running":true`, `"reachable":true`, `"service_id":"dev.quantum-log.collector"`} {
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

func (*fakeCollectorManager) Status(_ context.Context, listen string) (CollectorStatus, error) {
	return CollectorStatus{Listen: listen}, nil
}
