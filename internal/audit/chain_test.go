package audit

import "testing"

func TestVerifyDetectsTamperedEvent(t *testing.T) {
	t.Parallel()

	records := []Record{
		NewRecord("session-1", "first", ""),
	}
	records = append(records, NewRecord("session-1", "second", records[0].Hash))

	if err := Verify(records); err != nil {
		t.Fatalf("Verify() unexpected error: %v", err)
	}

	records[1].Payload = "changed"
	if err := Verify(records); err == nil {
		t.Fatal("Verify() did not detect tampering")
	}
}
