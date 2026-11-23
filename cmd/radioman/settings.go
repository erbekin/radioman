package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Settings holds persistent user preferences for radioman.
type Settings struct {
	ContinueOnExit bool   `json:"continue_on_exit"`
	AutoStop       string `json:"auto_stop"`
}

// SettingsStore manages loading and saving Settings on disk.
type SettingsStore struct {
	path string
	Settings
}

func DefaultSettings() Settings {
	return Settings{
		ContinueOnExit: true,
		AutoStop:       "0",
	}
}

func DefaultSettingsPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	return filepath.Join(configDir, "radioman", "settings.json")
}

func LoadSettings() (*SettingsStore, error) {
	return LoadSettingsFromPath(DefaultSettingsPath())
}

func LoadSettingsFromPath(path string) (*SettingsStore, error) {
	store := &SettingsStore{
		path:     path,
		Settings: DefaultSettings(),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return store, err
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return store, nil
	}

	if err := json.Unmarshal(data, &store.Settings); err != nil {
		store.Settings = DefaultSettings()
		return store, err
	}
	if strings.TrimSpace(store.Settings.AutoStop) == "" {
		store.Settings.AutoStop = "0"
	}
	return store, nil
}

func (s *SettingsStore) Save() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.Settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func (s *SettingsStore) ToggleContinueOnExit() (bool, error) {
	s.ContinueOnExit = !s.ContinueOnExit
	return s.ContinueOnExit, s.Save()
}

func (s *SettingsStore) SetContinueOnExit(v bool) error {
	s.ContinueOnExit = v
	return s.Save()
}

func (s *SettingsStore) AutoStopDuration() (time.Duration, error) {
	return ParseAutoStopDuration(s.AutoStop)
}

func (s *SettingsStore) SetAutoStop(raw string) (string, time.Duration, error) {
	norm, dur, err := NormalizeAutoStopValue(raw)
	if err != nil {
		return "", 0, err
	}
	s.AutoStop = norm
	if err := s.Save(); err != nil {
		return "", 0, err
	}
	return norm, dur, nil
}
