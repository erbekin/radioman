package main

import (
	"path/filepath"
	"testing"
	"time"
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
	if s.AutoStop != "0" {
		t.Fatalf("expected AutoStop to default to \"0\", got %q", s.AutoStop)
	}
	if d, err := s.AutoStopDuration(); err != nil || d != 0 {
		t.Fatalf("expected AutoStopDuration() = (0, nil), got (%v, %v)", d, err)
	}

	if err := s.SetContinueOnExit(false); err != nil {
		t.Fatalf("SetContinueOnExit(false): %v", err)
	}
	if norm, dur, err := s.SetAutoStop("10m"); err != nil || norm != "10m" || dur != 10*time.Minute {
		t.Fatalf("SetAutoStop(\"10m\") = (%q, %v, %v)", norm, dur, err)
	}

	reloaded, err := LoadSettingsFromPath(path)
	if err != nil {
		t.Fatalf("reload LoadSettingsFromPath: %v", err)
	}
	if reloaded.ContinueOnExit {
		t.Fatal("expected reloaded ContinueOnExit to be false")
	}
	if reloaded.AutoStop != "10m" {
		t.Fatalf("expected reloaded AutoStop to be \"10m\", got %q", reloaded.AutoStop)
	}
	if d, err := reloaded.AutoStopDuration(); err != nil || d != 10*time.Minute {
		t.Fatalf("expected reloaded AutoStopDuration() = 10m, got (%v, %v)", d, err)
	}

	if _, _, err := reloaded.SetAutoStop("invalid"); err == nil {
		t.Fatal("expected SetAutoStop(\"invalid\") to fail")
	}
	if reloaded.AutoStop != "10m" {
		t.Fatalf("expected invalid SetAutoStop not to mutate AutoStop, got %q", reloaded.AutoStop)
	}

	if norm, dur, err := reloaded.SetAutoStop("off"); err != nil || norm != "0" || dur != 0 {
		t.Fatalf("SetAutoStop(\"off\") = (%q, %v, %v)", norm, dur, err)
	}

	next, err := reloaded.ToggleContinueOnExit()
	if err != nil {
		t.Fatalf("ToggleContinueOnExit: %v", err)
	}
	if !next || !reloaded.ContinueOnExit {
		t.Fatal("expected ToggleContinueOnExit to flip back to true")
	}
}
