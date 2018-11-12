package main

type goLuasStatusDirectionModel struct {
	Message           string `json:"message"`
	ForecastsEnabled  string `json:"forecastsEnabled"`
	OperatingNormally string `json:"operatingNormally"`
}

type goLuasStatusModel struct {
	Inbound  goLuasStatusDirectionModel `json:"inbound"`
	Outbound goLuasStatusDirectionModel `json:"outbound"`
}

type goLuasTramModel struct {
	Direction   string `json:"direction"`
	DueMinutes  string `json:"dueMinutes"`
	Destination string `json:"destination"`
}

type goLuasForecastModel struct {
	Message string            `json:"message"`
	Status  goLuasStatusModel `json:"status"`
	Trams   []goLuasTramModel `json:"trams"`
}

type rpaTramModel struct {
	DueMins     string `xml:"dueMins,attr" json:"dueMins"`
	Destination string `xml:"destination,attr" json:"destination"`
}

type rpaDirectionModel struct {
	Name              string         `xml:"name,attr" json:"name"`
	StatusMessage     string         `xml:"statusMessage,attr" json:"statusMessage"`
	ForecastsEnabled  string         `xml:"forecastsEnabled,attr" json:"forecastsEnabled"`
	OperatingNormally string         `xml:"operatingNormally,attr" json:"operatingNormally"`
	Trams             []rpaTramModel `xml:"tram" json:"tram"`
}

type rpaForecastModel struct {
	Created    string              `xml:"created,attr" json:"created"`
	Stop       string              `xml:"stop,attr" json:"stop"`
	StopAbv    string              `xml:"stopAbv,attr" json:"stopAbv"`
	Message    string              `xml:"message" json:"message"`
	Directions []rpaDirectionModel `xml:"direction" json:"direction"`
}

type stopModel struct {
	Car              int8              `json:"car"`
	Coordinates      map[string]string `json:"coordinates"`
	Cycle            int8              `json:"cycle"`
	DisplayIrishName string            `json:"displayIrishName"`
	DisplayName      string            `json:"displayName"`
	Line             string            `json:"line"`
	ShortName        string            `json:"shortName"`
}

type rpaFareCalcResultModel struct {
	Peak           string `xml:"peak,attr" json:"peak"`
	Offpeak        string `xml:"offpeak,attr" json:"offpeak"`
	ZonesTravelled string `xml:"zonesTravelled,attr" json:"zonesTravelled"`
}

type rpaFareCalcModel struct {
	Created string                 `xml:"created,attr" json:"created"`
	Result  rpaFareCalcResultModel `xml:"result" json:"result"`
}
