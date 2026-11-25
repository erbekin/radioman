package models

// StationCheck describes one RadioBrowser stream check.
type StationCheck struct {
	CheckUUID                 string   `json:"checkuuid"`
	StationUUID               string   `json:"stationuuid"`
	Source                    string   `json:"source"`
	Codec                     string   `json:"codec"`
	Bitrate                   int      `json:"bitrate"`
	HLS                       int      `json:"hls"`
	OK                        int      `json:"ok"`
	TimestampISO8601          string   `json:"timestamp_iso8601"`
	Timestamp                 string   `json:"timestamp"`
	URLCache                  string   `json:"urlcache"`
	MetaInfoOverridesDatabase int      `json:"metainfo_overrides_database"`
	Public                    *int     `json:"public,omitempty"`
	Name                      *string  `json:"name,omitempty"`
	Description               *string  `json:"description,omitempty"`
	Tags                      *string  `json:"tags,omitempty"`
	CountryCode               *string  `json:"countrycode,omitempty"`
	CountrySubdivisionCode    *string  `json:"countrysubdivisioncode,omitempty"`
	Homepage                  *string  `json:"homepage,omitempty"`
	Favicon                   *string  `json:"favicon,omitempty"`
	LoadBalancer              *string  `json:"loadbalancer,omitempty"`
	ServerSoftware            *string  `json:"server_software,omitempty"`
	Sampling                  *uint    `json:"sampling,omitempty"`
	TimingMS                  uint     `json:"timing_ms"`
	LanguageCodes             *string  `json:"languagecodes,omitempty"`
	SSLError                  uint     `json:"ssl_error"`
	GeoLat                    *float64 `json:"geo_lat,omitempty"`
	GeoLong                   *float64 `json:"geo_long,omitempty"`
}
