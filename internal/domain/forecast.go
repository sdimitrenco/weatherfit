package domain

import "time"

// Location is the point a forecast is requested for.
type Location struct {
	Name      string
	Latitude  float64
	Longitude float64
}

// HourPoint is the forecast for one hour. Wind is always in m/s, snow in cm
// and precipitation in mm, whatever units the API response used.
type HourPoint struct {
	Time                     time.Time
	TemperatureC             Opt[float64]
	ApparentTemperatureC     Opt[float64]
	PrecipitationProbability Opt[int]
	PrecipitationMM          Opt[float64]
	RainMM                   Opt[float64]
	ShowersMM                Opt[float64]
	SnowfallCM               Opt[float64]
	WeatherCode              Opt[int]
	WindSpeedMS              Opt[float64]
	WindDirectionDeg         Opt[int]
	WindGustsMS              Opt[float64]
	UVIndex                  Opt[float64]
	IsDay                    Opt[bool]
}

// CurrentPoint is the observed weather at request time.
type CurrentPoint struct {
	Time                 time.Time
	TemperatureC         Opt[float64]
	ApparentTemperatureC Opt[float64]
	PrecipitationMM      Opt[float64]
	WeatherCode          Opt[int]
	WindSpeedMS          Opt[float64]
	WindDirectionDeg     Opt[int]
	WindGustsMS          Opt[float64]
	RelativeHumidity     Opt[int]
	IsDay                Opt[bool]
}

// DaySummary is the daily block of the response for one calendar day.
type DaySummary struct {
	Date                        time.Time
	TemperatureMaxC             Opt[float64]
	TemperatureMinC             Opt[float64]
	ApparentTemperatureMaxC     Opt[float64]
	ApparentTemperatureMinC     Opt[float64]
	PrecipitationSumMM          Opt[float64]
	PrecipitationProbabilityMax Opt[int]
	WindSpeedMaxMS              Opt[float64]
	WindGustsMaxMS              Opt[float64]
	WindDirectionDominantDeg    Opt[int]
	UVIndexMax                  Opt[float64]
	Sunrise                     Opt[time.Time]
	Sunset                      Opt[time.Time]
}

// Forecast holds several days in the local timezone of the point.
type Forecast struct {
	Location Location
	Timezone *time.Location
	Current  Opt[CurrentPoint]
	Days     []DaySummary
	Hours    []HourPoint
}

// Day returns the summary for a calendar date in the forecast timezone.
func (f Forecast) Day(date time.Time) (DaySummary, bool) {
	target := date.In(f.timezone()).Format(time.DateOnly)
	for _, day := range f.Days {
		if day.Date.Format(time.DateOnly) == target {
			return day, true
		}
	}
	return DaySummary{}, false
}

// HoursOfDay returns the hours belonging to a calendar date.
func (f Forecast) HoursOfDay(date time.Time) []HourPoint {
	target := date.In(f.timezone()).Format(time.DateOnly)
	hours := make([]HourPoint, 0, 24)
	for _, hour := range f.Hours {
		if hour.Time.Format(time.DateOnly) == target {
			hours = append(hours, hour)
		}
	}
	return hours
}

// HoursInRange returns the hours in the half-open interval [from, to).
func (f Forecast) HoursInRange(from, to time.Time) []HourPoint {
	hours := make([]HourPoint, 0, 24)
	for _, hour := range f.Hours {
		if !hour.Time.Before(from) && hour.Time.Before(to) {
			hours = append(hours, hour)
		}
	}
	return hours
}

func (f Forecast) timezone() *time.Location {
	if f.Timezone == nil {
		return time.UTC
	}
	return f.Timezone
}
