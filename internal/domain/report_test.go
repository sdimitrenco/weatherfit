package domain

import (
	"strings"
	"testing"
	"time"
)

func sunnyHours() []HourPoint {
	hours := make([]HourPoint, 0, 4)
	for hour := 7; hour <= 10; hour++ {
		hours = append(hours, HourPoint{
			Time:                     hourAt(hour),
			TemperatureC:             Some(float64(13 + hour - 7)),
			ApparentTemperatureC:     Some(float64(11 + hour - 7)),
			PrecipitationProbability: Some(0),
			PrecipitationMM:          Some(0.0),
			SnowfallCM:               Some(0.0),
			WeatherCode:              Some(0),
			WindSpeedMS:              Some(2.0),
			WindDirectionDeg:         Some(270),
			WindGustsMS:              Some(4.0),
			UVIndex:                  Some(float64(hour - 6)),
			IsDay:                    Some(true),
		})
	}
	return hours
}

func TestAnalyzeAggregates(t *testing.T) {
	location := Location{Name: "Дрезден", Latitude: 51.05, Longitude: 13.74}
	report := Analyze(location, hourAt(0), DaySummary{}, sunnyHours())

	if len(report.Hours) != 4 {
		t.Fatalf("часов = %d, want 4", len(report.Hours))
	}
	if minimum, _ := report.TemperatureMinC.Get(); minimum != 13 {
		t.Errorf("минимум температуры = %v, want 13", minimum)
	}
	if maximum, _ := report.TemperatureMaxC.Get(); maximum != 16 {
		t.Errorf("максимум температуры = %v, want 16", maximum)
	}
	if minimum, _ := report.ApparentMinC.Get(); minimum != 11 {
		t.Errorf("минимум ощущаемой = %v, want 11", minimum)
	}
	if maximum, _ := report.ApparentMaxC.Get(); maximum != 14 {
		t.Errorf("максимум ощущаемой = %v, want 14", maximum)
	}
	if uv, _ := report.UVIndexMax.Get(); uv != 4 {
		t.Errorf("максимум УФ = %v, want 4", uv)
	}
	if report.Precipitation.Verdict != RainNotNeeded {
		t.Errorf("вердикт = %v, want «не нужно»", report.Precipitation.Verdict)
	}
	if report.Hours[0].Condition.Icon != "☀️" {
		t.Errorf("иконка первого часа = %q", report.Hours[0].Condition.Icon)
	}
	if point, ok := report.Hours[0].Compass.Get(); !ok || point.Rose != RoseWest {
		t.Errorf("румб первого часа = %v", point)
	}
}

func TestAnalyzeFallsBackToDailySummary(t *testing.T) {
	day := DaySummary{
		Date:                    hourAt(0),
		TemperatureMinC:         Some(8.0),
		TemperatureMaxC:         Some(19.0),
		ApparentTemperatureMinC: Some(6.0),
		ApparentTemperatureMaxC: Some(17.0),
		UVIndexMax:              Some(4.0),
	}
	hours := []HourPoint{{Time: hourAt(7)}}

	report := Analyze(Location{}, hourAt(0), day, hours)
	if minimum, _ := report.TemperatureMinC.Get(); minimum != 8 {
		t.Errorf("минимум = %v, want 8 из daily", minimum)
	}
	if maximum, _ := report.ApparentMaxC.Get(); maximum != 17 {
		t.Errorf("максимум ощущаемой = %v, want 17 из daily", maximum)
	}
	if uv, _ := report.UVIndexMax.Get(); uv != 4 {
		t.Errorf("УФ = %v, want 4 из daily", uv)
	}
}

func TestAnalyzeCollectsUnknownCodes(t *testing.T) {
	hours := []HourPoint{
		{Time: hourAt(7), WeatherCode: Some(42)},
		{Time: hourAt(8), WeatherCode: Some(42)},
		{Time: hourAt(9), WeatherCode: Some(3)},
		{Time: hourAt(10), WeatherCode: Some(199)},
		{Time: hourAt(11), WeatherCode: None[int]()},
	}

	report := Analyze(Location{}, hourAt(0), DaySummary{}, hours)
	if len(report.UnknownCodes) != 2 {
		t.Fatalf("неизвестных кодов = %v, want two unique values", report.UnknownCodes)
	}
	if report.UnknownCodes[0] != 42 || report.UnknownCodes[1] != 199 {
		t.Errorf("неизвестные коды = %v", report.UnknownCodes)
	}
}

func TestHeadlines(t *testing.T) {
	tests := []struct {
		name     string
		hours    []HourPoint
		rainKind HeadlineKind
		band     BandID
	}{
		{
			name:     "солнечный день",
			hours:    sunnyHours(),
			rainKind: HeadlineDry,
			band:     BandSweaterCoat,
		},
		{
			name: "дождь с сильным ветром",
			hours: []HourPoint{
				{
					Time: hourAt(14), ApparentTemperatureC: Some(12.0),
					PrecipitationProbability: Some(80), PrecipitationMM: Some(1.2), RainMM: Some(1.2),
					WeatherCode: Some(63), WindSpeedMS: Some(9.0), WindDirectionDeg: Some(225),
					WindGustsMS: Some(15.0), IsDay: Some(true),
				},
				{
					Time: hourAt(15), ApparentTemperatureC: Some(12.0),
					PrecipitationProbability: Some(90), PrecipitationMM: Some(2.0), RainMM: Some(2.0),
					WeatherCode: Some(63), WindSpeedMS: Some(9.0), WindDirectionDeg: Some(225),
					WindGustsMS: Some(15.0), IsDay: Some(true),
				},
			},
			rainKind: HeadlineRainCoat,
			band:     BandHoodieJacket,
		},
		{
			name: "дождь без ветра",
			hours: []HourPoint{
				{
					Time: hourAt(14), ApparentTemperatureC: Some(16.0),
					PrecipitationProbability: Some(80), PrecipitationMM: Some(1.2), RainMM: Some(1.2),
					WeatherCode: Some(63), WindSpeedMS: Some(2.0), WindGustsMS: Some(4.0),
				},
			},
			rainKind: HeadlineRainUmbrella,
			band:     BandHoodieJacket,
		},
		{
			name: "морось на всякий случай",
			hours: []HourPoint{
				{
					Time: hourAt(9), ApparentTemperatureC: Some(19.0),
					PrecipitationProbability: Some(40), PrecipitationMM: Some(0.2), RainMM: Some(0.2),
					WeatherCode: Some(51),
				},
			},
			rainKind: HeadlineRainMaybe,
			band:     BandTShirtPants,
		},
		{
			name: "снег",
			hours: []HourPoint{
				{
					Time: hourAt(9), ApparentTemperatureC: Some(-3.0),
					PrecipitationProbability: Some(80), PrecipitationMM: Some(0.5),
					SnowfallCM: Some(0.5), WeatherCode: Some(73),
				},
			},
			rainKind: HeadlineSnow,
			band:     BandWinterCoat,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := Analyze(Location{}, hourAt(0), DaySummary{}, tc.hours)
			headlines := report.Headlines()
			if len(headlines) != 2 {
				t.Fatalf("строк вердикта = %d, want 2", len(headlines))
			}
			if headlines[0].Kind != tc.rainKind {
				t.Errorf("вердикт по дождю = %q, want %q", headlines[0].Kind, tc.rainKind)
			}
			if headlines[1].Kind != HeadlineOutfit {
				t.Errorf("вторая строка = %q, want вердикт по одежде", headlines[1].Kind)
			}
			if headlines[1].Band != tc.band {
				t.Errorf("комплект = %q, want %q", headlines[1].Band, tc.band)
			}
			for _, headline := range headlines {
				if headline.Icon == "" {
					t.Error("иконка вердикта пуста")
				}
			}
		})
	}
}

func TestHeadlinesCarryRainWindows(t *testing.T) {
	hours := []HourPoint{
		{Time: hourAt(13), PrecipitationProbability: Some(10), PrecipitationMM: Some(0.0)},
		{Time: hourAt(14), PrecipitationProbability: Some(80), PrecipitationMM: Some(1.2), RainMM: Some(1.2)},
		{Time: hourAt(15), PrecipitationProbability: Some(80), PrecipitationMM: Some(1.2), RainMM: Some(1.2)},
	}

	report := Analyze(Location{}, hourAt(0), DaySummary{}, hours)
	windows := report.Headlines()[0].Windows
	if len(windows) != 1 {
		t.Fatalf("окон = %d, want 1", len(windows))
	}
	from, to := windows[0].Hours()
	if from != 14 || to != 15 {
		t.Errorf("окно = %d–%d, want 14–15", from, to)
	}
}

func TestForecastSelectors(t *testing.T) {
	forecast := Forecast{
		Timezone: berlinTZ,
		Days: []DaySummary{
			{Date: time.Date(2026, 9, 10, 0, 0, 0, 0, berlinTZ), TemperatureMaxC: Some(22.0)},
			{Date: time.Date(2026, 9, 11, 0, 0, 0, 0, berlinTZ), TemperatureMaxC: Some(18.0)},
		},
		Hours: []HourPoint{
			{Time: hourAt(22)},
			{Time: hourAt(23)},
			{Time: time.Date(2026, 9, 11, 0, 0, 0, 0, berlinTZ)},
			{Time: time.Date(2026, 9, 11, 7, 0, 0, 0, berlinTZ)},
		},
	}

	today, ok := forecast.Day(hourAt(15))
	if !ok {
		t.Fatal("день не найден")
	}
	if maximum, _ := today.TemperatureMaxC.Get(); maximum != 22 {
		t.Errorf("максимум сегодня = %v, want 22", maximum)
	}

	if _, found := forecast.Day(time.Date(2026, 9, 12, 0, 0, 0, 0, berlinTZ)); found {
		t.Error("для отсутствующей даты день не должен находиться")
	}

	if hours := forecast.HoursOfDay(hourAt(0)); len(hours) != 2 {
		t.Errorf("часов сегодня = %d, want 2", len(hours))
	}
	if hours := forecast.HoursOfDay(time.Date(2026, 9, 11, 12, 0, 0, 0, berlinTZ)); len(hours) != 2 {
		t.Errorf("часов завтра = %d, want 2", len(hours))
	}

	inRange := forecast.HoursInRange(hourAt(23), time.Date(2026, 9, 11, 7, 0, 0, 0, berlinTZ))
	if len(inRange) != 2 {
		t.Errorf("часов в интервале = %d, want 2", len(inRange))
	}
	if !inRange[0].Time.Equal(hourAt(23)) {
		t.Errorf("первый час интервала = %v", inRange[0].Time)
	}
}

func TestOptHelpers(t *testing.T) {
	value := Some(5)
	if got, ok := value.Get(); !ok || got != 5 {
		t.Errorf("Get = %v, %v", got, ok)
	}
	if value.Or(9) != 5 {
		t.Error("Or должен вернуть значение")
	}

	empty := None[int]()
	if empty.Valid() {
		t.Error("None не должен быть валидным")
	}
	if empty.Or(9) != 9 {
		t.Error("Or должен вернуть fallback")
	}

	mapped := Map(Some(2), func(v int) string { return strings.Repeat("x", v) })
	if got, _ := mapped.Get(); got != "xx" {
		t.Errorf("Map = %q", got)
	}
	if Map(None[int](), func(v int) int { return v }).Valid() {
		t.Error("Map по пустому значению должен остаться пустым")
	}
}
