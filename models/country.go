package models

// Country is a country returned by the RadioBrowser countries endpoint.
type Country struct {
	Name         string `json:"name"`
	ISO3166_1    string `json:"iso_3166_1"`
	StationCount string `json:"stationcount"`
}

// CountriesQuery contains optional filters for the countries endpoint.
type CountriesQuery struct {
	Order      string
	Reverse    bool
	HideBroken bool
	Offset     int
	Limit      int
}
