package domain

import "time"

// RainVerdict — нужна ли защита от дождя.
type RainVerdict int

const (
	RainNotNeeded RainVerdict = iota
	RainJustInCase
	RainRequired
)

// Пороги осадков.
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

// RainWindow — непрерывный интервал мокрых часов, границы включительно.
type RainWindow struct {
	From time.Time
	To   time.Time
}

// PrecipitationAnalysis — итог по осадкам за набор часов.
type PrecipitationAnalysis struct {
	Verdict        RainVerdict
	Heavy          bool
	TotalMM        float64
	MaxHourlyMM    float64
	MaxProbability Opt[int]
	Windows        []RainWindow
	Thunderstorm   bool
	Freezing       bool
	Snow           bool
}

// Wet сообщает, что в час ожидаются заметные осадки: вероятность не ниже 30%
// и хотя бы 0.1 мм.
func (h HourPoint) Wet() bool {
	probability, hasProbability := h.PrecipitationProbability.Get()
	amount, hasAmount := h.PrecipitationMM.Get()
	if !hasProbability || !hasAmount {
		return false
	}
	return probability >= ProbabilityMaybePercent && amount >= PrecipitationTraceMM
}

// AnalyzePrecipitation считает вердикт по дождю, окна осадков и опасные явления.
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

// UmbrellaUseless сообщает, что при таком ветре зонт бесполезен и нужен дождевик.
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
