package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetPromptCaptureModePersistsValidatedMode(t *testing.T) {
	paths := Paths{Home: t.TempDir()}
	paths.ConfigFile = filepath.Join(paths.Home, "config.yaml")
	if err := Ensure(paths); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"off", "hash", "full"} {
		if err := SetPromptCaptureMode(paths, mode); err != nil {
			t.Fatalf("SetPromptCaptureMode(%s): %v", mode, err)
		}
		contents, err := os.ReadFile(paths.ConfigFile)
		if err != nil || !strings.Contains(string(contents), "promptCapture: "+mode) {
			t.Fatalf("config=%q err=%v", contents, err)
		}
	}
	if err := SetPromptCaptureMode(paths, "unsafe"); err == nil {
		t.Fatal("accepted invalid prompt mode")
	}
}
