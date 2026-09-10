// Package render turns a report into a Telegram HTML message.
package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/i18n"
)

// Layout selects how the hourly block is printed.
type Layout string

const (
	LayoutLines Layout = "lines"
	LayoutTable Layout = "table"
)

// ParseLayout reads a layout from configuration.
func ParseLayout(raw string) (Layout, error) {
	switch Layout(strings.ToLower(strings.TrimSpace(raw))) {
	case LayoutLines:
		return LayoutLines, nil
	case LayoutTable:
		return LayoutTable, nil
	default:
		return "", fmt.Errorf("%q is not supported, want lines or table", raw)
	}
}

const (
	// MessageLimit is the Telegram message length limit.
	MessageLimit = 4096

	missing = i18n.Missing
)

// Options carries everything the renderer needs beyond the report itself.
type Options struct {
	Printer  *i18n.Printer
	WindUnit domain.WindUnit
	Layout   Layout
}

func (o Options) printer() *i18n.Printer {
	if o.Printer == nil {
		return i18n.For(i18n.Default)
	}
	return o.Printer
}

func (o Options) unit() domain.WindUnit {
	if o.WindUnit == "" {
		return domain.WindUnitMS
	}
	return o.WindUnit
}

func (o Options) layout() Layout {
	if o.Layout == "" {
		return LayoutLines
	}
	return o.Layout
}

// Report builds the daily forecast message.
func Report(report domain.Report, options Options) string {
	printer := options.printer()

	var message strings.Builder
	for _, headline := range report.Headlines() {
		message.WriteString(headline.Icon + " " + escape(printer.Headline(headline)) + "\n")
	}
	message.WriteString("\n")

	message.WriteString(headerLine(report, options))
	message.WriteString(temperatureLine(report, options))
	message.WriteString(windLine(report, options))
	message.WriteString(sunLine(report, options))

	message.WriteString("\n⏱ <b>" + escape(printer.T(i18n.KeyHourlyHeader)) + "</b>\n")
	message.WriteString(hourlyBlock(report, options))

	message.WriteString("\n👕 <b>" + escape(printer.T(i18n.KeyOutfitHeader)) + "</b>\n")
	message.WriteString(escape(printer.AdviceText(report.Advice, options.unit())) + "\n")

	message.WriteString(attribution(printer))

	return trimToLimit(message.String())
}

// Current builds the short "weather right now" message.
func Current(place domain.Location, current domain.CurrentPoint, options Options) string {
	printer := options.printer()
	condition := domain.ConditionFor(current.WeatherCode, current.IsDay)
	unit := options.unit()

	var message strings.Builder
	fmt.Fprintf(&message, "%s <b>%s</b>\n", condition.Icon, escape(printer.T(i18n.KeyNowIn, place.Name)))

	line := escape(printer.Condition(condition)) + ", " + printer.Temperature(current.TemperatureC)
	if current.ApparentTemperatureC.Valid() {
		line += " (" + escape(printer.T(i18n.KeyFeelsLike, printer.Temperature(current.ApparentTemperatureC))) + ")"
	}
	message.WriteString(line + "\n")

	windText := "💨 " + escape(windPhrase(printer, current.WindSpeedMS, domain.CompassFor(current.WindDirectionDeg), unit))
	if gusts, ok := current.WindGustsMS.Get(); ok && gusts >= domain.GustWarningMS {
		windText += ", " + escape(printer.T(i18n.KeyWindGusts, i18n.Number(unit.FromMS(gusts), 0)))
	}
	message.WriteString(windText + "\n")

	if humidity, ok := current.RelativeHumidity.Get(); ok {
		message.WriteString("💧 " + escape(printer.T(i18n.KeyHumidity, humidity)) + "\n")
	}
	if amount, ok := current.PrecipitationMM.Get(); ok && amount > 0 {
		message.WriteString("🌧 " + escape(printer.T(i18n.KeyPrecipitation, i18n.Number(amount, 1))) + "\n")
	}

	message.WriteString("\n🕒 " + escape(printer.T(i18n.KeyMeasuredAt, current.Time.Format("15:04"))) + "\n")
	message.WriteString(attribution(printer))

	return trimToLimit(message.String())
}

// attribution is required by the CC BY 4.0 license of the Open-Meteo data.
func attribution(printer *i18n.Printer) string {
	return "\n<i>" + escape(printer.T(i18n.KeyAttribution)) + "</i>"
}

func headerLine(report domain.Report, options Options) string {
	printer := options.printer()
	return fmt.Sprintf("%s <b>%s</b> · %s\n",
		worstCondition(report).Icon,
		escape(report.Location.Name),
		escape(printer.Date(report.Date)),
	)
}

func temperatureLine(report domain.Report, options Options) string {
	printer := options.printer()
	return fmt.Sprintf("🌡 %s → %s (%s)\n",
		printer.Temperature(report.TemperatureMinC),
		printer.Temperature(report.TemperatureMaxC),
		escape(printer.T(i18n.KeyFeelsRange,
			printer.Temperature(report.ApparentMinC),
			printer.Temperature(report.ApparentMaxC),
		)),
	)
}

func windLine(report domain.Report, options Options) string {
	printer := options.printer()
	unit := options.unit()

	line := fmt.Sprintf("%s %s — %s",
		report.Wind.Level.Icon(),
		escape(windPhrase(printer, report.Wind.MaxSpeedMS, report.Wind.Direction, unit)),
		escape(printer.WindLevel(report.Wind.Level)),
	)
	if alert := report.Wind.Level.Alert(); alert != "" {
		line += " " + alert
	}
	if gusts, ok := report.Wind.MaxGustsMS.Get(); ok && gusts >= domain.GustWarningMS {
		line += ", " + escape(printer.T(i18n.KeyWindGusts, i18n.Number(unit.FromMS(gusts), 0)))
	}
	return line + "\n"
}

func windPhrase(printer *i18n.Printer, speed domain.Opt[float64], direction domain.Opt[domain.CompassPoint], unit domain.WindUnit) string {
	parts := []string{printer.T(i18n.KeyWindLabel)}
	if point, ok := direction.Get(); ok {
		parts = append(parts, printer.Rose(point.Rose)+" "+point.Arrow)
	}
	if value, ok := speed.Get(); ok {
		parts = append(parts, printer.T(i18n.KeyWindUpTo, i18n.Number(unit.FromMS(value), 0), printer.Unit(unit)))
	} else {
		parts = append(parts, missing)
	}
	return strings.Join(parts, " ")
}

func sunLine(report domain.Report, options Options) string {
	printer := options.printer()

	uv := missing
	if value, ok := report.UVIndexMax.Get(); ok {
		uv = i18n.Number(value, 0)
	}
	return fmt.Sprintf("☀️ %s %s · 🌅 %s · 🌇 %s\n",
		escape(printer.T(i18n.KeyUVLabel)),
		uv,
		formatClock(report.Day.Sunrise),
		formatClock(report.Day.Sunset),
	)
}

func hourlyBlock(report domain.Report, options Options) string {
	if len(report.Hours) == 0 {
		return missing + "\n"
	}
	if options.layout() == LayoutTable {
		return hourlyTable(report, options)
	}
	return hourlyLines(report, options)
}

func hourlyLines(report domain.Report, options Options) string {
	printer := options.printer()
	unit := options.unit()

	var block strings.Builder
	for _, hour := range report.Hours {
		fields := []string{
			hour.Time.Format("15"),
			hour.Condition.Icon,
			printer.Temperature(hour.TemperatureC),
			formatProbability(hour.PrecipitationProbability),
		}
		if amount, ok := hour.PrecipitationMM.Get(); ok && amount >= domain.PrecipitationTraceMM {
			fields = append(fields, i18n.Number(amount, 1)+printer.T(i18n.KeyUnitMM))
		}
		fields = append(fields, hourWind(printer, hour, unit))
		if alert := hour.WindLevel.Alert(); alert != "" {
			fields = append(fields, alert)
		}
		block.WriteString(strings.Join(fields, " ") + "\n")
	}
	return block.String()
}

func hourlyTable(report domain.Report, options Options) string {
	printer := options.printer()
	unit := options.unit()

	var block strings.Builder
	block.WriteString("<pre>\n")
	block.WriteString(escape(printer.T(i18n.KeyTableHeader)) + "\n")
	for _, hour := range report.Hours {
		amount := missing
		if value, ok := hour.PrecipitationMM.Get(); ok && value >= domain.PrecipitationTraceMM {
			amount = i18n.Number(value, 1)
		}
		direction := ""
		if point, ok := hour.Compass.Get(); ok {
			direction = printer.Rose(point.Rose) + point.Arrow
		}
		speed := missing
		if value, ok := hour.WindSpeedMS.Get(); ok {
			speed = i18n.Number(unit.FromMS(value), 0)
		}
		fmt.Fprintf(&block, "%s %4s %3s %4s %4s  %2s %s\n",
			hour.Time.Format("15"),
			printer.Temperature(hour.TemperatureC),
			formatBareTemperature(hour.ApparentTemperatureC),
			formatProbability(hour.PrecipitationProbability),
			amount,
			speed,
			direction,
		)
	}
	block.WriteString("</pre>\n")
	return block.String()
}

func hourWind(printer *i18n.Printer, hour domain.ReportHour, unit domain.WindUnit) string {
	speed := missing
	if value, ok := hour.WindSpeedMS.Get(); ok {
		speed = i18n.Number(unit.FromMS(value), 0)
	}
	direction := ""
	if point, ok := hour.Compass.Get(); ok {
		direction = " " + printer.Rose(point.Rose) + point.Arrow
	}
	return hour.WindLevel.Icon() + " " + speed + direction
}

func worstCondition(report domain.Report) domain.Condition {
	worst := domain.Condition{Icon: "❔", Kind: domain.ConditionUnknown}
	highest := -1
	for _, hour := range report.Hours {
		code, ok := hour.WeatherCode.Get()
		if !ok || code <= highest {
			continue
		}
		highest = code
		worst = hour.Condition
	}
	return worst
}

func formatBareTemperature(value domain.Opt[float64]) string {
	number, ok := value.Get()
	if !ok {
		return missing
	}
	return i18n.Number(number, 0)
}

func formatProbability(value domain.Opt[int]) string {
	number, ok := value.Get()
	if !ok {
		return missing
	}
	return fmt.Sprintf("%d%%", number)
}

func formatClock(value domain.Opt[time.Time]) string {
	moment, ok := value.Get()
	if !ok {
		return missing
	}
	return moment.Format("15:04")
}

func escape(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	return strings.ReplaceAll(text, ">", "&gt;")
}

func trimToLimit(message string) string {
	runes := []rune(message)
	if len(runes) <= MessageLimit {
		return message
	}
	const ellipsis = "\n…"
	return string(runes[:MessageLimit-len([]rune(ellipsis))]) + ellipsis
}
