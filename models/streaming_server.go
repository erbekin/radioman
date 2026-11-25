package models

// StreamingServer describes a server discovered while checking a stream.
type StreamingServer struct {
	UUID      string  `json:"uuid"`
	URL       string  `json:"url"`
	StatusURL *string `json:"statusurl,omitempty"`
	Error     *string `json:"error,omitempty"`
	Admin     string  `json:"admin"`
	Location  string  `json:"location"`
	Software  string  `json:"software"`
}
