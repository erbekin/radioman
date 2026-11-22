package main

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	radiobrowser "github.com/erbekin/radioman"
)

func newTestTUIModel(t *testing.T, opts TUIOptions) model {
	t.Helper()
	tmpDir := t.TempDir()
	favs, err := LoadFavoritesFromPath(filepath.Join(tmpDir, "favorites.json"))
	if err != nil {
		t.Fatalf("LoadFavoritesFromPath: %v", err)
	}
	settings := &SettingsStore{
		Settings: DefaultSettings(),
		path:     filepath.Join(tmpDir, "settings.json"),
	}
	bg := NewBackgroundController(filepath.Join(tmpDir, "player.json"), "")
	player := NewPlayer(PlayerBackend{Name: "mpv"}, bg)
	m := NewTUIModel(nil, player, favs, settings, opts)
	m.loading = false
	m.stations = []radiobrowser.Station{
		{
			StationUUID: "uuid-1",
			Name:        "Jazz FM London",
			Country:     "United Kingdom",
			CountryCode: "GB",
			Codec:       "MP3",
			Bitrate:     128,
			Tags:        "jazz,smooth,blues",
			URLResolved: "http://example.com/jazz.mp3",
		},
		{
			StationUUID: "uuid-2",
			Name:        "Classic Rock Radio",
			Country:     "United States",
			CountryCode: "US",
			Codec:       "AAC",
			Bitrate:     192,
			Tags:        "rock,classic",
			URLResolved: "http://example.com/rock.aac",
		},
	}
	return m
}

func TestTUIViewLayoutAndFooter(t *testing.T) {
	m := newTestTUIModel(t, TUIOptions{InitialTab: tabTop, Visualizer: true})

	for _, size := range []struct{ w, h int }{
		{100, 30},
		{80, 24},
		{60, 18},
	} {
		updated, _ := m.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		m = updated.(model)

		view := m.View()
		if gotH := lipgloss.Height(view); gotH != size.h {
			t.Errorf("size %dx%d: expected view height %d, got %d", size.w, size.h, size.h, gotH)
		}
		for i, line := range strings.Split(view, "\n") {
			if gotW := lipgloss.Width(line); gotW > size.w {
				t.Errorf("size %dx%d line %d exceeds width %d (got %d): %q", size.w, size.h, i+1, size.w, gotW, line)
			}
		}

		footer := m.renderFooter()
		if gotLines := lipgloss.Height(footer); gotLines != 1 {
			t.Errorf("size %dx%d: expected compact 1-line footer, got %d lines", size.w, size.h, gotLines)
		}
	}
}

func TestTUISearchBarStatesAndHelpToggle(t *testing.T) {
	m := newTestTUIModel(t, TUIOptions{
		InitialTab:     tabSearch,
		InitialName:    "jazz",
		InitialCountry: "GB",
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(model)

	// Not actively editing search filters -> compact 1-line summary bar
	summaryBar := m.renderSearchBar()
	if gotH := lipgloss.Height(summaryBar); gotH != 1 {
		t.Fatalf("expected 1-line search summary when not editing, got %d", gotH)
	}
	if !strings.Contains(summaryBar, "jazz") || !strings.Contains(summaryBar, "GB") {
		t.Errorf("expected search summary to show active filter chips, got: %q", summaryBar)
	}

	// Press '/' to open search editor -> 3-line focused panel
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(model)
	if !m.searching {
		t.Fatalf("expected searching=true after pressing '/'")
	}
	editBar := m.renderSearchBar()
	if gotH := lipgloss.Height(editBar); gotH != 3 {
		t.Fatalf("expected 3-line search panel while editing, got %d", gotH)
	}
	if gotH := lipgloss.Height(m.View()); gotH != 30 {
		t.Errorf("expected full view height 30 while editing search, got %d", gotH)
	}

	// Press Esc to exit search editing, then '?' to toggle help panel
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)
	if m.searching {
		t.Fatalf("expected searching=false after Esc")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(model)
	if !m.showHelp {
		t.Fatalf("expected showHelp=true after pressing '?'")
	}
	if !strings.Contains(m.View(), "Keyboard shortcuts") {
		t.Errorf("expected help panel in view when showHelp=true")
	}
	if gotH := lipgloss.Height(m.View()); gotH != 30 {
		t.Errorf("expected full view height 30 with help open, got %d", gotH)
	}
}
