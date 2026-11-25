package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	radiobrowser "github.com/erbekin/radioman"
	"github.com/erbekin/radioman/models"
)

func writeICYFrame(w http.ResponseWriter, metaint int, title string) {
	audio := make([]byte, metaint)
	_, _ = w.Write(audio)

	if title == "" {
		_, _ = w.Write([]byte{0})
	} else {
		raw := fmt.Sprintf("StreamTitle='%s';", title)
		blocks := (len(raw) + 15) / 16
		buf := make([]byte, 1+blocks*16)
		buf[0] = byte(blocks)
		copy(buf[1:], raw)
		_, _ = w.Write(buf)
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, cond func() bool, desc string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", desc)
}

func TestWatchICYMetadataUpdatesAndReconnects(t *testing.T) {
	origInit := icyInitialBackoff
	origMax := icyMaxBackoff
	icyInitialBackoff = 10 * time.Millisecond
	icyMaxBackoff = 50 * time.Millisecond
	t.Cleanup(func() {
		icyInitialBackoff = origInit
		icyMaxBackoff = origMax
	})

	var connCount atomic.Int32
	const metaint = 16

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Icy-MetaData") != "1" {
			t.Errorf("expected Icy-MetaData=1 header, got %q", r.Header.Get("Icy-MetaData"))
		}
		n := connCount.Add(1)
		w.Header().Set("icy-metaint", "16")
		w.WriteHeader(http.StatusOK)

		if n == 1 {
			writeICYFrame(w, metaint, "Track One")
			// Sleep past the client's 50ms Timeout to verify stream is not aborted by body timeout.
			time.Sleep(90 * time.Millisecond)
			writeICYFrame(w, metaint, "Track Two")
			// Return to simulate a transient stream drop.
			return
		}

		writeICYFrame(w, metaint, "Track Three")
		<-r.Context().Done()
	}))
	defer srv.Close()

	client := radiobrowser.NewClientWithHosts(&http.Client{Timeout: 50 * time.Millisecond}, nil)
	bg := NewBackgroundController(filepath.Join(t.TempDir(), "player_state.json"), "")
	p := NewPlayer(PlayerBackend{Name: "mpv"}, bg)

	station := models.Station{
		StationUUID: "icy-1",
		Name:        "ICY Test FM",
		URLResolved: srv.URL,
	}
	p.mu.Lock()
	p.current = &station
	p.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.watchICYMetadata(ctx, client, station)
	}()

	waitForCondition(t, 2*time.Second, func() bool {
		return p.StreamTitle() == "Track Three"
	}, "StreamTitle to update to Track Three after reconnect")

	if got := connCount.Load(); got < 2 {
		t.Fatalf("expected at least 2 connections (reconnect after drop), got %d", got)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watchICYMetadata did not exit after context cancellation")
	}
}

func TestWatchICYMetadataStopsWhenUnsupported(t *testing.T) {
	var connCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connCount.Add(1)
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := radiobrowser.NewClientWithHosts(&http.Client{Timeout: time.Second}, nil)
	bg := NewBackgroundController(filepath.Join(t.TempDir(), "player_state.json"), "")
	p := NewPlayer(PlayerBackend{Name: "mpv"}, bg)

	station := models.Station{
		StationUUID: "icy-unsupported",
		Name:        "No ICY FM",
		URLResolved: srv.URL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		p.watchICYMetadata(ctx, client, station)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watchICYMetadata did not exit for non-ICY stream")
	}

	if got := connCount.Load(); got != 1 {
		t.Fatalf("expected exactly 1 connection for non-ICY stream, got %d", got)
	}
}
