package i18n

import (
	"fmt"
	"strings"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
)

var catalogs = map[Lang]map[Key]string{
	English: catalogEN,
	Russian: catalogRU,
	German:  catalogDE,
}

// Printer renders localized text for one language.
type Printer struct {
	lang Lang
}

// For returns a printer for the language, falling back to Default.
func For(lang Lang) *Printer {
	if !Valid(lang) {
		lang = Default
	}
	return &Printer{lang: lang}
}

// Lang returns the printer language.
func (p *Printer) Lang() Lang {
	return p.lang
}

// T looks up a key and formats it with args. A missing key falls back to
// English and then to the key itself, so a gap in a catalog never panics.
func (p *Printer) T(key Key, args ...any) string {
	format, ok := catalogs[p.lang][key]
	if !ok {
		if format, ok = catalogEN[key]; !ok {
			return string(key)
		}
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

var weekdayKeys = map[time.Weekday]Key{
	time.Monday:    KeyWeekday1,
	time.Tuesday:   KeyWeekday2,
	time.Wednesday: KeyWeekday3,
	time.Thursday:  KeyWeekday4,
	time.Friday:    KeyWeekday5,
	time.Saturday:  KeyWeekday6,
	time.Sunday:    KeyWeekday7,
}

var monthKeys = map[time.Month]Key{
	time.January:   KeyMonth1,
	time.February:  KeyMonth2,
	time.March:     KeyMonth3,
	time.April:     KeyMonth4,
	time.May:       KeyMonth5,
	time.June:      KeyMonth6,
	time.July:      KeyMonth7,
	time.August:    KeyMonth8,
	time.September: KeyMonth9,
	time.October:   KeyMonth10,
	time.November:  KeyMonth11,
	time.December:  KeyMonth12,
}

// Weekday returns the localized weekday name.
func (p *Printer) Weekday(day time.Weekday) string {
	return p.T(weekdayKeys[day])
}

// Month returns the localized month name in the form used with a day number.
func (p *Printer) Month(month time.Month) string {
	return p.T(monthKeys[month])
}

// Date formats a date as weekday plus day and month.
func (p *Printer) Date(date time.Time) string {
	return fmt.Sprintf("%s, %d %s", p.Weekday(date.Weekday()), date.Day(), p.Month(date.Month()))
}

var roseKeys = map[domain.Rose]Key{
	domain.RoseNorth:     KeyRoseNorth,
	domain.RoseNorthEast: KeyRoseNorthEast,
	domain.RoseEast:      KeyRoseEast,
	domain.RoseSouthEast: KeyRoseSouthEast,
	domain.RoseSouth:     KeyRoseSouth,
	domain.RoseSouthWest: KeyRoseSouthWest,
	domain.RoseWest:      KeyRoseWest,
	domain.RoseNorthWest: KeyRoseNorthWest,
}

// Rose returns the localized compass point abbreviation.
func (p *Printer) Rose(rose domain.Rose) string {
	return p.T(roseKeys[rose])
}

var windLevelKeys = map[domain.WindLevel]Key{
	domain.WindCalm:       KeyWindCalm,
	domain.WindLight:      KeyWindLight,
	domain.WindModerate:   KeyWindModerate,
	domain.WindStrong:     KeyWindStrong,
	domain.WindVeryStrong: KeyWindVeryStrong,
}

// WindLevel returns the localized wind level label.
func (p *Printer) WindLevel(level domain.WindLevel) string {
	return p.T(windLevelKeys[level])
}

// Unit returns the localized wind unit label.
func (p *Printer) Unit(unit domain.WindUnit) string {
	if unit == domain.WindUnitKMH {
		return p.T(KeyUnitKMH)
	}
	return p.T(KeyUnitMS)
}

var conditionKeys = map[domain.ConditionKind]Key{
	domain.ConditionUnknown:          KeyConditionUnknown,
	domain.ConditionClear:            KeyConditionClear,
	domain.ConditionMostlyClear:      KeyConditionMostlyClear,
	domain.ConditionPartlyCloudy:     KeyConditionPartlyCloudy,
	domain.ConditionOvercast:         KeyConditionOvercast,
	domain.ConditionFog:              KeyConditionFog,
	domain.ConditionDrizzle:          KeyConditionDrizzle,
	domain.ConditionHeavyDrizzle:     KeyConditionHeavyDrizzle,
	domain.ConditionFreezingDrizzle:  KeyConditionFreezingDrizzle,
	domain.ConditionLightRain:        KeyConditionLightRain,
	domain.ConditionRain:             KeyConditionRain,
	domain.ConditionHeavyRain:        KeyConditionHeavyRain,
	domain.ConditionFreezingRain:     KeyConditionFreezingRain,
	domain.ConditionLightSnow:        KeyConditionLightSnow,
	domain.ConditionSnow:             KeyConditionSnow,
	domain.ConditionHeavySnow:        KeyConditionHeavySnow,
	domain.ConditionSnowGrains:       KeyConditionSnowGrains,
	domain.ConditionRainShowers:      KeyConditionRainShowers,
	domain.ConditionHeavyRainShowers: KeyConditionHeavyRainShowers,
	domain.ConditionSnowShowers:      KeyConditionSnowShowers,
	domain.ConditionHeavySnowShowers: KeyConditionHeavySnowShowers,
	domain.ConditionThunderstorm:     KeyConditionThunderstorm,
	domain.ConditionThunderstormHail: KeyConditionThunderstormHail,
}

// Condition returns the localized weather description.
func (p *Printer) Condition(condition domain.Condition) string {
	key, ok := conditionKeys[condition.Kind]
	if !ok {
		key = KeyConditionUnknown
	}
	return p.T(key)
}

var bandKeys = map[domain.BandID]Key{
	domain.BandTShirtShorts: KeyBandTShirtShorts,
	domain.BandTShirtPants:  KeyBandTShirtPants,
	domain.BandHoodieJacket: KeyBandHoodieJacket,
	domain.BandSweaterCoat:  KeyBandSweaterCoat,
	domain.BandWarmCoat:     KeyBandWarmCoat,
	domain.BandWinterCoat:   KeyBandWinterCoat,
	domain.BandDownJacket:   KeyBandDownJacket,
}

var outerKeys = map[domain.BandID]Key{
	domain.BandHoodieJacket: KeyOuterHoodieJacket,
	domain.BandSweaterCoat:  KeyOuterSweaterCoat,
	domain.BandWarmCoat:     KeyOuterWarmCoat,
	domain.BandWinterCoat:   KeyOuterWinterCoat,
}

// Band returns the localized clothing set.
func (p *Printer) Band(band domain.BandID) string {
	key, ok := bandKeys[band]
	if !ok {
		key = KeyBandHoodieJacket
	}
	return p.T(key)
}

var adviceKeys = map[domain.AdviceKind]Key{
	domain.AdviceBase:              KeyAdviceBase,
	domain.AdviceBaseNoTemperature: KeyAdviceBaseNoTemperature,
	domain.AdviceLayering:          KeyAdviceLayering,
	domain.AdviceLayeringWithOuter: KeyAdviceLayeringWithOuter,
	domain.AdviceRainMaybe:         KeyAdviceRainMaybe,
	domain.AdviceRainUmbrella:      KeyAdviceRainUmbrella,
	domain.AdviceRainCoat:          KeyAdviceRainCoat,
	domain.AdviceWaterproofShoes:   KeyAdviceWaterproofShoes,
	domain.AdviceSnow:              KeyAdviceSnow,
	domain.AdviceHazardThunder:     KeyAdviceHazardThunder,
	domain.AdviceHazardIce:         KeyAdviceHazardIce,
	domain.AdviceHazardSnow:        KeyAdviceHazardSnow,
	domain.AdviceWindproof:         KeyAdviceWindproof,
	domain.AdviceWindproofGusts:    KeyAdviceWindproofGusts,
	domain.AdviceUVGlasses:         KeyAdviceUVGlasses,
	domain.AdviceUVSunscreen:       KeyAdviceUVSunscreen,
}

// Advice renders one advice item.
func (p *Printer) Advice(item domain.AdviceItem, unit domain.WindUnit) string {
	key, ok := adviceKeys[item.Kind]
	if !ok {
		return ""
	}

	switch item.Kind {
	case domain.AdviceBase:
		return p.T(key, p.Temperature(item.TemperatureC), p.Band(item.Band))
	case domain.AdviceBaseNoTemperature:
		return p.T(key, p.Band(item.Band))
	case domain.AdviceLayering:
		return p.T(key, p.Temperature(item.TemperatureC))
	case domain.AdviceLayeringWithOuter:
		return p.T(key, p.Temperature(item.TemperatureC), p.T(outerKeys[item.Band]))
	case domain.AdviceRainMaybe, domain.AdviceRainUmbrella, domain.AdviceRainCoat, domain.AdviceSnow:
		return p.T(key, p.windowSuffix(item.Windows))
	case domain.AdviceWindproof:
		return p.T(key, p.WindLevel(item.WindLevel))
	case domain.AdviceWindproofGusts:
		return p.T(key, p.WindLevel(item.WindLevel), Number(unit.FromMS(item.SpeedMS.Or(0)), 0))
	case domain.AdviceUVGlasses, domain.AdviceUVSunscreen:
		return p.T(key, Number(item.UVIndex.Or(0), 0))
	default:
		return p.T(key)
	}
}

// AdviceText joins all advice items into one paragraph.
func (p *Printer) AdviceText(advice domain.OutfitAdvice, unit domain.WindUnit) string {
	sentences := make([]string, 0, len(advice.Items))
	for _, item := range advice.Items {
		if text := p.Advice(item, unit); text != "" {
			sentences = append(sentences, text)
		}
	}
	return strings.Join(sentences, " ")
}

// Headline renders one verdict line without its icon.
func (p *Printer) Headline(headline domain.Headline) string {
	switch headline.Kind {
	case domain.HeadlineSnow:
		if window := p.RainWindows(headline.Windows); window != "" {
			return p.T(KeyHeadlineSnowWindow, window)
		}
		return p.T(KeyHeadlineSnow)
	case domain.HeadlineRainCoat:
		return p.T(KeyHeadlineRainCoat) + p.windowSuffix(headline.Windows)
	case domain.HeadlineRainUmbrella:
		return p.T(KeyHeadlineRainUmbrella) + p.windowSuffix(headline.Windows)
	case domain.HeadlineRainMaybe:
		return p.T(KeyHeadlineRainMaybe) + p.windowSuffix(headline.Windows)
	case domain.HeadlineDry:
		return p.T(KeyHeadlineDry)
	case domain.HeadlineOutfit:
		if headline.TemperatureC.Valid() {
			return p.T(KeyHeadlineOutfit, p.Temperature(headline.TemperatureC), p.Band(headline.Band))
		}
		return p.Band(headline.Band)
	default:
		return ""
	}
}

// RainWindows renders precipitation windows, for example "14–17 h".
func (p *Printer) RainWindows(windows []domain.RainWindow) string {
	if len(windows) == 0 {
		return ""
	}

	labels := make([]string, 0, len(windows))
	for _, window := range windows {
		from, to := window.Hours()
		if from == to {
			labels = append(labels, fmt.Sprintf("%02d", from))
			continue
		}
		labels = append(labels, fmt.Sprintf("%02d–%02d", from, to))
	}

	joined := labels[0]
	if len(labels) > 1 {
		joined = strings.Join(labels[:len(labels)-1], ", ") + " " + p.T(KeyWindowsJoiner) + " " + labels[len(labels)-1]
	}
	return p.T(KeyWindowsHours, joined)
}

func (p *Printer) windowSuffix(windows []domain.RainWindow) string {
	window := p.RainWindows(windows)
	if window == "" {
		return ""
	}
	return p.T(KeyHeadlineWindowSuffix, window)
}

// Temperature formats a temperature with a degree sign, or a dash when absent.
func (p *Printer) Temperature(value domain.Opt[float64]) string {
	number, ok := value.Get()
	if !ok {
		return Missing
	}
	return Number(number, 0) + "°"
}
