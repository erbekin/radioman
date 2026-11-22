package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"

	radiobrowser "github.com/erbekin/radioman"
)

type PlayerBackend struct {
	Name string
	Path string
}

type Player struct {
	opMu        sync.Mutex
	mu          sync.Mutex
	backend     PlayerBackend
	bg          *BackgroundController
	pid         int
	ownedByTUI  bool
	metaCancel  context.CancelFunc
	current     *radiobrowser.Station
	streamTitle string
	streamBands [32]float64
	hasSignal   bool
}

func DetectPlayer(preferred string) (PlayerBackend, error) {
	if preferred != "" {
		path, err := findBinary(preferred)
		if err != nil {
			return PlayerBackend{}, fmt.Errorf("preferred player %q not found: %w", preferred, err)
		}
		return PlayerBackend{Name: strings.ToLower(preferred), Path: path}, nil
	}

	for _, candidate := range []string{"mpv", "ffplay", "vlc"} {
		if path, err := findBinary(candidate); err == nil {
			return PlayerBackend{Name: candidate, Path: path}, nil
		}
	}

	return PlayerBackend{}, errors.New("no supported audio player found (install mpv, ffplay, or VLC)")
}

func findBinary(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}

	switch strings.ToLower(name) {
	case "vlc":
		for _, path := range []string{
			`C:\Program Files\VideoLAN\VLC\vlc.exe`,
			`C:\Program Files (x86)\VideoLAN\VLC\vlc.exe`,
			`/Applications/VLC.app/Contents/MacOS/VLC`,
		} {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	case "mpv":
		for _, path := range []string{
			`C:\Program Files\mpv\mpv.exe`,
			`C:\Program Files (x86)\mpv\mpv.exe`,
		} {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	case "ffplay":
		for _, path := range []string{
			`C:\Program Files\ffmpeg\bin\ffplay.exe`,
		} {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("%s not found in PATH", name)
}

func NewPlayer(backend PlayerBackend, bg *BackgroundController) *Player {
	if bg == nil {
		bg = NewBackgroundController(DefaultStatePath(), backend.Name)
	}
	return &Player{
		backend: backend,
		bg:      bg,
	}
}

func buildPlayerArgs(backendName, streamURL string) []string {
	switch backendName {
	case "mpv":
		return []string{"--no-video", "--really-quiet", streamURL}
	case "ffplay":
		return []string{"-nodisp", "-autoexit", "-loglevel", "quiet", streamURL}
	default: // vlc
		return []string{"--intf", "dummy", "--no-video", "--quiet", streamURL}
	}
}

func (p *Player) BackendName() string {
	if p.backend.Name == "" {
		return "none"
	}
	return p.backend.Name
}

func (p *Player) Play(client *radiobrowser.Client, station radiobrowser.Station, favID uint, sname string) error {
	p.opMu.Lock()
	defer p.opMu.Unlock()

	st, err := p.bg.PlayStation(station, favID, sname)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.metaCancel != nil {
		p.metaCancel()
		p.metaCancel = nil
	}

	stCopy := station
	p.pid = st.PID
	p.current = &stCopy
	p.ownedByTUI = true
	p.streamTitle = ""
	p.streamBands = [32]float64{}
	p.hasSignal = false
	if st.Backend != "" && p.backend.Name == "" {
		p.backend = PlayerBackend{Name: st.Backend, Path: st.BackendPath}
	}

	if client != nil {
		metaCtx, cancel := context.WithCancel(context.Background())
		p.metaCancel = cancel
		go p.watchICYMetadata(metaCtx, client, stCopy)
	}

	return nil
}

func (p *Player) Stop() {
	p.opMu.Lock()
	defer p.opMu.Unlock()

	_, _, _ = p.bg.Stop()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.metaCancel != nil {
		p.metaCancel()
		p.metaCancel = nil
	}

	p.pid = 0
	p.current = nil
	p.ownedByTUI = false
	p.streamTitle = ""
	p.streamBands = [32]float64{}
	p.hasSignal = false
}

// SyncFromState reconciles the TUI player's in-memory state with player_state.json.
func (p *Player) SyncFromState(client *radiobrowser.Client) {
	p.opMu.Lock()
	defer p.opMu.Unlock()

	st, _, err := p.bg.Status()
	if err != nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !st.Playing || st.Station == nil {
		if p.current != nil {
			if p.metaCancel != nil {
				p.metaCancel()
				p.metaCancel = nil
			}
			p.pid = 0
			p.current = nil
			p.ownedByTUI = false
			p.streamTitle = ""
			p.streamBands = [32]float64{}
			p.hasSignal = false
		}
		return
	}

	if p.pid == st.PID && p.current != nil && sameStation(*p.current, *st.Station) {
		return
	}

	if p.metaCancel != nil {
		p.metaCancel()
		p.metaCancel = nil
	}

	stCopy := *st.Station
	p.pid = st.PID
	p.current = &stCopy
	p.ownedByTUI = false
	p.streamTitle = ""
	p.streamBands = [32]float64{}
	p.hasSignal = false
	if st.Backend != "" && p.backend.Name == "" {
		p.backend = PlayerBackend{Name: st.Backend, Path: st.BackendPath}
	}

	if client != nil {
		metaCtx, cancel := context.WithCancel(context.Background())
		p.metaCancel = cancel
		go p.watchICYMetadata(metaCtx, client, stCopy)
	}
}

// Shutdown stops playback on TUI exit when continueOnExit is false and the
// playback was started by this TUI session; otherwise background playback continues.
func (p *Player) Shutdown(continueOnExit bool) {
	p.opMu.Lock()
	p.mu.Lock()
	shouldStop := !continueOnExit && p.ownedByTUI && p.current != nil
	if p.metaCancel != nil {
		p.metaCancel()
		p.metaCancel = nil
	}
	p.mu.Unlock()
	p.opMu.Unlock()

	if shouldStop {
		p.Stop()
	}
}

func (p *Player) CurrentStation() *radiobrowser.Station {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == nil {
		return nil
	}
	cp := *p.current
	return &cp
}

func (p *Player) StreamTitle() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.streamTitle
}

func (p *Player) IsPlaying(station radiobrowser.Station) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == nil {
		return false
	}
	return sameStation(*p.current, station)
}

func (p *Player) AudioSpectrum() ([32]float64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.streamBands, p.hasSignal
}

func (p *Player) analyzeAudioChunk(station radiobrowser.Station, chunk []byte) {
	if len(chunk) < 64 {
		return
	}

	var bands [32]float64
	step := max(1, len(chunk)/32)
	for b := range 32 {
		start := b * step
		end := min(len(chunk), start+step)
		if start >= end {
			break
		}
		var sumDiff float64
		var zc int
		prev := int(chunk[start]) - 128
		for i := start + 1; i < end; i++ {
			cur := int(chunk[i]) - 128
			diff := math.Abs(float64(cur - prev))
			sumDiff += diff
			if (prev < 0 && cur >= 0) || (prev >= 0 && cur < 0) {
				zc++
			}
			prev = cur
		}
		n := float64(max(1, end-start-1))
		avgDiff := sumDiff / n / 128.0
		zcRate := float64(zc) / n

		// Shape frequency curve (stronger bass/mid, natural treble rolloff)
		freqPos := float64(b) / 31.0
		envelope := 1.05 - 0.35*freqPos
		val := (avgDiff*0.65 + zcRate*0.55) * envelope
		bands[b] = clamp01(val)
	}

	p.mu.Lock()
	if p.current != nil && sameStation(*p.current, station) {
		p.streamBands = bands
		p.hasSignal = true
	}
	p.mu.Unlock()
}

func (p *Player) watchICYMetadata(ctx context.Context, client *radiobrowser.Client, station radiobrowser.Station) {
	stream, err := client.OpenStream(ctx, station)
	if err != nil {
		return
	}
	defer stream.Close()

	if stream.MetadataInterval <= 0 {
		buf := make([]byte, 4096)
		for {
			n, err := stream.Read(buf)
			if n > 0 {
				p.analyzeAudioChunk(station, buf[:n])
			}
			if err != nil {
				return
			}
		}
	}

	audioBuf := make([]byte, stream.MetadataInterval)
	lenByte := make([]byte, 1)

	for {
		if _, err := io.ReadFull(stream, audioBuf); err != nil {
			return
		}
		p.analyzeAudioChunk(station, audioBuf)
		if _, err := io.ReadFull(stream, lenByte); err != nil {
			return
		}
		metaLen := int(lenByte[0]) * 16
		if metaLen == 0 {
			continue
		}
		metaBuf := make([]byte, metaLen)
		if _, err := io.ReadFull(stream, metaBuf); err != nil {
			return
		}
		title := parseStreamTitle(string(metaBuf))
		if title != "" {
			p.mu.Lock()
			if p.current != nil && sameStation(*p.current, station) {
				p.streamTitle = title
			}
			p.mu.Unlock()
		}
	}
}

func parseStreamTitle(raw string) string {
	raw = strings.TrimRight(raw, "\x00")
	const prefix = "StreamTitle='"
	start := strings.Index(raw, prefix)
	if start == -1 {
		return ""
	}
	rest := raw[start+len(prefix):]
	end := strings.Index(rest, "';")
	if end == -1 {
		end = strings.Index(rest, "'")
	}
	if end == -1 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}
