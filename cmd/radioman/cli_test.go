package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	radiobrowser "github.com/erbekin/radioman"
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
	_, _ = favs.Toggle(radiobrowser.Station{
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
	if !strings.Contains(out.String(), "continue_on_exit: true") {
		t.Fatalf("unexpected settings output: %s", out.String())
	}

	out.Reset()
	if err := runCLIWithDeps([]string{"settings", "continue_on_exit", "off"}, favs, bg, settings, &out); err != nil {
		t.Fatalf("settings off failed: %v", err)
	}
	if !strings.Contains(out.String(), "continue_on_exit: false") || settings.ContinueOnExit {
		t.Fatalf("expected continue_on_exit: false, got %s (val=%v)", out.String(), settings.ContinueOnExit)
	}
}
