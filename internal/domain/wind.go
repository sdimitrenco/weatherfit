package domain

import "math"

// WindLevel — сила ветра по средней скорости, по мотивам шкалы Бофорта.
type WindLevel int

const (
	WindCalm WindLevel = iota
	WindLight
	WindModerate
	WindStrong
	WindVeryStrong
)

// Пороги силы ветра в м/с.
const (
	WindCalmMaxMS     = 1.6
	WindLightMaxMS    = 5.5
	WindModerateMaxMS = 8.0
	WindStrongMaxMS   = 13.9
	GustWarningMS     = 14.0
)

// WindLevelFor возвращает уровень ветра для средней скорости в м/с.
func WindLevelFor(speedMS float64) WindLevel {
	switch {
	case speedMS < WindCalmMaxMS:
		return WindCalm
	case speedMS < WindLightMaxMS:
		return WindLight
	case speedMS < WindModerateMaxMS:
		return WindModerate
	case speedMS < WindStrongMaxMS:
		return WindStrong
	default:
		return WindVeryStrong
	}
}

// Label возвращает название уровня.
func (l WindLevel) Label() string {
	switch l {
	case WindCalm:
		return "штиль"
	case WindLight:
		return "слабый"
	case WindModerate:
		return "умеренный"
	case WindStrong:
		return "сильный"
	case WindVeryStrong:
		return "штормовой"
	default:
		return "штиль"
	}
}

// Icon возвращает иконку уровня.
func (l WindLevel) Icon() string {
	switch l {
	case WindCalm, WindLight:
		return "🍃"
	case WindModerate:
		return "🌬"
	case WindStrong:
		return "💨"
	case WindVeryStrong:
		return "🌪"
	default:
		return "🍃"
	}
}

// Alert возвращает знак предупреждения для сильного ветра, иначе пустую строку.
func (l WindLevel) Alert() string {
	switch l {
	case WindStrong:
		return "⚠️"
	case WindVeryStrong:
		return "⛔"
	default:
		return ""
	}
}

// NeedsWindproof сообщает, что нужен ветрозащитный верх.
func (l WindLevel) NeedsWindproof() bool {
	return l >= WindStrong
}

// CompassPoint — румб направления, откуда дует, и стрелка, куда дует.
type CompassPoint struct {
	Rumb  string
	Arrow string
}

var compassPoints = []CompassPoint{
	{Rumb: "С", Arrow: "↓"},
	{Rumb: "СВ", Arrow: "↙"},
	{Rumb: "В", Arrow: "←"},
	{Rumb: "ЮВ", Arrow: "↖"},
	{Rumb: "Ю", Arrow: "↑"},
	{Rumb: "ЮЗ", Arrow: "↗"},
	{Rumb: "З", Arrow: "→"},
	{Rumb: "СЗ", Arrow: "↘"},
}

// CompassFor переводит направление «откуда дует» в румб и стрелку «куда дует».
func CompassFor(fromDegrees Opt[int]) Opt[CompassPoint] {
	degrees, ok := fromDegrees.Get()
	if !ok {
		return None[CompassPoint]()
	}
	normalized := math.Mod(math.Mod(float64(degrees), 360)+360, 360)
	index := int((normalized+22.5)/45) % len(compassPoints)
	return Some(compassPoints[index])
}

// WindSummary — сводка по ветру за набор часов.
type WindSummary struct {
	MaxSpeedMS Opt[float64]
	MaxGustsMS Opt[float64]
	// Direction — направление в самый ветреный час набора.
	Direction   Opt[CompassPoint]
	Level       WindLevel
	GustWarning bool
}

// SummarizeWind считает максимальную скорость, порывы и направление на пике.
func SummarizeWind(hours []HourPoint) WindSummary {
	summary := WindSummary{
		MaxSpeedMS: None[float64](),
		MaxGustsMS: None[float64](),
		Direction:  None[CompassPoint](),
	}

	peak := math.Inf(-1)
	for _, hour := range hours {
		if speed, ok := hour.WindSpeedMS.Get(); ok && speed > peak {
			peak = speed
			summary.MaxSpeedMS = Some(speed)
			summary.Direction = CompassFor(hour.WindDirectionDeg)
		}
		if gusts, ok := hour.WindGustsMS.Get(); ok {
			if current, has := summary.MaxGustsMS.Get(); !has || gusts > current {
				summary.MaxGustsMS = Some(gusts)
			}
		}
	}

	summary.Level = WindLevelFor(summary.MaxSpeedMS.Or(0))
	summary.GustWarning = summary.MaxGustsMS.Or(0) >= GustWarningMS
	return summary
}
