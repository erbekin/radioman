package radiobrowser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Stream is an open audio stream returned by RadioBrowser.
//
// The reader contains the stream's audio bytes. If MetadataInterval is
// greater than zero, ICY metadata may be interleaved at that interval.
type Stream struct {
	io.ReadCloser

	URL              string
	ContentType      string
	MetadataInterval int
	Response         *http.Response
}

// StreamURL returns the resolved stream URL when available, otherwise URL.
func (s Station) StreamURL() (string, error) {
	streamURL := strings.TrimSpace(s.URLResolved)
	if streamURL == "" {
		streamURL = strings.TrimSpace(s.URL)
	}
	if streamURL == "" {
		return "", fmt.Errorf("station %q has no stream URL", s.StationUUID)
	}
	return streamURL, nil
}

// OpenStream opens a station's audio stream.
//
// The caller must close the returned stream. The request asks the server for
// ICY metadata; MetadataInterval indicates where that metadata is interleaved.
func (c *Client) OpenStream(ctx context.Context, station Station) (*Stream, error) {
	streamURL, err := station.StreamURL()
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create stream request: %w", err)
	}
	request.Header.Set("Icy-MetaData", "1")

	streamClient := c.streamClient
	if streamClient == nil {
		streamClient = newStreamHTTPClient(c.httpClient)
	}
	response, err := streamClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("open stream: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		response.Body.Close()
		return nil, fmt.Errorf("stream returned status %s", response.Status)
	}

	metadataInterval := 0
	if value := response.Header.Get("icy-metaint"); value != "" {
		metadataInterval, err = strconv.Atoi(value)
		if err != nil || metadataInterval < 0 {
			metadataInterval = 0
		}
	}

	return &Stream{
		ReadCloser:       response.Body,
		URL:              streamURL,
		ContentType:      response.Header.Get("Content-Type"),
		MetadataInterval: metadataInterval,
		Response:         response,
	}, nil
}
