package main

import (
	"path/filepath"
	"testing"
)

func TestSettingsDefaultsAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	s, err := LoadSettingsFromPath(path)
	if err != nil {
		t.Fatalf("LoadSettingsFromPath: %v", err)
	}
	if !s.ContinueOnExit {
		t.Fatal("expected ContinueOnExit to default to true")
	}

	if err := s.SetContinueOnExit(false); err != nil {
		t.Fatalf("SetContinueOnExit(false): %v", err)
	}

	reloaded, err := LoadSettingsFromPath(path)
	if err != nil {
		t.Fatalf("reload LoadSettingsFromPath: %v", err)
	}
	if reloaded.ContinueOnExit {
		t.Fatal("expected reloaded ContinueOnExit to be false")
	}

	next, err := reloaded.ToggleContinueOnExit()
	if err != nil {
		t.Fatalf("ToggleContinueOnExit: %v", err)
	}
	if !next || !reloaded.ContinueOnExit {
		t.Fatal("expected ToggleContinueOnExit to flip back to true")
	}
}
