package models

// StationCheckStep represents one step in the stream-checking process.
// Steps form a tree when redirects or playlists create additional steps.
type StationCheckStep struct {
	StepUUID        string  `json:"stepuuid"`
	ParentStepUUID  *string `json:"parent_stepuuid,omitempty"`
	CheckUUID       string  `json:"checkuuid"`
	StationUUID     string  `json:"stationuuid"`
	URL             string  `json:"url"`
	URLType         *string `json:"urltype,omitempty"`
	Error           *string `json:"error,omitempty"`
	CreationISO8601 string  `json:"creation_iso8601"`
}
