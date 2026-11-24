package radiobrowser

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenStreamIgnoresClientBodyTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			_, _ = w.Write([]byte("chunk1"))
			flusher.Flush()
			time.Sleep(250 * time.Millisecond)
			_, _ = w.Write([]byte("chunk2"))
			flusher.Flush()
		}
	}))
	defer srv.Close()

	client := NewClientWithHosts(&http.Client{Timeout: 100 * time.Millisecond}, nil)
	stream, err := client.OpenStream(context.Background(), Station{
		StationUUID: "stream-1",
		URLResolved: srv.URL,
	})
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	defer stream.Close()

	body, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("ReadAll past client timeout: %v", err)
	}
	if string(body) != "chunk1chunk2" {
		t.Fatalf("got body %q, want %q", string(body), "chunk1chunk2")
	}
}

func TestOpenStreamRespectsResponseHeaderTimeout(t *testing.T) {
	done := make(chan struct{})
	defer close(done)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClientWithHosts(&http.Client{Timeout: 80 * time.Millisecond}, nil)
	_, err := client.OpenStream(context.Background(), Station{
		StationUUID: "stream-2",
		URLResolved: srv.URL,
	})
	if err == nil {
		t.Fatal("expected OpenStream to fail when server stalls before sending headers")
	}
}
