package cli

import "testing"

func TestCopilotCLIHookEventAcceptsConfiguredEvents(t *testing.T) {
	t.Parallel()

	input := []byte(`{"sessionId":"session-1","cwd":"/repo","eventId":"event-1","prompt":"hello"}`)
	for _, eventType := range []string{
		"sessionStart",
		"sessionEnd",
		"userPromptSubmitted",
		"agentStop",
		"errorOccurred",
		"preToolUse",
		"postToolUse",
		"subagentStart",
		"subagentStop",
	} {
		event, err := copilotCLIHookEvent(input, eventType)
		if err != nil {
			t.Fatalf("event %q: %v", eventType, err)
		}
		if event.SessionID != "session-1" || event.UpstreamEventID != "event-1" {
			t.Fatalf("event %q identity = session %q, upstream %q", eventType, event.SessionID, event.UpstreamEventID)
		}
	}
}

func TestCopilotCLIHookEventRejectsUnknownEvents(t *testing.T) {
	t.Parallel()

	if _, err := copilotCLIHookEvent([]byte(`{}`), "unknown"); err == nil {
		t.Fatal("unknown event was accepted")
	}
}

func TestCopilotCLIHookEventDerivesPromptIdentityWithoutEventID(t *testing.T) {
	input := []byte(`{"sessionId":"session-1","cwd":"/repo","timestamp":1763294400000,"prompt":"hello"}`)
	first, err := copilotCLIHookEvent(input, "userPromptSubmitted")
	if err != nil {
		t.Fatal(err)
	}
	second, err := copilotCLIHookEvent(input, "userPromptSubmitted")
	if err != nil {
		t.Fatal(err)
	}
	if first.EventType != "interaction.prompt" || first.UpstreamEventID == "" || first.UpstreamEventID != second.UpstreamEventID {
		t.Fatalf("prompt identity = %#v / %#v", first, second)
	}
	if got := first.OccurredAt.UnixMilli(); got != 1763294400000 {
		t.Fatalf("occurred at = %d", got)
	}
}
