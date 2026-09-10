package domain

import "time"

// ReportHour is a forecast hour with icon and wind level already resolved.
type ReportHour struct {
	HourPoint
	Condition Condition
	Compass   Opt[CompassPoint]
	WindLevel WindLevel
}

// Report holds everything a message about one day needs.
type Report struct {
	Location        Location
	Date            time.Time
	Day             DaySummary
	Hours           []ReportHour
	TemperatureMinC Opt[float64]
	TemperatureMaxC Opt[float64]
	ApparentMinC    Opt[float64]
	ApparentMaxC    Opt[float64]
	UVIndexMax      Opt[float64]
	Wind            WindSummary
	Precipitation   PrecipitationAnalysis
	Advice          OutfitAdvice
	// UnknownCodes lists WMO codes missing from the table; the caller logs a warning.
	UnknownCodes []int
}

// HeadlineKind identifies a verdict line for translation lookup.
type HeadlineKind string

const (
	HeadlineSnow         HeadlineKind = "snow"
	HeadlineRainCoat     HeadlineKind = "rain_coat"
	HeadlineRainUmbrella HeadlineKind = "rain_umbrella"
	HeadlineRainMaybe    HeadlineKind = "rain_maybe"
	HeadlineDry          HeadlineKind = "dry"
	HeadlineOutfit       HeadlineKind = "outfit"
)

// Headline is one verdict line at the top of the message.
type Headline struct {
	Kind         HeadlineKind
	Icon         string
	Band         BandID
	TemperatureC Opt[float64]
	Windows      []RainWindow
}

// Analyze turns a set of hours into a report ready for rendering.
func Analyze(location Location, date time.Time, day DaySummary, hours []HourPoint) Report {
	report := Report{
		Location:        location,
		Date:            date,
		Day:             day,
		Hours:           make([]ReportHour, 0, len(hours)),
		TemperatureMinC: None[float64](),
		TemperatureMaxC: None[float64](),
		ApparentMinC:    None[float64](),
		ApparentMaxC:    None[float64](),
		UVIndexMax:      None[float64](),
	}

	seenUnknown := make(map[int]struct{})
	for _, hour := range hours {
		condition := ConditionFor(hour.WeatherCode, hour.IsDay)
		if !condition.Known {
			if code, ok := hour.WeatherCode.Get(); ok {
				if _, seen := seenUnknown[code]; !seen {
					seenUnknown[code] = struct{}{}
					report.UnknownCodes = append(report.UnknownCodes, code)
				}
			}
		}

		report.Hours = append(report.Hours, ReportHour{
			HourPoint: hour,
			Condition: condition,
			Compass:   CompassFor(hour.WindDirectionDeg),
			WindLevel: WindLevelFor(hour.WindSpeedMS.Or(0)),
		})

		report.TemperatureMinC = keepLower(report.TemperatureMinC, hour.TemperatureC)
		report.TemperatureMaxC = keepHigher(report.TemperatureMaxC, hour.TemperatureC)
		report.ApparentMinC = keepLower(report.ApparentMinC, hour.ApparentTemperatureC)
		report.ApparentMaxC = keepHigher(report.ApparentMaxC, hour.ApparentTemperatureC)
		report.UVIndexMax = keepHigher(report.UVIndexMax, hour.UVIndex)
	}

	report.Wind = SummarizeWind(hours)
	report.Precipitation = AnalyzePrecipitation(hours)

	if !report.TemperatureMinC.Valid() {
		report.TemperatureMinC = day.TemperatureMinC
	}
	if !report.TemperatureMaxC.Valid() {
		report.TemperatureMaxC = day.TemperatureMaxC
	}
	if !report.ApparentMinC.Valid() {
		report.ApparentMinC = day.ApparentTemperatureMinC
	}
	if !report.ApparentMaxC.Valid() {
		report.ApparentMaxC = day.ApparentTemperatureMaxC
	}
	if !report.UVIndexMax.Valid() {
		report.UVIndexMax = day.UVIndexMax
	}

	report.Advice = BuildOutfitAdvice(OutfitInput{
		ApparentMinC:  report.ApparentMinC,
		ApparentMaxC:  report.ApparentMaxC,
		Wind:          report.Wind,
		Precipitation: report.Precipitation,
		UVIndexMax:    report.UVIndexMax,
	})

	return report
}

// Headlines returns the verdict lines: rain first, clothing second.
func (r Report) Headlines() []Headline {
	return []Headline{r.rainHeadline(), r.outfitHeadline()}
}

func (r Report) rainHeadline() Headline {
	windows := r.Precipitation.Windows

	if r.Precipitation.SnowDominant() {
		return Headline{Kind: HeadlineSnow, Icon: "❄️", Windows: windows}
	}

	switch r.Precipitation.Verdict {
	case RainRequired:
		if r.Precipitation.UmbrellaUseless(r.Wind) {
			return Headline{Kind: HeadlineRainCoat, Icon: "☂️", Windows: windows}
		}
		return Headline{Kind: HeadlineRainUmbrella, Icon: "☂️", Windows: windows}
	case RainJustInCase:
		return Headline{Kind: HeadlineRainMaybe, Icon: "🌂", Windows: windows}
	default:
		return Headline{Kind: HeadlineDry, Icon: "🙂"}
	}
}

func (r Report) outfitHeadline() Headline {
	band := bandFor(r.ApparentMinC)

	icon := "🧥"
	if value, ok := r.ApparentMinC.Get(); ok {
		switch {
		case value >= 18:
			icon = "👕"
		case value < 0:
			icon = "🧣"
		}
	}

	return Headline{
		Kind:         HeadlineOutfit,
		Icon:         icon,
		Band:         band.id,
		TemperatureC: r.ApparentMinC,
	}
}

// Hours returns the first and last hour of the window.
func (w RainWindow) Hours() (int, int) {
	return w.From.Hour(), w.To.Hour()
}

func keepLower(current, candidate Opt[float64]) Opt[float64] {
	value, ok := candidate.Get()
	if !ok {
		return current
	}
	if existing, has := current.Get(); has && existing <= value {
		return current
	}
	return Some(value)
}

func keepHigher(current, candidate Opt[float64]) Opt[float64] {
	value, ok := candidate.Get()
	if !ok {
		return current
	}
	if existing, has := current.Get(); has && existing >= value {
		return current
	}
	return Some(value)
}
