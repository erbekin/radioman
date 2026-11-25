package models

import (
	"fmt"
	"strings"
)

type Station struct {
	ChangeUUID                string  `json:"changeuuid"`
	StationUUID               string  `json:"stationuuid"`
	Name                      string  `json:"name"`
	URL                       string  `json:"url"`
	URLResolved               string  `json:"url_resolved"`
	Homepage                  string  `json:"homepage"`
	Favicon                   string  `json:"favicon"`
	Tags                      string  `json:"tags"`
	Country                   string  `json:"country"`
	CountryCode               string  `json:"countrycode"`
	State                     string  `json:"state"`
	ISO3166_2                 string  `json:"iso_3166_2"`
	Language                  string  `json:"language"`
	LanguageCodes             string  `json:"languagecodes"`
	Votes                     int     `json:"votes"`
	LastChange                string  `json:"lastchange"`
	LastChangeTimeISO8601     string  `json:"lastchangetime_iso8601"`
	Codec                     string  `json:"codec"`
	Bitrate                   int     `json:"bitrate"`
	HLS                       int     `json:"hls"`
	LastCheckOK               int     `json:"lastcheckok"`
	LastCheck                 string  `json:"lastcheck"`
	LastCheckTimeISO8601      string  `json:"lastchecktime_iso8601"`
	LastCheckOKTime           string  `json:"lastcheckoktime"`
	LastCheckOKTimeISO8601    string  `json:"lastcheckoktime_iso8601"`
	LastLocalCheck            string  `json:"lastlocalcheck"`
	LastLocalCheckTimeISO8601 string  `json:"lastlocalchecktime_iso8601"`
	ClickTimestamp            string  `json:"clicktimestamp"`
	ClickTimestampISO8601     string  `json:"clicktimestamp_iso8601"`
	ClickCount                int     `json:"clickcount"`
	ClickTrend                int     `json:"clicktrend"`
	SSLError                  int     `json:"ssl_error"`
	GeoLat                    float64 `json:"geo_lat"`
	GeoLong                   float64 `json:"geo_long"`
	GeoDistance               float64 `json:"geo_distance"`
	HasExtendedInfo           *bool   `json:"has_extended_info,omitempty"`
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

type Stats map[string]any

// StationsQuery contains optional filters for the stations search endpoint.
type StationsQuery struct {
	Name        string
	Country     string
	CountryCode string
	Tag         string
	Order       string
	Reverse     bool
	HideBroken  bool
	Offset      int
	Limit       int
}
