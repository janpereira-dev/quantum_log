package sqlite

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestInteractionRecordsOneRootForEachPromptAndLinksChildren(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	for i := 0; i < 100; i++ {
		if _, created, err := store.RecordInteraction(ctx, InteractionInput{Source: "test", SessionID: "session", UpstreamID: "prompt-" + strconv.Itoa(i), OccurredAt: time.Now().UTC()}); err != nil || !created {
			t.Fatalf("RecordInteraction(%d) = created:%t err:%v", i, created, err)
		}
	}
	interactionID, created, err := store.RecordInteraction(ctx, InteractionInput{Source: "test", SessionID: "session", UpstreamID: "prompt-0", OccurredAt: time.Now().UTC()})
	if err != nil || created {
		t.Fatalf("duplicate interaction = %q created:%t err:%v", interactionID, created, err)
	}
	count, err := store.InteractionCount(ctx, "")
	if err != nil || count != 100 {
		t.Fatalf("InteractionCount() = %d, %v; want 100", count, err)
	}
}
