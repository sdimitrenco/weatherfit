package domain

import (
	"testing"
	"time"
)

func TestCompassForBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		from  int
		rumb  string
		arrow string
	}{
		{name: "север 0", from: 0, rumb: "С", arrow: "↓"},
		{name: "север 360", from: 360, rumb: "С", arrow: "↓"},
		{name: "север 720", from: 720, rumb: "С", arrow: "↓"},
		{name: "граница 22 остаётся севером", from: 22, rumb: "С", arrow: "↓"},
		{name: "граница 23 уже северо-восток", from: 23, rumb: "СВ", arrow: "↙"},
		{name: "северо-восток 45", from: 45, rumb: "СВ", arrow: "↙"},
		{name: "граница 67", from: 67, rumb: "СВ", arrow: "↙"},
		{name: "восток 90", from: 90, rumb: "В", arrow: "←"},
		{name: "юго-восток 135", from: 135, rumb: "ЮВ", arrow: "↖"},
		{name: "юг 180", from: 180, rumb: "Ю", arrow: "↑"},
		{name: "юго-запад 225", from: 225, rumb: "ЮЗ", arrow: "↗"},
		{name: "запад 270", from: 270, rumb: "З", arrow: "→"},
		{name: "северо-запад 315", from: 315, rumb: "СЗ", arrow: "↘"},
		{name: "граница 337 северо-запад", from: 337, rumb: "СЗ", arrow: "↘"},
		{name: "граница 338 север", from: 338, rumb: "С", arrow: "↓"},
		{name: "отрицательный -45 северо-запад", from: -45, rumb: "СЗ", arrow: "↘"},
		{name: "отрицательный -90 запад", from: -90, rumb: "З", arrow: "→"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			point, ok := CompassFor(Some(tc.from)).Get()
			if !ok {
				t.Fatal("румб пуст")
			}
			if point.Rumb != tc.rumb {
				t.Errorf("румб для %d° = %q, ожидалось %q", tc.from, point.Rumb, tc.rumb)
			}
			if point.Arrow != tc.arrow {
				t.Errorf("стрелка для %d° = %q, ожидалось %q", tc.from, point.Arrow, tc.arrow)
			}
		})
	}
}

func TestCompassForOppositeArrows(t *testing.T) {
	for from := 0; from < 360; from++ {
		fromPoint, _ := CompassFor(Some(from)).Get()
		toPoint, _ := CompassFor(Some((from + 180) % 360)).Get()
		if fromPoint.Arrow == toPoint.Arrow {
			t.Fatalf("%d° и %d° дали одну стрелку %q", from, (from+180)%360, fromPoint.Arrow)
		}
	}
}

func TestCompassForMissingDirection(t *testing.T) {
	if CompassFor(None[int]()).Valid() {
		t.Error("для отсутствующего направления румб должен быть пустым")
	}
}

func TestWindLevelForThresholds(t *testing.T) {
	tests := []struct {
		speed float64
		want  WindLevel
		label string
	}{
		{speed: 0, want: WindCalm, label: "штиль"},
		{speed: 1.59, want: WindCalm, label: "штиль"},
		{speed: 1.6, want: WindLight, label: "слабый"},
		{speed: 5.4, want: WindLight, label: "слабый"},
		{speed: 5.49, want: WindLight, label: "слабый"},
		{speed: 5.5, want: WindModerate, label: "умеренный"},
		{speed: 7.9, want: WindModerate, label: "умеренный"},
		{speed: 7.99, want: WindModerate, label: "умеренный"},
		{speed: 8.0, want: WindStrong, label: "сильный"},
		{speed: 13.8, want: WindStrong, label: "сильный"},
		{speed: 13.89, want: WindStrong, label: "сильный"},
		{speed: 13.9, want: WindVeryStrong, label: "штормовой"},
		{speed: 30, want: WindVeryStrong, label: "штормовой"},
	}

	for _, tc := range tests {
		got := WindLevelFor(tc.speed)
		if got != tc.want {
			t.Errorf("WindLevelFor(%v) = %v, ожидалось %v", tc.speed, got, tc.want)
		}
		if got.Label() != tc.label {
			t.Errorf("метка для %v = %q, ожидалось %q", tc.speed, got.Label(), tc.label)
		}
	}
}

func TestWindLevelIconsAndAlerts(t *testing.T) {
	tests := []struct {
		level     WindLevel
		icon      string
		alert     string
		windproof bool
	}{
		{level: WindCalm, icon: "🍃", alert: "", windproof: false},
		{level: WindLight, icon: "🍃", alert: "", windproof: false},
		{level: WindModerate, icon: "🌬", alert: "", windproof: false},
		{level: WindStrong, icon: "💨", alert: "⚠️", windproof: true},
		{level: WindVeryStrong, icon: "🌪", alert: "⛔", windproof: true},
	}

	for _, tc := range tests {
		if tc.level.Icon() != tc.icon {
			t.Errorf("иконка %v = %q, ожидалось %q", tc.level, tc.level.Icon(), tc.icon)
		}
		if tc.level.Alert() != tc.alert {
			t.Errorf("предупреждение %v = %q, ожидалось %q", tc.level, tc.level.Alert(), tc.alert)
		}
		if tc.level.NeedsWindproof() != tc.windproof {
			t.Errorf("ветрозащита %v = %v, ожидалось %v", tc.level, tc.level.NeedsWindproof(), tc.windproof)
		}
	}
}

func TestSummarizeWind(t *testing.T) {
	base := time.Date(2026, 9, 10, 7, 0, 0, 0, time.UTC)
	hours := []HourPoint{
		{Time: base, WindSpeedMS: Some(3.0), WindDirectionDeg: Some(90), WindGustsMS: Some(6.0)},
		{Time: base.Add(time.Hour), WindSpeedMS: Some(9.0), WindDirectionDeg: Some(225), WindGustsMS: Some(13.9)},
		{Time: base.Add(2 * time.Hour), WindSpeedMS: Some(4.0), WindDirectionDeg: Some(270), WindGustsMS: Some(8.0)},
	}

	summary := SummarizeWind(hours)
	if speed, _ := summary.MaxSpeedMS.Get(); speed != 9.0 {
		t.Errorf("максимум = %v, ожидалось 9.0", speed)
	}
	if gusts, _ := summary.MaxGustsMS.Get(); gusts != 13.9 {
		t.Errorf("порывы = %v, ожидалось 13.9", gusts)
	}
	if summary.GustWarning {
		t.Error("порывы 13.9 не должны включать предупреждение")
	}
	if summary.Level != WindStrong {
		t.Errorf("уровень = %v, ожидался сильный", summary.Level)
	}
	point, ok := summary.Direction.Get()
	if !ok || point.Rumb != "ЮЗ" {
		t.Errorf("направление на пике = %v, ожидалось ЮЗ", point)
	}
}

func TestSummarizeWindGustWarningThreshold(t *testing.T) {
	hours := []HourPoint{{WindSpeedMS: Some(5.0), WindGustsMS: Some(14.0)}}
	if !SummarizeWind(hours).GustWarning {
		t.Error("порывы 14.0 должны включать предупреждение")
	}
}

func TestSummarizeWindEmptyAndMissing(t *testing.T) {
	empty := SummarizeWind(nil)
	if empty.MaxSpeedMS.Valid() || empty.MaxGustsMS.Valid() || empty.Direction.Valid() {
		t.Error("для пустого набора часов сводка должна быть пустой")
	}
	if empty.Level != WindCalm {
		t.Errorf("уровень = %v, ожидался штиль", empty.Level)
	}

	partial := SummarizeWind([]HourPoint{{WindSpeedMS: None[float64](), WindGustsMS: Some(3.0)}})
	if partial.MaxSpeedMS.Valid() {
		t.Error("скорость должна остаться пустой")
	}
	if gusts, _ := partial.MaxGustsMS.Get(); gusts != 3.0 {
		t.Errorf("порывы = %v, ожидалось 3.0", gusts)
	}
}
