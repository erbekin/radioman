package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erbekin/radioman/models"
)

func TestPlayerSyncAndShutdown(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "player_state.json")
	st := models.Station{
		StationUUID: "sync-1",
		Name:        "Sync Radio",
		URL:         "http://example.com/sync",
	}

	// Simulate an active background player started externally (e.g., via CLI).
	stopAt := time.Now().Add(10 * time.Minute)
	initial := &PlayerState{
		PID:        os.Getpid(),
		Backend:    "",
		Playing:    true,
		Station:    &st,
		FavoriteID: 7,
		ShortName:  "Sync-Radio",
		StopAt:     stopAt,
	}
	if err := SavePlayerState(statePath, initial); err != nil {
		t.Fatalf("SavePlayerState: %v", err)
	}

	bg := NewBackgroundController(statePath, "")
	p := NewPlayer(PlayerBackend{}, bg)

	// SyncFromState should attach to the active background station.
	p.SyncFromState(nil)
	if !p.IsPlaying(st) {
		t.Fatal("expected player to be playing Sync Radio after SyncFromState")
	}
	cur := p.CurrentStation()
	if cur == nil || cur.StationUUID != st.StationUUID {
		t.Fatalf("CurrentStation() = %+v, want %q", cur, st.StationUUID)
	}
	if !p.StopAt().Equal(stopAt) {
		t.Fatalf("StopAt() = %v, want %v", p.StopAt(), stopAt)
	}

	// Shutdown on an externally-started session must leave background playback intact.
	p.Shutdown(false)
	afterShutdown, err := LoadPlayerState(statePath)
	if err != nil {
		t.Fatalf("LoadPlayerState: %v", err)
	}
	if !afterShutdown.Playing || afterShutdown.PID != initial.PID {
		t.Fatalf("expected Shutdown to preserve external playback, got %+v", afterShutdown)
	}

	// SyncFromState should clear CurrentStation when state transitions to stopped externally.
	initial.Playing = false
	initial.PID = 0
	if err := SavePlayerState(statePath, initial); err != nil {
		t.Fatalf("SavePlayerState stopped: %v", err)
	}
	p.SyncFromState(nil)
	if p.CurrentStation() != nil {
		t.Fatalf("expected CurrentStation() to be nil after external stop, got %+v", p.CurrentStation())
	}
}

func TestPlayerStopAttachedPreservesStation(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "player_state.json")
	st := models.Station{
		StationUUID: "sync-2",
		Name:        "Stop Test Radio",
		URL:         "http://example.com/stop",
	}

	// Use an invalid/non-existent PID so Stop updates state without killing our test process.
	initial := &PlayerState{
		PID:        0,
		Backend:    "vlc",
		Playing:    false,
		Station:    &st,
		FavoriteID: 3,
		ShortName:  "Stop-Test",
	}
	if err := SavePlayerState(statePath, initial); err != nil {
		t.Fatalf("SavePlayerState: %v", err)
	}

	bg := NewBackgroundController(statePath, "")
	p := NewPlayer(PlayerBackend{}, bg)
	p.Stop()

	after, err := LoadPlayerState(statePath)
	if err != nil {
		t.Fatalf("LoadPlayerState: %v", err)
	}
	if after.Playing {
		t.Fatal("expected Playing to be false after Stop")
	}
	if after.Station == nil || after.Station.StationUUID != st.StationUUID || after.FavoriteID != 3 || after.ShortName != "Stop-Test" {
		t.Fatalf("expected Stop to preserve remembered station metadata, got %+v", after)
	}
}

func TestPlayerShutdownContinueOnExit(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "player_state.json")
	st := models.Station{
		StationUUID: "sync-3",
		Name:        "Continue Radio",
		URL:         "http://example.com/continue",
	}

	initial := &PlayerState{
		PID:     os.Getpid(),
		Playing: true,
		Station: &st,
	}
	if err := SavePlayerState(statePath, initial); err != nil {
		t.Fatalf("SavePlayerState: %v", err)
	}

	bg := NewBackgroundController(statePath, "")
	p := NewPlayer(PlayerBackend{}, bg)
	p.SyncFromState(nil)

	// Simulate playback started by this TUI session.
	p.mu.Lock()
	p.ownedByTUI = true
	p.mu.Unlock()

	p.Shutdown(true)

	after, err := LoadPlayerState(statePath)
	if err != nil {
		t.Fatalf("LoadPlayerState: %v", err)
	}
	if !after.Playing || after.PID != initial.PID {
		t.Fatalf("expected Shutdown(true) to keep TUI-started playback running, got %+v", after)
	}
}

func TestRunAutoStopWorker(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "player_state.json")
	st := models.Station{
		StationUUID: "auto-1",
		Name:        "Timer Radio",
		URL:         "http://example.com/timer",
	}
	stopAt := time.Now().Add(-10 * time.Millisecond)

	// 1. Mismatched PID should not stop playback.
	initial := &PlayerState{
		PID:      99999999,
		TimerPID: 88888888,
		Playing:  true,
		Station:  &st,
		StopAt:   stopAt,
	}
	if err := SavePlayerState(statePath, initial); err != nil {
		t.Fatalf("SavePlayerState: %v", err)
	}
	if err := runAutoStopWorker(statePath, 11111111, stopAt.UnixNano()); err != nil {
		t.Fatalf("runAutoStopWorker mismatch PID: %v", err)
	}
	afterMismatch, err := LoadPlayerState(statePath)
	if err != nil {
		t.Fatalf("LoadPlayerState: %v", err)
	}
	if !afterMismatch.Playing || afterMismatch.PID != 99999999 {
		t.Fatalf("expected mismatched PID not to stop state, got %+v", afterMismatch)
	}

	// 2. Mismatched StopAt should not stop playback.
	if err := runAutoStopWorker(statePath, 99999999, stopAt.Add(time.Second).UnixNano()); err != nil {
		t.Fatalf("runAutoStopWorker mismatch StopAt: %v", err)
	}
	afterStopMismatch, err := LoadPlayerState(statePath)
	if err != nil {
		t.Fatalf("LoadPlayerState: %v", err)
	}
	if !afterStopMismatch.Playing {
		t.Fatalf("expected mismatched StopAt not to stop state, got %+v", afterStopMismatch)
	}

	// 3. Matching PID and StopAt should stop playback and clear timer fields.
	if err := runAutoStopWorker(statePath, 99999999, stopAt.UnixNano()); err != nil {
		t.Fatalf("runAutoStopWorker matching: %v", err)
	}
	afterMatch, err := LoadPlayerState(statePath)
	if err != nil {
		t.Fatalf("LoadPlayerState: %v", err)
	}
	if afterMatch.Playing || afterMatch.PID != 0 || afterMatch.TimerPID != 0 || !afterMatch.StopAt.IsZero() {
		t.Fatalf("expected matching worker to clear playing state, got %+v", afterMatch)
	}
}
