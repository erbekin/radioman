package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erbekin/radioman/models"
)

func TestExpandCLIAliases(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{[]string{"fl"}, []string{"favs", "list"}},
		{[]string{"fp", "1"}, []string{"favs", "play", "1"}},
		{[]string{"pp"}, []string{"player", "play"}},
		{[]string{"ps"}, []string{"player", "stop"}},
		{[]string{"pc"}, []string{"player", "current"}},
	}

	for _, tc := range cases {
		got := expandCLIAliases(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("expandCLIAliases(%v) = %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("expandCLIAliases(%v)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestCLIWorkflow(t *testing.T) {
	dir := t.TempDir()
	favs, err := LoadFavoritesFromPath(filepath.Join(dir, "favorites.json"))
	if err != nil {
		t.Fatalf("LoadFavoritesFromPath: %v", err)
	}
	_, _ = favs.Toggle(models.Station{
		StationUUID: "u1",
		Name:        "KralTürk",
		URL:         "http://example.com/kral",
		Codec:       "MP3",
		Bitrate:     128,
		CountryCode: "TR",
	})

	statePath := filepath.Join(dir, "player_state.json")
	bg := NewBackgroundController(statePath, "")
	settings, err := LoadSettingsFromPath(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("LoadSettingsFromPath: %v", err)
	}

	// 1. favs list (via alias fl)
	var out bytes.Buffer
	if err := runCLIWithDeps([]string{"fl"}, favs, bg, settings, &out); err != nil {
		t.Fatalf("fl failed: %v", err)
	}
	if !strings.Contains(out.String(), "KralTurk") || !strings.Contains(out.String(), "1") {
		t.Fatalf("unexpected fl output: %s", out.String())
	}

	// 2. favs rename
	out.Reset()
	if err := runCLIWithDeps([]string{"favs", "rename", "1", "kral"}, favs, bg, settings, &out); err != nil {
		t.Fatalf("favs rename failed: %v", err)
	}
	if !strings.Contains(out.String(), "kral") {
		t.Fatalf("unexpected rename output: %s", out.String())
	}

	// 3. player play (pp) fails when no current station active
	out.Reset()
	if err := runCLIWithDeps([]string{"pp"}, favs, bg, settings, &out); err == nil {
		t.Fatal("expected pp to fail when no current station is active")
	}

	// 4. player (no-arg) prints status
	out.Reset()
	if err := runCLIWithDeps([]string{"player"}, favs, bg, settings, &out); err != nil {
		t.Fatalf("player status failed: %v", err)
	}
	if !strings.Contains(out.String(), "Status: stopped") {
		t.Fatalf("unexpected player output: %s", out.String())
	}

	// 5. player stop preserves remembered current station
	remembered := &PlayerState{
		Playing:    false,
		Station:    &favs.Entries[0].Station,
		FavoriteID: favs.Entries[0].ID,
		ShortName:  favs.Entries[0].ShortName,
	}
	if err := SavePlayerState(statePath, remembered); err != nil {
		t.Fatalf("SavePlayerState: %v", err)
	}

	out.Reset()
	if err := runCLIWithDeps([]string{"ps"}, favs, bg, settings, &out); err != nil {
		t.Fatalf("ps failed: %v", err)
	}
	out.Reset()
	if err := runCLIWithDeps([]string{"pc"}, favs, bg, settings, &out); err != nil {
		t.Fatalf("pc failed: %v", err)
	}
	if !strings.Contains(out.String(), "KralTürk") || !strings.Contains(out.String(), "#1 (kral)") {
		t.Fatalf("unexpected pc output: %s", out.String())
	}

	// 6. settings CLI command
	out.Reset()
	if err := runCLIWithDeps([]string{"settings"}, favs, bg, settings, &out); err != nil {
		t.Fatalf("settings failed: %v", err)
	}
	if !strings.Contains(out.String(), "continue_on_exit: true") || !strings.Contains(out.String(), "auto_stop: 0") {
		t.Fatalf("unexpected settings output: %s", out.String())
	}

	out.Reset()
	if err := runCLIWithDeps([]string{"settings", "continue_on_exit", "off"}, favs, bg, settings, &out); err != nil {
		t.Fatalf("settings off failed: %v", err)
	}
	if !strings.Contains(out.String(), "continue_on_exit: false") || settings.ContinueOnExit {
		t.Fatalf("expected continue_on_exit: false, got %s (val=%v)", out.String(), settings.ContinueOnExit)
	}

	out.Reset()
	if err := runCLIWithDeps([]string{"settings", "auto_stop", "10m"}, favs, bg, settings, &out); err != nil {
		t.Fatalf("settings auto_stop 10m failed: %v", err)
	}
	if !strings.Contains(out.String(), "auto_stop: 10m") || settings.AutoStop != "10m" {
		t.Fatalf("expected auto_stop: 10m, got %s (val=%q)", out.String(), settings.AutoStop)
	}

	out.Reset()
	if err := runCLIWithDeps([]string{"settings", "auto_stop", "0"}, favs, bg, settings, &out); err != nil {
		t.Fatalf("settings auto_stop 0 failed: %v", err)
	}
	if !strings.Contains(out.String(), "auto_stop: 0") || settings.AutoStop != "0" {
		t.Fatalf("expected auto_stop: 0, got %s (val=%q)", out.String(), settings.AutoStop)
	}

	out.Reset()
	if err := runCLIWithDeps([]string{"settings", "auto_stop", "badunit"}, favs, bg, settings, &out); err == nil {
		t.Fatal("expected invalid auto_stop value to fail")
	}

	// 7. player / pc status output with active StopAt
	active := &PlayerState{
		PID:        os.Getpid(),
		Playing:    true,
		Station:    &favs.Entries[0].Station,
		FavoriteID: favs.Entries[0].ID,
		ShortName:  favs.Entries[0].ShortName,
		StopAt:     time.Now().Add(5 * time.Minute),
	}
	if err := SavePlayerState(statePath, active); err != nil {
		t.Fatalf("SavePlayerState active: %v", err)
	}
	out.Reset()
	if err := runCLIWithDeps([]string{"player"}, favs, bg, settings, &out); err != nil {
		t.Fatalf("player with active StopAt failed: %v", err)
	}
	if !strings.Contains(out.String(), "Auto-stop: in ") {
		t.Fatalf("expected Auto-stop line in player output, got: %s", out.String())
	}
	out.Reset()
	if err := runCLIWithDeps([]string{"pc"}, favs, bg, settings, &out); err != nil {
		t.Fatalf("pc with active StopAt failed: %v", err)
	}
	if !strings.Contains(out.String(), "Auto-stop: in ") {
		t.Fatalf("expected Auto-stop line in pc output, got: %s", out.String())
	}
}

func TestResolveAutoStopOverride(t *testing.T) {
	settings := &SettingsStore{
		Settings: Settings{
			ContinueOnExit: true,
			AutoStop:       "10m",
		},
	}

	d, err := resolveAutoStop(settings, nil)
	if err != nil || d != 10*time.Minute {
		t.Fatalf("resolveAutoStop(nil) = (%v, %v), want 10m", d, err)
	}

	override := 2 * time.Minute
	d, err = resolveAutoStop(settings, &override)
	if err != nil || d != 2*time.Minute {
		t.Fatalf("resolveAutoStop(2m) = (%v, %v), want 2m", d, err)
	}

	zero := time.Duration(0)
	d, err = resolveAutoStop(settings, &zero)
	if err != nil || d != 0 {
		t.Fatalf("resolveAutoStop(0) = (%v, %v), want 0", d, err)
	}
}
