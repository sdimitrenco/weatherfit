package domain

import "time"

// Location — точка, для которой запрашивается прогноз.
type Location struct {
	Name      string
	Latitude  float64
	Longitude float64
}

// HourPoint — прогноз на один час. Скорости ветра всегда в м/с, снег в см,
// осадки в мм, независимо от единиц, в которых пришёл ответ API.
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

// CurrentPoint — фактическая погода на момент запроса.
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

// DaySummary — сводка по календарному дню из daily-блока ответа.
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

// Forecast — прогноз на несколько дней в локальной таймзоне точки.
type Forecast struct {
	Location Location
	Timezone *time.Location
	Current  Opt[CurrentPoint]
	Days     []DaySummary
	Hours    []HourPoint
}

// Day возвращает сводку по календарной дате в таймзоне прогноза.
func (f Forecast) Day(date time.Time) (DaySummary, bool) {
	target := date.In(f.timezone()).Format(time.DateOnly)
	for _, day := range f.Days {
		if day.Date.Format(time.DateOnly) == target {
			return day, true
		}
	}
	return DaySummary{}, false
}

// HoursOfDay возвращает часы, попадающие в календарную дату прогноза.
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

// HoursInRange возвращает часы в полуинтервале [from, to).
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
