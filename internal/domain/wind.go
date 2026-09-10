package domain

import "math"

// WindLevel groups mean wind speed into Beaufort-like bands.
type WindLevel int

const (
	WindCalm WindLevel = iota
	WindLight
	WindModerate
	WindStrong
	WindVeryStrong
)

// Wind thresholds in m/s.
const (
	WindCalmMaxMS     = 1.6
	WindLightMaxMS    = 5.5
	WindModerateMaxMS = 8.0
	WindStrongMaxMS   = 13.9
	GustWarningMS     = 14.0
)

// WindLevelFor returns the level for a mean speed in m/s.
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

// Icon returns the level icon.
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

// Alert returns a warning sign for strong wind, or an empty string.
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

// NeedsWindproof reports that a windproof layer is needed.
func (l WindLevel) NeedsWindproof() bool {
	return l >= WindStrong
}

// CompassPoint holds the compass point the wind blows from and an arrow
// pointing where it blows to.
type CompassPoint struct {
	Rose  Rose
	Arrow string
}

// Rose indexes the eight compass points clockwise from north.
type Rose int

const (
	RoseNorth Rose = iota
	RoseNorthEast
	RoseEast
	RoseSouthEast
	RoseSouth
	RoseSouthWest
	RoseWest
	RoseNorthWest
)

var arrows = [8]string{"↓", "↙", "←", "↖", "↑", "↗", "→", "↘"}

// CompassFor converts a "wind from" bearing into a compass point and an arrow
// pointing where the wind blows to.
func CompassFor(fromDegrees Opt[int]) Opt[CompassPoint] {
	degrees, ok := fromDegrees.Get()
	if !ok {
		return None[CompassPoint]()
	}
	normalized := math.Mod(math.Mod(float64(degrees), 360)+360, 360)
	index := int((normalized+22.5)/45) % len(arrows)
	return Some(CompassPoint{Rose: Rose(index), Arrow: arrows[index]})
}

// WindSummary aggregates wind over a set of hours.
type WindSummary struct {
	MaxSpeedMS Opt[float64]
	MaxGustsMS Opt[float64]
	// Direction is taken from the windiest hour of the set.
	Direction   Opt[CompassPoint]
	Level       WindLevel
	GustWarning bool
}

// SummarizeWind computes peak speed, peak gusts and the direction at the peak.
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
