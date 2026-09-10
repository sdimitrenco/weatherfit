package openmeteo

import (
	"errors"
	"fmt"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/port"
)

func toDomain(parsed response, request port.ForecastRequest) (domain.Forecast, error) {
	timezone := request.Timezone
	if len(parsed.Hourly.Time) == 0 {
		return domain.Forecast{}, errors.New("openmeteo: в ответе нет почасовых данных")
	}

	hours := make([]domain.HourPoint, 0, len(parsed.Hourly.Time))
	for i, raw := range parsed.Hourly.Time {
		moment, err := time.ParseInLocation(localTimeForm, raw, timezone)
		if err != nil {
			return domain.Forecast{}, fmt.Errorf("openmeteo: не удалось разобрать время %q: %w", raw, err)
		}
		hours = append(hours, domain.HourPoint{
			Time:                     moment,
			TemperatureC:             optFloat(parsed.Hourly.Temperature2m, i),
			ApparentTemperatureC:     optFloat(parsed.Hourly.ApparentTemperature, i),
			PrecipitationProbability: optInt(parsed.Hourly.PrecipitationProbability, i),
			PrecipitationMM:          optFloat(parsed.Hourly.Precipitation, i),
			RainMM:                   optFloat(parsed.Hourly.Rain, i),
			ShowersMM:                optFloat(parsed.Hourly.Showers, i),
			SnowfallCM:               optFloat(parsed.Hourly.Snowfall, i),
			WeatherCode:              optInt(parsed.Hourly.WeatherCode, i),
			WindSpeedMS:              optFloat(parsed.Hourly.WindSpeed10m, i),
			WindDirectionDeg:         optInt(parsed.Hourly.WindDirection10m, i),
			WindGustsMS:              optFloat(parsed.Hourly.WindGusts10m, i),
			UVIndex:                  optFloat(parsed.Hourly.UVIndex, i),
			IsDay:                    optBool(parsed.Hourly.IsDay, i),
		})
	}

	days := make([]domain.DaySummary, 0, len(parsed.Daily.Time))
	for i, raw := range parsed.Daily.Time {
		date, err := time.ParseInLocation(time.DateOnly, raw, timezone)
		if err != nil {
			return domain.Forecast{}, fmt.Errorf("openmeteo: не удалось разобрать дату %q: %w", raw, err)
		}
		days = append(days, domain.DaySummary{
			Date:                        date,
			TemperatureMaxC:             optFloat(parsed.Daily.Temperature2mMax, i),
			TemperatureMinC:             optFloat(parsed.Daily.Temperature2mMin, i),
			ApparentTemperatureMaxC:     optFloat(parsed.Daily.ApparentTemperatureMax, i),
			ApparentTemperatureMinC:     optFloat(parsed.Daily.ApparentTemperatureMin, i),
			PrecipitationSumMM:          optFloat(parsed.Daily.PrecipitationSum, i),
			PrecipitationProbabilityMax: optInt(parsed.Daily.PrecipitationProbabilityMax, i),
			WindSpeedMaxMS:              optFloat(parsed.Daily.WindSpeed10mMax, i),
			WindGustsMaxMS:              optFloat(parsed.Daily.WindGusts10mMax, i),
			WindDirectionDominantDeg:    optInt(parsed.Daily.WindDirection10mDominant, i),
			UVIndexMax:                  optFloat(parsed.Daily.UVIndexMax, i),
			Sunrise:                     optMoment(parsed.Daily.Sunrise, i, timezone),
			Sunset:                      optMoment(parsed.Daily.Sunset, i, timezone),
		})
	}

	return domain.Forecast{
		Location: request.Place,
		Timezone: timezone,
		Current:  currentToDomain(parsed.Current, timezone),
		Days:     days,
		Hours:    hours,
	}, nil
}

func currentToDomain(raw *current, timezone *time.Location) domain.Opt[domain.CurrentPoint] {
	if raw == nil {
		return domain.None[domain.CurrentPoint]()
	}
	moment, err := time.ParseInLocation(localTimeForm, raw.Time, timezone)
	if err != nil {
		return domain.None[domain.CurrentPoint]()
	}
	return domain.Some(domain.CurrentPoint{
		Time:                 moment,
		TemperatureC:         optPointer(raw.Temperature2m),
		ApparentTemperatureC: optPointer(raw.ApparentTemperature),
		PrecipitationMM:      optPointer(raw.Precipitation),
		WeatherCode:          optPointer(raw.WeatherCode),
		WindSpeedMS:          optPointer(raw.WindSpeed10m),
		WindDirectionDeg:     optPointer(raw.WindDirection10m),
		WindGustsMS:          optPointer(raw.WindGusts10m),
		RelativeHumidity:     optPointer(raw.RelativeHumidity2m),
		IsDay:                optDayFlag(raw.IsDay),
	})
}

func optMoment(values []*string, i int, timezone *time.Location) domain.Opt[time.Time] {
	if i >= len(values) || values[i] == nil {
		return domain.None[time.Time]()
	}
	moment, err := time.ParseInLocation(localTimeForm, *values[i], timezone)
	if err != nil {
		return domain.None[time.Time]()
	}
	return domain.Some(moment)
}

func optPointer[T any](value *T) domain.Opt[T] {
	if value == nil {
		return domain.None[T]()
	}
	return domain.Some(*value)
}

func optDayFlag(value *int) domain.Opt[bool] {
	if value == nil {
		return domain.None[bool]()
	}
	return domain.Some(*value == 1)
}

func optFloat(values []*float64, i int) domain.Opt[float64] {
	if i >= len(values) || values[i] == nil {
		return domain.None[float64]()
	}
	return domain.Some(*values[i])
}

func optInt(values []*int, i int) domain.Opt[int] {
	if i >= len(values) || values[i] == nil {
		return domain.None[int]()
	}
	return domain.Some(*values[i])
}

func optBool(values []*int, i int) domain.Opt[bool] {
	value, ok := optInt(values, i).Get()
	if !ok {
		return domain.None[bool]()
	}
	return domain.Some(value == 1)
}
