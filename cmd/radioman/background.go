package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	radiobrowser "github.com/erbekin/radioman"
)

// PlayerState persists the background player's process and current station info.
type PlayerState struct {
	PID         int                   `json:"pid"`
	Backend     string                `json:"backend"`
	BackendPath string                `json:"backend_path"`
	Playing     bool                  `json:"playing"`
	Station     *radiobrowser.Station `json:"station,omitempty"`
	FavoriteID  uint                  `json:"favorite_id,omitempty"`
	ShortName   string                `json:"sname,omitempty"`
	StartedAt   time.Time             `json:"started_at,omitempty"`
	StoppedAt   time.Time             `json:"stopped_at,omitempty"`
}

func DefaultStatePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	return filepath.Join(configDir, "radioman", "player_state.json")
}

func LoadPlayerState(path string) (*PlayerState, error) {
	st := &PlayerState{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, err
	}
	if len(data) == 0 {
		return st, nil
	}
	if err := json.Unmarshal(data, st); err != nil {
		return &PlayerState{}, err
	}
	return st, nil
}

func SavePlayerState(path string, st *PlayerState) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "player_state-*.tmp")
	if err != nil {
		return os.WriteFile(path, data, 0o644)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return os.WriteFile(path, data, 0o644)
	}
	return nil
}

type BackgroundController struct {
	statePath       string
	preferredPlayer string
}

func NewBackgroundController(statePath, preferredPlayer string) *BackgroundController {
	if statePath == "" {
		statePath = DefaultStatePath()
	}
	return &BackgroundController{
		statePath:       statePath,
		preferredPlayer: preferredPlayer,
	}
}

func (c *BackgroundController) refreshState() (*PlayerState, error) {
	st, err := LoadPlayerState(c.statePath)
	if err != nil {
		return nil, err
	}
	if st.Playing && !isProcessAlive(st.PID, st.Backend) {
		st.Playing = false
		st.PID = 0
		_ = SavePlayerState(c.statePath, st)
	}
	return st, nil
}

func (c *BackgroundController) Status() (*PlayerState, PlayerBackend, error) {
	st, err := c.refreshState()
	if err != nil {
		return nil, PlayerBackend{}, err
	}
	backend, detectErr := DetectPlayer(c.preferredPlayer)
	if detectErr != nil && st.Backend != "" {
		backend = PlayerBackend{Name: st.Backend, Path: st.BackendPath}
	}
	return st, backend, nil
}

func (c *BackgroundController) PlayStation(station radiobrowser.Station, favID uint, sname string) (*PlayerState, error) {
	st, err := c.refreshState()
	if err != nil {
		return nil, err
	}
	if st.Playing && st.PID > 0 {
		_ = killProcess(st.PID, st.Backend)
	}

	backend, err := DetectPlayer(c.preferredPlayer)
	if err != nil {
		return nil, err
	}

	streamURL, err := station.StreamURL()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(backend.Path, buildPlayerArgs(backend.Name, streamURL)...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	configureDetached(cmd)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", backend.Name, err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()

	stCopy := station
	next := &PlayerState{
		PID:         pid,
		Backend:     backend.Name,
		BackendPath: backend.Path,
		Playing:     true,
		Station:     &stCopy,
		FavoriteID:  favID,
		ShortName:   sname,
		StartedAt:   time.Now(),
	}
	if err := SavePlayerState(c.statePath, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (c *BackgroundController) PlayCurrent() (*PlayerState, error) {
	st, err := c.refreshState()
	if err != nil {
		return nil, err
	}
	if st.Station == nil {
		return nil, errors.New("no current station active")
	}
	if st.Playing {
		return st, nil
	}
	return c.PlayStation(*st.Station, st.FavoriteID, st.ShortName)
}

func (c *BackgroundController) Stop() (*PlayerState, bool, error) {
	st, err := c.refreshState()
	if err != nil {
		return nil, false, err
	}
	wasPlaying := st.Playing
	if st.PID > 0 {
		if err := killProcess(st.PID, st.Backend); err != nil {
			return nil, wasPlaying, err
		}
	}
	st.PID = 0
	st.Playing = false
	st.StoppedAt = time.Now()
	if err := SavePlayerState(c.statePath, st); err != nil {
		return nil, wasPlaying, err
	}
	return st, wasPlaying, nil
}
