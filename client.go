package radiobrowser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"
)

type Client struct {
	httpClient *http.Client
	hosts      []string

	mu      sync.Mutex
	current int
}

func NewClient(httpClient *http.Client) (*Client, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	hosts, err := discoverHosts()
	if err != nil {
		return nil, err
	}

	return &Client{
		httpClient: httpClient,
		hosts:      hosts,
	}, nil
}

func (c *Client) Stats(ctx context.Context) (Stats, error) {
	var stats Stats
	if err := c.get(ctx, "/json/stats", nil, &stats); err != nil {
		return nil, err
	}
	return stats, nil
}

func (c *Client) SearchStations(ctx context.Context, name string) ([]Station, error) {
	return c.SearchStationsWithQuery(ctx, StationsQuery{Name: name})
}

func (c *Client) SearchStationsWithQuery(ctx context.Context, options StationsQuery) ([]Station, error) {
	query := url.Values{}
	if options.Name != "" {
		query.Set("name", options.Name)
	}
	if options.Country != "" {
		query.Set("country", options.Country)
	}
	if options.CountryCode != "" {
		query.Set("countrycode", options.CountryCode)
	}
	if options.Tag != "" {
		query.Set("tag", options.Tag)
	}
	if options.Order != "" {
		query.Set("order", options.Order)
	}
	if options.Reverse {
		query.Set("reverse", "true")
	}
	if options.HideBroken {
		query.Set("hidebroken", "true")
	}
	if options.Offset > 0 {
		query.Set("offset", fmt.Sprintf("%d", options.Offset))
	}
	if options.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", options.Limit))
	}

	var stations []Station
	if err := c.get(ctx, "/json/stations/search", query, &stations); err != nil {
		return nil, err
	}
	return stations, nil
}

func (c *Client) Stations(ctx context.Context, options StationsQuery) ([]Station, error) {
	query := url.Values{}
	if options.Order != "" {
		query.Set("order", options.Order)
	}
	if options.Reverse {
		query.Set("reverse", "true")
	}
	if options.HideBroken {
		query.Set("hidebroken", "true")
	}
	if options.Offset > 0 {
		query.Set("offset", fmt.Sprintf("%d", options.Offset))
	}
	if options.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", options.Limit))
	}

	var stations []Station
	if err := c.get(ctx, "/json/stations", query, &stations); err != nil {
		return nil, err
	}
	return stations, nil
}

func (c *Client) TopStations(ctx context.Context, limit int) ([]Station, error) {
	return c.Stations(ctx, StationsQuery{
		Order:      "clickcount",
		Reverse:    true,
		HideBroken: true,
		Limit:      limit,
	})
}

func (c *Client) Countries(ctx context.Context, options CountriesQuery) ([]Country, error) {
	query := url.Values{}
	if options.Order != "" {
		query.Set("order", options.Order)
	}
	if options.Reverse {
		query.Set("reverse", "true")
	}
	if options.HideBroken {
		query.Set("hidebroken", "true")
	}
	if options.Offset > 0 {
		query.Set("offset", fmt.Sprintf("%d", options.Offset))
	}
	if options.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", options.Limit))
	}

	var countries []Country
	if err := c.get(ctx, "/json/countries", query, &countries); err != nil {
		return nil, err
	}
	return countries, nil
}

func (c *Client) get(
	ctx context.Context,
	requestPath string,
	query url.Values,
	result any,
) error {
	var lastErr error

	attempts := max(len(c.hosts), 3)
	for attempt := range attempts {
		if err := ctx.Err(); err != nil {
			return err
		}
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(150 * time.Millisecond):
			}
		}

		host := c.nextHost()
		baseURL, err := url.Parse(host)
		if err != nil {
			lastErr = fmt.Errorf("parse RadioBrowser host: %w", err)
			continue
		}
		baseURL.Path = path.Join(baseURL.Path, requestPath)
		baseURL.RawQuery = query.Encode()

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL.String(), nil)
		if err != nil {
			lastErr = fmt.Errorf("create RadioBrowser request: %w", err)
			continue
		}
		request.Header.Set("User-Agent", "radioman/1.0")

		response, err := c.httpClient.Do(request)
		if err != nil {
			c.httpClient.CloseIdleConnections()
			lastErr = fmt.Errorf("request RadioBrowser: %w", err)
			continue
		}

		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			response.Body.Close()
			lastErr = fmt.Errorf("RadioBrowser returned status %s", response.Status)
			continue
		}

		decodeErr := json.NewDecoder(response.Body).Decode(result)
		response.Body.Close()
		if decodeErr != nil {
			lastErr = fmt.Errorf("decode RadioBrowser response: %w", decodeErr)
			continue
		}
		return nil
	}

	if lastErr == nil {
		lastErr = errors.New("no RadioBrowser hosts available")
	}
	return lastErr
}

func (c *Client) nextHost() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	host := c.hosts[c.current]
	c.current = (c.current + 1) % len(c.hosts)
	return host
}
