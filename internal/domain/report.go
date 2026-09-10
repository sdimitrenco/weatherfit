package domain

import (
	"fmt"
	"strings"
	"time"
)

// ReportHour — час прогноза с уже посчитанными иконками и уровнем ветра.
type ReportHour struct {
	HourPoint
	Condition Condition
	Compass   Opt[CompassPoint]
	WindLevel WindLevel
}

// Report — всё, что нужно для сообщения об одном дне.
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
	// UnknownCodes — коды WMO, которых нет в таблице; вызывающий слой пишет warning.
	UnknownCodes []int
}

// Headline — строка вердикта в начале сообщения.
type Headline struct {
	Icon string
	Text string
}

// Analyze превращает набор часов в готовый к рендеру отчёт.
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
		ApparentMinC:   report.ApparentMinC,
		ApparentMaxC:   report.ApparentMaxC,
		Wind:           report.Wind,
		Precipitation:  report.Precipitation,
		UVIndexMax:     report.UVIndexMax,
		RainWindowText: FormatRainWindows(report.Precipitation.Windows),
	})

	return report
}

// Headlines возвращает первые строки сообщения: вердикт по дождю и по одежде.
func (r Report) Headlines() []Headline {
	return []Headline{r.rainHeadline(), r.outfitHeadline()}
}

func (r Report) rainHeadline() Headline {
	window := FormatRainWindows(r.Precipitation.Windows)
	suffix := ""
	if window != "" {
		suffix = " — дождь " + window
	}

	switch r.Precipitation.Verdict {
	case RainRequired:
		if r.Precipitation.UmbrellaUseless(r.Wind) {
			return Headline{Icon: "☂️", Text: "Бери дождевик" + suffix}
		}
		return Headline{Icon: "☂️", Text: "Бери зонт" + suffix}
	case RainJustInCase:
		return Headline{Icon: "🌂", Text: "Зонт на всякий случай" + suffix}
	default:
		if r.Precipitation.Snow {
			return Headline{Icon: "❄️", Text: "Дождя нет, но будет снег"}
		}
		return Headline{Icon: "🙂", Text: "Дождя не ожидается"}
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

	if value, ok := r.ApparentMinC.Get(); ok {
		return Headline{Icon: icon, Text: fmt.Sprintf("Утром ~%s: %s", degrees(value), band.kit)}
	}
	return Headline{Icon: icon, Text: band.kit}
}

// FormatRainWindows выводит окна осадков как «14–17 ч» или «10–11 и 14–17 ч».
func FormatRainWindows(windows []RainWindow) string {
	if len(windows) == 0 {
		return ""
	}
	labels := make([]string, 0, len(windows))
	for _, window := range windows {
		labels = append(labels, window.hoursLabel())
	}
	if len(labels) == 1 {
		return labels[0] + " ч"
	}
	return strings.Join(labels[:len(labels)-1], ", ") + " и " + labels[len(labels)-1] + " ч"
}

func (w RainWindow) hoursLabel() string {
	if w.From.Equal(w.To) {
		return fmt.Sprintf("%02d", w.From.Hour())
	}
	return fmt.Sprintf("%02d–%02d", w.From.Hour(), w.To.Hour())
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
