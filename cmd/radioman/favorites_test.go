package main

import (
	"os"
	"path/filepath"
	"testing"

	radiobrowser "github.com/erbekin/radioman"
)

func TestFavoritesStoreIDsAndSnames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "favorites.json")
	store, err := LoadFavoritesFromPath(path)
	if err != nil {
		t.Fatalf("LoadFavoritesFromPath: %v", err)
	}

	s1 := radiobrowser.Station{StationUUID: "u1", Name: "KralTürk", URL: "http://example.com/1"}
	s2 := radiobrowser.Station{StationUUID: "u2", Name: "KralTürk", URL: "http://example.com/2"}
	s3 := radiobrowser.Station{StationUUID: "u3", Name: "Jazz FM", URL: "http://example.com/3"}

	if added, err := store.Toggle(s1); err != nil || !added {
		t.Fatalf("Toggle s1: added=%v err=%v", added, err)
	}
	if added, err := store.Toggle(s2); err != nil || !added {
		t.Fatalf("Toggle s2: added=%v err=%v", added, err)
	}
	if added, err := store.Toggle(s3); err != nil || !added {
		t.Fatalf("Toggle s3: added=%v err=%v", added, err)
	}

	if store.Entries[0].ID != 1 || store.Entries[0].ShortName != "KralTurk" {
		t.Fatalf("unexpected entry 0: %+v", store.Entries[0])
	}
	if store.Entries[1].ID != 2 || store.Entries[1].ShortName != "KralTurk-2" {
		t.Fatalf("unexpected entry 1: %+v", store.Entries[1])
	}
	if store.Entries[2].ID != 3 || store.Entries[2].ShortName != "Jazz-FM" {
		t.Fatalf("unexpected entry 2: %+v", store.Entries[2])
	}

	// Remove #2 and ensure #3 keeps ID 3, and next added gets ID 4.
	if added, err := store.Toggle(s2); err != nil || added {
		t.Fatalf("Remove s2: added=%v err=%v", added, err)
	}
	s4 := radiobrowser.Station{StationUUID: "u4", Name: "Rock FM", URL: "http://example.com/4"}
	if added, err := store.Toggle(s4); err != nil || !added {
		t.Fatalf("Toggle s4: added=%v err=%v", added, err)
	}

	if fav, ok := store.Find("3"); !ok || fav.ShortName != "Jazz-FM" {
		t.Fatalf("Find(3) = %+v, ok=%v", fav, ok)
	}
	if fav, ok := store.Find("rock-fm"); !ok || fav.ID != 4 {
		t.Fatalf("Find(rock-fm) = %+v, ok=%v", fav, ok)
	}

	// SetShortName: reject invalid, reject duplicate, accept valid.
	if _, err := store.SetShortName("1", "bad name!"); err == nil {
		t.Fatal("expected error for invalid sname")
	}
	if _, err := store.SetShortName("1", "jazz-fm"); err == nil {
		t.Fatal("expected error for duplicate sname")
	}
	updated, err := store.SetShortName("1", "kral")
	if err != nil {
		t.Fatalf("SetShortName valid: %v", err)
	}
	if updated.ShortName != "kral" {
		t.Fatalf("updated.ShortName = %q, want %q", updated.ShortName, "kral")
	}
}

func TestFavoritesLegacyMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "favorites.json")
	legacyJSON := `[
  {"stationuuid":"a1","name":"Christmas FM Non-Stop!","url":"http://example.com/xmas"},
  {"stationuuid":"a2","name":"DFM Дискач 90-x","url":"http://example.com/dfm"}
]`
	if err := os.WriteFile(path, []byte(legacyJSON), 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	store, err := LoadFavoritesFromPath(path)
	if err != nil {
		t.Fatalf("LoadFavoritesFromPath: %v", err)
	}
	if store.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", store.Len())
	}
	if store.Entries[0].ID != 1 || store.Entries[0].ShortName != "Christmas-FM-Non-Stop" {
		t.Fatalf("entry 0 = %+v", store.Entries[0])
	}
	if store.Entries[1].ID != 2 || store.Entries[1].ShortName != "DFM-90-x" {
		t.Fatalf("entry 1 = %+v", store.Entries[1])
	}
}
