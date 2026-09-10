package domain

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

var berlinTZ = time.FixedZone("CEST", 2*60*60)

func hourAt(hour int) time.Time {
	return time.Date(2026, 9, 10, hour, 0, 0, 0, berlinTZ)
}

type hourSpec struct {
	hour        int
	probability int
	amountMM    float64
	code        int
	snowCM      float64
}

func buildHours(specs []hourSpec) []HourPoint {
	hours := make([]HourPoint, 0, len(specs))
	for _, spec := range specs {
		point := HourPoint{
			Time:                     hourAt(spec.hour),
			PrecipitationProbability: Some(spec.probability),
			PrecipitationMM:          Some(spec.amountMM),
			SnowfallCM:               Some(spec.snowCM),
		}
		if spec.code != 0 {
			point.WeatherCode = Some(spec.code)
		}
		hours = append(hours, point)
	}
	return hours
}

func TestAnalyzePrecipitationVerdicts(t *testing.T) {
	tests := []struct {
		name  string
		specs []hourSpec
		want  RainVerdict
		heavy bool
	}{
		{
			name:  "сухо, нулевая вероятность",
			specs: []hourSpec{{hour: 7}, {hour: 8}},
			want:  RainNotNeeded,
		},
		{
			name:  "высокая вероятность без осадков",
			specs: []hourSpec{{hour: 7, probability: 80, amountMM: 0.05}},
			want:  RainNotNeeded,
		},
		{
			name:  "низкая вероятность с осадками",
			specs: []hourSpec{{hour: 7, probability: 29, amountMM: 1.0}},
			want:  RainNotNeeded,
		},
		{
			name:  "граница 30% и 0.1 мм",
			specs: []hourSpec{{hour: 7, probability: 30, amountMM: 0.1}},
			want:  RainJustInCase,
		},
		{
			name:  "59% и заметные осадки",
			specs: []hourSpec{{hour: 7, probability: 59, amountMM: 1.0}},
			want:  RainJustInCase,
		},
		{
			name:  "60% но морось меньше 0.3 мм",
			specs: []hourSpec{{hour: 7, probability: 60, amountMM: 0.2}},
			want:  RainJustInCase,
		},
		{
			name:  "граница 60% и 0.3 мм",
			specs: []hourSpec{{hour: 7, probability: 60, amountMM: 0.3}},
			want:  RainRequired,
		},
		{
			name: "сумма 2 мм при вероятности 50%",
			specs: []hourSpec{
				{hour: 7, probability: 50, amountMM: 0.7},
				{hour: 8, probability: 50, amountMM: 0.7},
				{hour: 9, probability: 50, amountMM: 0.6},
			},
			want: RainRequired,
		},
		{
			name: "сумма 2 мм но вероятность 49%",
			specs: []hourSpec{
				{hour: 7, probability: 49, amountMM: 1.0},
				{hour: 8, probability: 49, amountMM: 1.0},
			},
			want: RainJustInCase,
		},
		{
			name:  "ливень 2.5 мм за час",
			specs: []hourSpec{{hour: 14, probability: 90, amountMM: 2.5}},
			want:  RainRequired,
			heavy: true,
		},
		{
			name: "сумма 10 мм",
			specs: []hourSpec{
				{hour: 14, probability: 70, amountMM: 2.4},
				{hour: 15, probability: 70, amountMM: 2.4},
				{hour: 16, probability: 70, amountMM: 2.4},
				{hour: 17, probability: 70, amountMM: 2.4},
				{hour: 18, probability: 70, amountMM: 0.4},
			},
			want:  RainRequired,
			heavy: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			analysis := AnalyzePrecipitation(buildHours(tc.specs))
			if analysis.Verdict != tc.want {
				t.Errorf("вердикт = %v, ожидалось %v", analysis.Verdict, tc.want)
			}
			if analysis.Heavy != tc.heavy {
				t.Errorf("сильный дождь = %v, ожидалось %v", analysis.Heavy, tc.heavy)
			}
		})
	}
}

func windowHours(windows []RainWindow) string {
	parts := make([]string, 0, len(windows))
	for _, window := range windows {
		from, to := window.Hours()
		if from == to {
			parts = append(parts, fmt.Sprintf("%02d", from))
			continue
		}
		parts = append(parts, fmt.Sprintf("%02d-%02d", from, to))
	}
	return strings.Join(parts, ",")
}

func TestAnalyzePrecipitationWindows(t *testing.T) {
	tests := []struct {
		name  string
		specs []hourSpec
		want  string
	}{
		{
			name:  "без осадков окон нет",
			specs: []hourSpec{{hour: 7}, {hour: 8}},
			want:  "",
		},
		{
			name:  "один час",
			specs: []hourSpec{{hour: 7}, {hour: 14, probability: 80, amountMM: 1.0}, {hour: 15}},
			want:  "14",
		},
		{
			name: "склеенный интервал",
			specs: []hourSpec{
				{hour: 13},
				{hour: 14, probability: 80, amountMM: 1.2},
				{hour: 15, probability: 90, amountMM: 2.0},
				{hour: 16, probability: 70, amountMM: 0.5},
				{hour: 17, probability: 60, amountMM: 0.3},
				{hour: 18},
			},
			want: "14-17",
		},
		{
			name: "два интервала",
			specs: []hourSpec{
				{hour: 10, probability: 60, amountMM: 0.4},
				{hour: 11, probability: 60, amountMM: 0.4},
				{hour: 12},
				{hour: 13},
				{hour: 16, probability: 80, amountMM: 1.0},
				{hour: 17, probability: 80, amountMM: 1.0},
			},
			want: "10-11,16-17",
		},
		{
			name: "три интервала",
			specs: []hourSpec{
				{hour: 8, probability: 60, amountMM: 0.4},
				{hour: 10, probability: 60, amountMM: 0.4},
				{hour: 12, probability: 60, amountMM: 0.4},
			},
			want: "08,10,12",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			analysis := AnalyzePrecipitation(buildHours(tc.specs))
			if got := windowHours(analysis.Windows); got != tc.want {
				t.Errorf("окна = %q, ожидалось %q", got, tc.want)
			}
		})
	}
}

func TestAnalyzePrecipitationGapInDataBreaksWindow(t *testing.T) {
	hours := []HourPoint{
		{Time: hourAt(14), PrecipitationProbability: Some(80), PrecipitationMM: Some(1.0)},
		{Time: hourAt(15), PrecipitationProbability: None[int](), PrecipitationMM: Some(1.0)},
		{Time: hourAt(16), PrecipitationProbability: Some(80), PrecipitationMM: Some(1.0)},
	}
	analysis := AnalyzePrecipitation(hours)
	if got := windowHours(analysis.Windows); got != "14,16" {
		t.Errorf("окна = %q, ожидалось «14,16»", got)
	}
}

func TestAnalyzePrecipitationHazards(t *testing.T) {
	tests := []struct {
		name    string
		specs   []hourSpec
		thunder bool
		freeze  bool
		snow    bool
	}{
		{name: "гроза", specs: []hourSpec{{hour: 15, probability: 80, amountMM: 3.0, code: 95}}, thunder: true},
		{name: "гроза с градом", specs: []hourSpec{{hour: 15, probability: 80, amountMM: 3.0, code: 99}}, thunder: true},
		{name: "ледяная морось", specs: []hourSpec{{hour: 8, probability: 60, amountMM: 0.4, code: 56}}, freeze: true},
		{name: "ледяной дождь", specs: []hourSpec{{hour: 8, probability: 60, amountMM: 0.4, code: 67}}, freeze: true},
		{name: "снег по коду", specs: []hourSpec{{hour: 8, probability: 60, amountMM: 0.4, code: 73}}, snow: true},
		{name: "снег по snowfall", specs: []hourSpec{{hour: 8, probability: 60, amountMM: 0.4, snowCM: 0.4}}, snow: true},
		{name: "снегопад 86", specs: []hourSpec{{hour: 8, probability: 60, amountMM: 0.4, code: 86}}, snow: true},
		{name: "обычный дождь", specs: []hourSpec{{hour: 8, probability: 60, amountMM: 0.4, code: 63}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			analysis := AnalyzePrecipitation(buildHours(tc.specs))
			if analysis.Thunderstorm != tc.thunder {
				t.Errorf("гроза = %v, ожидалось %v", analysis.Thunderstorm, tc.thunder)
			}
			if analysis.Freezing != tc.freeze {
				t.Errorf("гололёд = %v, ожидалось %v", analysis.Freezing, tc.freeze)
			}
			if analysis.Snow != tc.snow {
				t.Errorf("снег = %v, ожидалось %v", analysis.Snow, tc.snow)
			}
		})
	}
}

func TestAnalyzePrecipitationTotals(t *testing.T) {
	analysis := AnalyzePrecipitation(buildHours([]hourSpec{
		{hour: 7, probability: 10, amountMM: 0.0},
		{hour: 8, probability: 40, amountMM: 0.4},
		{hour: 9, probability: 90, amountMM: 1.6},
	}))

	if analysis.TotalMM < 1.99 || analysis.TotalMM > 2.01 {
		t.Errorf("сумма = %v, ожидалось 2.0", analysis.TotalMM)
	}
	if analysis.MaxHourlyMM != 1.6 {
		t.Errorf("максимум за час = %v, ожидалось 1.6", analysis.MaxHourlyMM)
	}
	if probability, ok := analysis.MaxProbability.Get(); !ok || probability != 90 {
		t.Errorf("максимальная вероятность = %v, %v, ожидалось 90", probability, ok)
	}
}

func TestAnalyzePrecipitationMissingValues(t *testing.T) {
	analysis := AnalyzePrecipitation([]HourPoint{
		{Time: hourAt(7)},
		{Time: hourAt(8), PrecipitationProbability: None[int](), PrecipitationMM: None[float64]()},
	})
	if analysis.Verdict != RainNotNeeded {
		t.Errorf("вердикт = %v, ожидалось «не нужно»", analysis.Verdict)
	}
	if analysis.MaxProbability.Valid() {
		t.Error("максимальная вероятность должна быть пустой")
	}
	if len(analysis.Windows) != 0 {
		t.Errorf("окон = %d, ожидалось 0", len(analysis.Windows))
	}
}

func TestUmbrellaUseless(t *testing.T) {
	tests := []struct {
		name     string
		verdict  RainVerdict
		wind     WindSummary
		expected bool
	}{
		{
			name:     "обязательно и сильный ветер",
			verdict:  RainRequired,
			wind:     WindSummary{Level: WindStrong},
			expected: true,
		},
		{
			name:     "обязательно и порывы",
			verdict:  RainRequired,
			wind:     WindSummary{Level: WindLight, GustWarning: true},
			expected: true,
		},
		{
			name:     "обязательно и слабый ветер",
			verdict:  RainRequired,
			wind:     WindSummary{Level: WindLight},
			expected: false,
		},
		{
			name:     "на всякий случай и сильный ветер",
			verdict:  RainJustInCase,
			wind:     WindSummary{Level: WindStrong},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			analysis := PrecipitationAnalysis{Verdict: tc.verdict}
			if got := analysis.UmbrellaUseless(tc.wind); got != tc.expected {
				t.Errorf("UmbrellaUseless = %v, ожидалось %v", got, tc.expected)
			}
		})
	}
}

func TestSnowDominant(t *testing.T) {
	snowy := AnalyzePrecipitation([]HourPoint{{
		Time:                     hourAt(9),
		PrecipitationProbability: Some(80),
		PrecipitationMM:          Some(0.5),
		RainMM:                   Some(0.0),
		ShowersMM:                Some(0.0),
		SnowfallCM:               Some(0.6),
		WeatherCode:              Some(73),
	}})
	if !snowy.SnowDominant() {
		t.Error("осадки из снега должны считаться снежными")
	}

	sleet := AnalyzePrecipitation([]HourPoint{{
		Time:                     hourAt(9),
		PrecipitationProbability: Some(80),
		PrecipitationMM:          Some(1.5),
		RainMM:                   Some(1.0),
		SnowfallCM:               Some(0.3),
		WeatherCode:              Some(73),
	}})
	if sleet.SnowDominant() {
		t.Error("при жидких осадках день не считается снежным")
	}

	rain := AnalyzePrecipitation(buildHours([]hourSpec{{hour: 14, probability: 80, amountMM: 1.0, code: 63}}))
	if rain.SnowDominant() {
		t.Error("дождь не должен считаться снегом")
	}
}
