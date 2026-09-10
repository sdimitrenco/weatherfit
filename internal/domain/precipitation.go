package domain

import "time"

// RainVerdict says whether rain protection is needed.
type RainVerdict int

const (
	RainNotNeeded RainVerdict = iota
	RainJustInCase
	RainRequired
)

// Precipitation thresholds.
const (
	ProbabilityMaybePercent    = 30
	ProbabilityLikelyPercent   = 60
	ProbabilitySumPercent      = 50
	PrecipitationTraceMM       = 0.1
	PrecipitationDrizzleMM     = 0.3
	PrecipitationHeavyHourMM   = 2.5
	PrecipitationSumRequiredMM = 2.0
	PrecipitationSumHeavyMM    = 10.0
)

// RainWindow is a continuous run of wet hours, both bounds inclusive.
type RainWindow struct {
	From time.Time
	To   time.Time
}

// PrecipitationAnalysis summarizes precipitation over a set of hours.
type PrecipitationAnalysis struct {
	Verdict RainVerdict
	Heavy   bool
	TotalMM float64
	// LiquidMM is rain plus showers without snow, telling a snowy day from a wet one.
	LiquidMM       float64
	SnowfallCM     float64
	MaxHourlyMM    float64
	MaxProbability Opt[int]
	Windows        []RainWindow
	Thunderstorm   bool
	Freezing       bool
	Snow           bool
}

// Wet reports meaningful precipitation in this hour: at least 30% probability
// and at least 0.1 mm.
func (h HourPoint) Wet() bool {
	probability, hasProbability := h.PrecipitationProbability.Get()
	amount, hasAmount := h.PrecipitationMM.Get()
	if !hasProbability || !hasAmount {
		return false
	}
	return probability >= ProbabilityMaybePercent && amount >= PrecipitationTraceMM
}

// AnalyzePrecipitation computes the rain verdict, precipitation windows and hazards.
func AnalyzePrecipitation(hours []HourPoint) PrecipitationAnalysis {
	analysis := PrecipitationAnalysis{
		Verdict:        RainNotNeeded,
		MaxProbability: None[int](),
	}

	required := false
	anyWet := false

	for _, hour := range hours {
		amount, hasAmount := hour.PrecipitationMM.Get()
		if hasAmount {
			analysis.TotalMM += amount
			if amount > analysis.MaxHourlyMM {
				analysis.MaxHourlyMM = amount
			}
			if amount >= PrecipitationHeavyHourMM {
				analysis.Heavy = true
			}
		}

		analysis.LiquidMM += hour.RainMM.Or(0) + hour.ShowersMM.Or(0)
		analysis.SnowfallCM += hour.SnowfallCM.Or(0)

		if probability, ok := hour.PrecipitationProbability.Get(); ok {
			if current, has := analysis.MaxProbability.Get(); !has || probability > current {
				analysis.MaxProbability = Some(probability)
			}
			if probability >= ProbabilityLikelyPercent && hasAmount && amount >= PrecipitationDrizzleMM {
				required = true
			}
		}

		if hour.Wet() {
			anyWet = true
		}
		if IsThunderstorm(hour.WeatherCode) {
			analysis.Thunderstorm = true
		}
		if IsFreezing(hour.WeatherCode) {
			analysis.Freezing = true
		}
		if IsSnow(hour.WeatherCode) || hour.SnowfallCM.Or(0) > 0 {
			analysis.Snow = true
		}
	}

	if analysis.TotalMM >= PrecipitationSumRequiredMM &&
		analysis.MaxProbability.Or(0) >= ProbabilitySumPercent {
		required = true
	}
	if analysis.TotalMM >= PrecipitationSumHeavyMM {
		analysis.Heavy = true
	}

	switch {
	case required:
		analysis.Verdict = RainRequired
	case anyWet:
		analysis.Verdict = RainJustInCase
	default:
		analysis.Verdict = RainNotNeeded
	}

	analysis.Windows = rainWindows(hours)
	return analysis
}

// SnowDominant reports that the precipitation is snow rather than rain, so
// boots and a hood matter more than an umbrella.
func (a PrecipitationAnalysis) SnowDominant() bool {
	return a.Snow && a.LiquidMM < PrecipitationTraceMM
}

// UmbrellaUseless reports that this wind makes an umbrella pointless.
func (a PrecipitationAnalysis) UmbrellaUseless(wind WindSummary) bool {
	if a.Verdict != RainRequired {
		return false
	}
	return wind.Level.NeedsWindproof() || wind.GustWarning
}

func rainWindows(hours []HourPoint) []RainWindow {
	var windows []RainWindow
	open := false

	for _, hour := range hours {
		if !hour.Wet() {
			open = false
			continue
		}
		last := len(windows) - 1
		if !open || hour.Time.Sub(windows[last].To) > time.Hour {
			windows = append(windows, RainWindow{From: hour.Time, To: hour.Time})
			open = true
			continue
		}
		windows[last].To = hour.Time
	}

	return windows
}
