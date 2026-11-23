package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	radiobrowser "github.com/erbekin/radioman"
)

// PlayerState persists the background player's process and current station info.
type PlayerState struct {
	PID         int                   `json:"pid"`
	TimerPID    int                   `json:"timer_pid,omitempty"`
	Backend     string                `json:"backend"`
	BackendPath string                `json:"backend_path"`
	Playing     bool                  `json:"playing"`
	Station     *radiobrowser.Station `json:"station,omitempty"`
	FavoriteID  uint                  `json:"favorite_id,omitempty"`
	ShortName   string                `json:"sname,omitempty"`
	StartedAt   time.Time             `json:"started_at,omitempty"`
	StopAt      time.Time             `json:"stop_at,omitempty"`
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
	autoStop        time.Duration
	spawnTimer      func(statePath string, targetPID int, stopAt time.Time) (int, error)
}

func NewBackgroundController(statePath, preferredPlayer string) *BackgroundController {
	if statePath == "" {
		statePath = DefaultStatePath()
	}
	return &BackgroundController{
		statePath:       statePath,
		preferredPlayer: preferredPlayer,
		spawnTimer:      spawnDetachedAutoStopWorker,
	}
}

func (c *BackgroundController) SetAutoStop(d time.Duration) {
	if d < 0 {
		d = 0
	}
	c.autoStop = d
}

func (c *BackgroundController) AutoStop() time.Duration {
	return c.autoStop
}

func (c *BackgroundController) refreshState() (*PlayerState, error) {
	st, err := LoadPlayerState(c.statePath)
	if err != nil {
		return nil, err
	}
	if st.Playing && !isProcessAlive(st.PID, st.Backend) {
		if st.TimerPID > 0 {
			_ = killProcess(st.TimerPID, "")
		}
		st.Playing = false
		st.PID = 0
		st.TimerPID = 0
		st.StopAt = time.Time{}
		_ = SavePlayerState(c.statePath, st)
		return st, nil
	}
	if st.Playing && !st.StopAt.IsZero() && !time.Now().Before(st.StopAt) && (st.TimerPID == 0 || !isProcessAlive(st.TimerPID, "")) {
		_ = killProcess(st.PID, st.Backend)
		st.Playing = false
		st.PID = 0
		st.TimerPID = 0
		st.StopAt = time.Time{}
		st.StoppedAt = time.Now()
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
	if st.TimerPID > 0 {
		_ = killProcess(st.TimerPID, "")
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

	startedAt := time.Now()
	var stopAt time.Time
	if c.autoStop > 0 {
		stopAt = startedAt.Add(c.autoStop)
	}

	stCopy := station
	next := &PlayerState{
		PID:         pid,
		Backend:     backend.Name,
		BackendPath: backend.Path,
		Playing:     true,
		Station:     &stCopy,
		FavoriteID:  favID,
		ShortName:   sname,
		StartedAt:   startedAt,
		StopAt:      stopAt,
	}
	if err := SavePlayerState(c.statePath, next); err != nil {
		return nil, err
	}

	if c.autoStop > 0 {
		spawn := c.spawnTimer
		if spawn == nil {
			spawn = spawnDetachedAutoStopWorker
		}
		if timerPID, err := spawn(c.statePath, pid, stopAt); err == nil && timerPID > 0 {
			next.TimerPID = timerPID
			_ = SavePlayerState(c.statePath, next)
		}
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
	if st.TimerPID > 0 {
		_ = killProcess(st.TimerPID, "")
	}
	if st.PID > 0 {
		if err := killProcess(st.PID, st.Backend); err != nil {
			return nil, wasPlaying, err
		}
	}
	st.PID = 0
	st.TimerPID = 0
	st.Playing = false
	st.StopAt = time.Time{}
	st.StoppedAt = time.Now()
	if err := SavePlayerState(c.statePath, st); err != nil {
		return nil, wasPlaying, err
	}
	return st, wasPlaying, nil
}

func spawnDetachedAutoStopWorker(statePath string, targetPID int, stopAt time.Time) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	cmd := exec.Command(
		exe,
		"__auto-stop-worker",
		statePath,
		strconv.Itoa(targetPID),
		strconv.FormatInt(stopAt.UnixNano(), 10),
	)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	configureDetached(cmd)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	return pid, nil
}

func runAutoStopWorker(statePath string, targetPID int, stopAtUnixNano int64) error {
	if wait := time.Until(time.Unix(0, stopAtUnixNano)); wait > 0 {
		time.Sleep(wait)
	}

	st, err := LoadPlayerState(statePath)
	if err != nil {
		return err
	}
	if !st.Playing || st.PID != targetPID || st.StopAt.IsZero() || st.StopAt.UnixNano() != stopAtUnixNano {
		return nil
	}

	if st.PID > 0 {
		_ = killProcess(st.PID, st.Backend)
	}
	st.PID = 0
	st.TimerPID = 0
	st.Playing = false
	st.StopAt = time.Time{}
	st.StoppedAt = time.Now()
	return SavePlayerState(statePath, st)
}
