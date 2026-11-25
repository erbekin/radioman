package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/erbekin/radioman/models"
)

// Favorite represents a saved station with a unique numeric ID and short name.
type Favorite struct {
	ID        uint                 `json:"id"`
	ShortName string               `json:"sname"`
	Station   models.Station `json:"station"`
}

type favoritesFile struct {
	NextID    uint       `json:"next_id"`
	Favorites []Favorite `json:"favorites"`
}

type FavoritesStore struct {
	path    string
	NextID  uint
	Entries []Favorite
}

func DefaultFavoritesPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	return filepath.Join(configDir, "radioman", "favorites.json")
}

func LoadFavorites() (*FavoritesStore, error) {
	return LoadFavoritesFromPath(DefaultFavoritesPath())
}

func LoadFavoritesFromPath(path string) (*FavoritesStore, error) {
	store := &FavoritesStore{
		path:    path,
		NextID:  1,
		Entries: []Favorite{},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return store, err
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return store, nil
	}

	// Backward compatibility: migrate legacy []models.Station array.
	if trimmed[0] == '[' {
		var legacy []models.Station
		if err := json.Unmarshal(trimmed, &legacy); err != nil {
			return store, err
		}
		existingNames := make([]string, 0, len(legacy))
		for i, st := range legacy {
			id := uint(i + 1)
			sname := GenerateUniqueShortName(st.Name, existingNames)
			existingNames = append(existingNames, sname)
			store.Entries = append(store.Entries, Favorite{
				ID:        id,
				ShortName: sname,
				Station:   st,
			})
		}
		store.NextID = uint(len(legacy) + 1)
		_ = store.Save()
		return store, nil
	}

	var file favoritesFile
	if err := json.Unmarshal(trimmed, &file); err != nil {
		return store, err
	}

	store.Entries = file.Favorites
	if store.Entries == nil {
		store.Entries = []Favorite{}
	}
	store.NextID = file.NextID

	if store.repair() {
		_ = store.Save()
	}

	return store, nil
}

func (f *FavoritesStore) repair() bool {
	changed := false
	var maxID uint
	seenIDs := make(map[uint]struct{}, len(f.Entries))
	for _, e := range f.Entries {
		if e.ID > 0 {
			seenIDs[e.ID] = struct{}{}
			if e.ID > maxID {
				maxID = e.ID
			}
		}
	}

	if f.NextID <= maxID {
		f.NextID = maxID + 1
		changed = true
	}
	if f.NextID == 0 {
		f.NextID = 1
		changed = true
	}

	usedNames := make([]string, 0, len(f.Entries))
	usedSet := make(map[string]struct{}, len(f.Entries))

	for i := range f.Entries {
		if f.Entries[i].ID == 0 {
			for {
				if _, ok := seenIDs[f.NextID]; !ok {
					break
				}
				f.NextID++
			}
			f.Entries[i].ID = f.NextID
			seenIDs[f.NextID] = struct{}{}
			f.NextID++
			changed = true
		}

		sname := f.Entries[i].ShortName
		_, dup := usedSet[strings.ToLower(sname)]
		if ValidateShortName(sname) != nil || dup {
			sname = GenerateUniqueShortName(f.Entries[i].Station.Name, usedNames)
			f.Entries[i].ShortName = sname
			changed = true
		}
		usedNames = append(usedNames, sname)
		usedSet[strings.ToLower(sname)] = struct{}{}
	}

	return changed
}

func (f *FavoritesStore) Save() error {
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}
	payload := favoritesFile{
		NextID:    f.NextID,
		Favorites: f.Entries,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(f.path, data, 0o644)
}

func (f *FavoritesStore) Stations() []models.Station {
	stations := make([]models.Station, len(f.Entries))
	for i, e := range f.Entries {
		stations[i] = e.Station
	}
	return stations
}

func (f *FavoritesStore) Len() int {
	return len(f.Entries)
}

func (f *FavoritesStore) existingNames(excludeIdx int) []string {
	names := make([]string, 0, len(f.Entries))
	for i, e := range f.Entries {
		if i == excludeIdx {
			continue
		}
		names = append(names, e.ShortName)
	}
	return names
}

func (f *FavoritesStore) IsFavorite(station models.Station) bool {
	return slices.ContainsFunc(f.Entries, func(e Favorite) bool {
		return sameStation(e.Station, station)
	})
}

func (f *FavoritesStore) GetByStation(station models.Station) (Favorite, bool) {
	for _, e := range f.Entries {
		if sameStation(e.Station, station) {
			return e, true
		}
	}
	return Favorite{}, false
}

func (f *FavoritesStore) Toggle(station models.Station) (added bool, err error) {
	idx := slices.IndexFunc(f.Entries, func(e Favorite) bool {
		return sameStation(e.Station, station)
	})
	if idx >= 0 {
		f.Entries = slices.Delete(f.Entries, idx, idx+1)
		return false, f.Save()
	}

	if f.NextID == 0 {
		f.NextID = 1
	}
	id := f.NextID
	f.NextID++

	sname := GenerateUniqueShortName(station.Name, f.existingNames(-1))
	f.Entries = append(f.Entries, Favorite{
		ID:        id,
		ShortName: sname,
		Station:   station,
	})
	return true, f.Save()
}

// Find looks up a favorite by numeric ID, exact sname, or case-insensitive sname.
func (f *FavoritesStore) Find(identifier string) (Favorite, bool) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return Favorite{}, false
	}

	if idVal, err := strconv.ParseUint(identifier, 10, 64); err == nil {
		for _, e := range f.Entries {
			if uint64(e.ID) == idVal {
				return e, true
			}
		}
	}

	for _, e := range f.Entries {
		if e.ShortName == identifier {
			return e, true
		}
	}

	for _, e := range f.Entries {
		if strings.EqualFold(e.ShortName, identifier) {
			return e, true
		}
	}

	return Favorite{}, false
}

// SetShortName updates the sname of the favorite identified by identifier (ID or sname).
func (f *FavoritesStore) SetShortName(identifier string, newSname string) (Favorite, error) {
	newSname = strings.TrimSpace(newSname)
	if err := ValidateShortName(newSname); err != nil {
		return Favorite{}, err
	}

	target, ok := f.Find(identifier)
	if !ok {
		return Favorite{}, fmt.Errorf("favorite %q not found", identifier)
	}

	targetIdx := -1
	for i, e := range f.Entries {
		if e.ID == target.ID {
			targetIdx = i
			continue
		}
		if strings.EqualFold(e.ShortName, newSname) {
			return Favorite{}, fmt.Errorf("short name %q is already used by #%d (%s)", newSname, e.ID, e.Station.Name)
		}
	}

	if targetIdx < 0 {
		return Favorite{}, fmt.Errorf("favorite %q not found", identifier)
	}

	f.Entries[targetIdx].ShortName = newSname
	if err := f.Save(); err != nil {
		return Favorite{}, err
	}
	return f.Entries[targetIdx], nil
}

// SetShortNameByStation updates the sname of the given favorited station.
func (f *FavoritesStore) SetShortNameByStation(station models.Station, newSname string) (Favorite, error) {
	newSname = strings.TrimSpace(newSname)
	if err := ValidateShortName(newSname); err != nil {
		return Favorite{}, err
	}

	targetIdx := slices.IndexFunc(f.Entries, func(e Favorite) bool {
		return sameStation(e.Station, station)
	})
	if targetIdx < 0 {
		return Favorite{}, errors.New("station is not in favorites")
	}

	for i, e := range f.Entries {
		if i == targetIdx {
			continue
		}
		if strings.EqualFold(e.ShortName, newSname) {
			return Favorite{}, fmt.Errorf("short name %q is already used by #%d (%s)", newSname, e.ID, e.Station.Name)
		}
	}

	f.Entries[targetIdx].ShortName = newSname
	if err := f.Save(); err != nil {
		return Favorite{}, err
	}
	return f.Entries[targetIdx], nil
}

func sameStation(a, b models.Station) bool {
	if a.StationUUID != "" && b.StationUUID != "" {
		return a.StationUUID == b.StationUUID
	}
	return a.Name == b.Name && a.URL == b.URL
}
