package render

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/i18n"
)

var update = flag.Bool("update", false, "rewrite the golden files")

const goldenDir = "../../../testdata/golden"

func berlin(t *testing.T) *time.Location {
	t.Helper()
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("cannot load the timezone: %v", err)
	}
	return location
}

type hourInput struct {
	hour         int
	temperature  float64
	apparent     float64
	probability  int
	amountMM     float64
	snowCM       float64
	code         int
	windMS       float64
	gustsMS      float64
	directionDeg int
	uv           float64
	night        bool
}

func buildReport(t *testing.T, place string, hours []hourInput) domain.Report {
	t.Helper()
	location := berlin(t)
	date := time.Date(2026, 9, 10, 0, 0, 0, 0, location)

	points := make([]domain.HourPoint, 0, len(hours))
	for _, input := range hours {
		points = append(points, domain.HourPoint{
			Time:                     time.Date(2026, 9, 10, input.hour, 0, 0, 0, location),
			TemperatureC:             domain.Some(input.temperature),
			ApparentTemperatureC:     domain.Some(input.apparent),
			PrecipitationProbability: domain.Some(input.probability),
			PrecipitationMM:          domain.Some(input.amountMM),
			SnowfallCM:               domain.Some(input.snowCM),
			WeatherCode:              domain.Some(input.code),
			WindSpeedMS:              domain.Some(input.windMS),
			WindDirectionDeg:         domain.Some(input.directionDeg),
			WindGustsMS:              domain.Some(input.gustsMS),
			UVIndex:                  domain.Some(input.uv),
			IsDay:                    domain.Some(!input.night),
		})
	}

	day := domain.DaySummary{
		Date:    date,
		Sunrise: domain.Some(time.Date(2026, 9, 10, 6, 32, 0, 0, location)),
		Sunset:  domain.Some(time.Date(2026, 9, 10, 19, 36, 0, 0, location)),
	}

	return domain.Analyze(
		domain.Location{Name: place, Latitude: 51.05, Longitude: 13.74},
		date,
		day,
		points,
	)
}

func sunnyDay(t *testing.T) domain.Report {
	hours := make([]hourInput, 0, 16)
	for hour := 7; hour <= 22; hour++ {
		hours = append(hours, hourInput{
			hour:         hour,
			temperature:  float64(13 + (hour-7)/2),
			apparent:     float64(11 + (hour-7)/2),
			probability:  0,
			code:         mapCode(hour, 0),
			windMS:       2.4,
			gustsMS:      5.0,
			directionDeg: 270,
			uv:           uvCurve(hour),
			night:        hour >= 20,
		})
	}
	return buildReport(t, "Дрезден", hours)
}

func rainyWindyDay(t *testing.T) domain.Report {
	hours := make([]hourInput, 0, 16)
	for hour := 7; hour <= 22; hour++ {
		input := hourInput{
			hour:         hour,
			temperature:  float64(13 + (hour-7)/2),
			apparent:     float64(11 + (hour-7)/2),
			probability:  10,
			code:         3,
			windMS:       6.0,
			gustsMS:      11.0,
			directionDeg: 225,
			uv:           uvCurve(hour) / 2,
			night:        hour >= 20,
		}
		if hour >= 14 && hour <= 17 {
			input.probability = 85
			input.amountMM = 1.4
			input.code = 63
			input.windMS = 9.2
			input.gustsMS = 15.5
		}
		hours = append(hours, input)
	}
	return buildReport(t, "Дрезден", hours)
}

func snowyDay(t *testing.T) domain.Report {
	hours := make([]hourInput, 0, 16)
	for hour := 7; hour <= 22; hour++ {
		hours = append(hours, hourInput{
			hour:         hour,
			temperature:  -4,
			apparent:     -9,
			probability:  70,
			amountMM:     0.4,
			snowCM:       0.6,
			code:         73,
			windMS:       4.0,
			gustsMS:      8.0,
			directionDeg: 45,
			night:        hour >= 17,
		})
	}
	return buildReport(t, "Дрезден", hours)
}

func thunderstormDay(t *testing.T) domain.Report {
	hours := make([]hourInput, 0, 16)
	for hour := 7; hour <= 22; hour++ {
		input := hourInput{
			hour:         hour,
			temperature:  float64(24 + (hour-7)/3),
			apparent:     float64(26 + (hour-7)/3),
			probability:  20,
			code:         1,
			windMS:       3.0,
			gustsMS:      6.0,
			directionDeg: 180,
			uv:           uvCurve(hour) + 2,
			night:        hour >= 20,
		}
		if hour >= 16 && hour <= 18 {
			input.probability = 90
			input.amountMM = 4.2
			input.code = 95
			input.windMS = 14.5
			input.gustsMS = 22.0
		}
		hours = append(hours, input)
	}
	return buildReport(t, "Дрезден", hours)
}

func mapCode(hour, base int) int {
	if hour >= 20 {
		return 0
	}
	return base
}

func uvCurve(hour int) float64 {
	switch {
	case hour < 9 || hour > 19:
		return 0
	case hour < 11:
		return 2
	case hour < 15:
		return 5
	default:
		return 3
	}
}

func goldenCompare(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(goldenDir, name)

	if *update {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatalf("cannot create the golden directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("cannot write the golden file: %v", err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read the golden file %s: %v (run go test -run Golden -update)", name, err)
	}
	if got != string(want) {
		t.Errorf("рендер отличается от %s\n--- получено ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestGoldenReports(t *testing.T) {
	scenarios := []struct {
		name    string
		report  func(*testing.T) domain.Report
		options Options
	}{
		{name: "sunny_en_lines.txt", report: sunnyDay, options: Options{Printer: i18n.For(i18n.English), Layout: LayoutLines}},
		{name: "sunny_ru_lines.txt", report: sunnyDay, options: Options{Printer: i18n.For(i18n.Russian), Layout: LayoutLines}},
		{name: "sunny_de_lines.txt", report: sunnyDay, options: Options{Printer: i18n.For(i18n.German), Layout: LayoutLines}},
		{name: "sunny_ru_table.txt", report: sunnyDay, options: Options{Printer: i18n.For(i18n.Russian), Layout: LayoutTable}},
		{name: "rain_wind_en_lines.txt", report: rainyWindyDay, options: Options{Printer: i18n.For(i18n.English), Layout: LayoutLines}},
		{name: "rain_wind_ru_lines.txt", report: rainyWindyDay, options: Options{Printer: i18n.For(i18n.Russian), Layout: LayoutLines}},
		{name: "rain_wind_de_lines.txt", report: rainyWindyDay, options: Options{Printer: i18n.For(i18n.German), Layout: LayoutLines}},
		{name: "rain_wind_ru_table.txt", report: rainyWindyDay, options: Options{Printer: i18n.For(i18n.Russian), Layout: LayoutTable}},
		{name: "snow_ru_lines.txt", report: snowyDay, options: Options{Printer: i18n.For(i18n.Russian), Layout: LayoutLines}},
		{name: "snow_en_lines.txt", report: snowyDay, options: Options{Printer: i18n.For(i18n.English), Layout: LayoutLines}},
		{name: "thunderstorm_ru_lines.txt", report: thunderstormDay, options: Options{Printer: i18n.For(i18n.Russian), Layout: LayoutLines}},
		{name: "thunderstorm_de_lines.txt", report: thunderstormDay, options: Options{Printer: i18n.For(i18n.German), Layout: LayoutLines}},
		{name: "rain_wind_ru_kmh.txt", report: rainyWindyDay, options: Options{Printer: i18n.For(i18n.Russian), Layout: LayoutLines, WindUnit: domain.WindUnitKMH}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			goldenCompare(t, scenario.name, Report(scenario.report(t), scenario.options))
		})
	}
}

func TestReportFitsTelegramLimit(t *testing.T) {
	for _, build := range []func(*testing.T) domain.Report{sunnyDay, rainyWindyDay, snowyDay, thunderstormDay} {
		for _, layout := range []Layout{LayoutLines, LayoutTable} {
			message := Report(build(t), Options{Layout: layout})
			if length := len([]rune(message)); length > MessageLimit {
				t.Errorf("сообщение занимает %d символов, лимит %d", length, MessageLimit)
			}
		}
	}
}

func TestHourlyLinesStayNarrow(t *testing.T) {
	const maxRunes = 30
	for _, build := range []func(*testing.T) domain.Report{sunnyDay, rainyWindyDay, snowyDay, thunderstormDay} {
		for _, layout := range []Layout{LayoutLines, LayoutTable} {
			message := Report(build(t), Options{Layout: layout})
			for _, line := range hourlyLinesOf(message) {
				if length := len([]rune(line)); length > maxRunes {
					t.Errorf("строка почасового блока длиной %d символов не влезет в экран телефона: %q", length, line)
				}
			}
		}
	}
}

func hourlyLinesOf(message string) []string {
	_, after, found := strings.Cut(message, "По часам</b>\n")
	if !found {
		return nil
	}
	block, _, _ := strings.Cut(after, "\n\n👕")
	var lines []string
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "<pre>" || line == "</pre>" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func TestReportHasAttribution(t *testing.T) {
	message := Report(sunnyDay(t), Options{Printer: i18n.For(i18n.Russian)})
	if !strings.Contains(message, i18n.For(i18n.Russian).T(i18n.KeyAttribution)) {
		t.Error("в сообщении нет атрибуции Open-Meteo")
	}
}

func TestReportStartsWithVerdict(t *testing.T) {
	message := Report(rainyWindyDay(t), Options{Printer: i18n.For(i18n.Russian)})
	head := strings.SplitN(message, "\n\n", 2)[0]
	if length := len([]rune(head)); length > 100 {
		t.Errorf("первые строки занимают %d символов, в push-уведомлении видно около 100:\n%s", length, head)
	}
	if !strings.Contains(head, "дождевик") {
		t.Errorf("вердикт по дождю должен быть в первых строках:\n%s", head)
	}
}

func TestReportEscapesDynamicText(t *testing.T) {
	report := buildReport(t, `<b>Дрезден</b> & "Ко"`, []hourInput{{hour: 7, temperature: 15, apparent: 14, code: 3, windMS: 2, directionDeg: 270}})
	message := Report(report, Options{Printer: i18n.For(i18n.Russian)})

	if strings.Contains(message, "<b>Дрезден</b>") {
		t.Error("название города должно быть экранировано")
	}
	if !strings.Contains(message, "&lt;b&gt;Дрезден&lt;/b&gt; &amp; ") {
		t.Errorf("экранирование не сработало:\n%s", message)
	}
}

func TestReportShowsMissingValuesAsDash(t *testing.T) {
	location := berlin(t)
	date := time.Date(2026, 9, 10, 0, 0, 0, 0, location)
	hours := []domain.HourPoint{{Time: time.Date(2026, 9, 10, 7, 0, 0, 0, location)}}

	report := domain.Analyze(domain.Location{Name: "Дрезден"}, date, domain.DaySummary{Date: date}, hours)
	message := Report(report, Options{Printer: i18n.For(i18n.Russian)})

	if !strings.Contains(message, missing) {
		t.Errorf("отсутствующие значения должны показываться как %q:\n%s", missing, message)
	}
	if strings.Contains(message, "0°") {
		t.Errorf("отсутствующая температура не должна превращаться в ноль:\n%s", message)
	}
}

func TestReportWindUnits(t *testing.T) {
	metric := Report(rainyWindyDay(t), Options{Printer: i18n.For(i18n.Russian), WindUnit: domain.WindUnitMS})
	if !strings.Contains(metric, "до 9 м/с") {
		t.Errorf("want скорость в м/с:\n%s", strings.SplitN(metric, "\n\n", 3)[1])
	}

	imperialish := Report(rainyWindyDay(t), Options{Printer: i18n.For(i18n.Russian), WindUnit: domain.WindUnitKMH})
	if !strings.Contains(imperialish, "до 33 км/ч") {
		t.Errorf("want скорость в км/ч:\n%s", strings.SplitN(imperialish, "\n\n", 3)[1])
	}
}

func TestCurrent(t *testing.T) {
	location := berlin(t)
	current := domain.CurrentPoint{
		Time:                 time.Date(2026, 9, 10, 10, 15, 0, 0, location),
		TemperatureC:         domain.Some(15.7),
		ApparentTemperatureC: domain.Some(14.5),
		PrecipitationMM:      domain.Some(0.0),
		WeatherCode:          domain.Some(3),
		WindSpeedMS:          domain.Some(2.25),
		WindDirectionDeg:     domain.Some(291),
		WindGustsMS:          domain.Some(6.2),
		RelativeHumidity:     domain.Some(68),
		IsDay:                domain.Some(true),
	}

	message := Current(domain.Location{Name: "Дрезден"}, current, Options{Printer: i18n.For(i18n.Russian)})
	for _, want := range []string{"Сейчас в Дрезден", "пасмурно", "16°", "ощущается 15°", "Влажность 68%", "10:15", i18n.For(i18n.Russian).T(i18n.KeyAttribution)} {
		if !strings.Contains(message, want) {
			t.Errorf("в сообщении нет %q:\n%s", want, message)
		}
	}
	if strings.Contains(message, "Осадки") {
		t.Errorf("при нулевых осадках строки быть не должно:\n%s", message)
	}
}

func TestCurrentWithRainAndGusts(t *testing.T) {
	current := domain.CurrentPoint{
		Time:            time.Date(2026, 9, 10, 15, 0, 0, 0, berlin(t)),
		TemperatureC:    domain.Some(12.0),
		PrecipitationMM: domain.Some(1.2),
		WeatherCode:     domain.Some(63),
		WindSpeedMS:     domain.Some(9.0),
		WindGustsMS:     domain.Some(16.0),
		IsDay:           domain.Some(true),
	}

	message := Current(domain.Location{Name: "Дрезден"}, current, Options{Printer: i18n.For(i18n.Russian)})
	if !strings.Contains(message, "Осадки 1.2 мм") {
		t.Errorf("нет строки об осадках:\n%s", message)
	}
	if !strings.Contains(message, "порывы 16") {
		t.Errorf("нет предупреждения о порывах:\n%s", message)
	}
}

func TestParseLayout(t *testing.T) {
	tests := []struct {
		raw  string
		want Layout
	}{
		{raw: "lines", want: LayoutLines},
		{raw: "TABLE", want: LayoutTable},
		{raw: " table ", want: LayoutTable},
	}
	for _, tc := range tests {
		got, err := ParseLayout(tc.raw)
		if err != nil || got != tc.want {
			t.Errorf("ParseLayout(%q) = %q, %v", tc.raw, got, err)
		}
	}
	if _, err := ParseLayout("json"); err == nil {
		t.Error("expected an error для неизвестного варианта")
	}
}

func TestTrimToLimit(t *testing.T) {
	long := strings.Repeat("я", MessageLimit+100)
	trimmed := trimToLimit(long)
	if length := len([]rune(trimmed)); length != MessageLimit {
		t.Errorf("длина после обрезки = %d, want %d", length, MessageLimit)
	}
	if !strings.HasSuffix(trimmed, "…") {
		t.Error("обрезанное сообщение должно заканчиваться многоточием")
	}
}

func TestReportRendersEveryLanguage(t *testing.T) {
	for _, lang := range i18n.Supported {
		message := Report(rainyWindyDay(t), Options{Printer: i18n.For(lang)})
		if strings.Contains(message, "headline.") || strings.Contains(message, "advice.") {
			t.Errorf("%q: в сообщении остался непереведённый ключ:\n%s", lang, message)
		}
		if !strings.Contains(message, i18n.For(i18n.Russian).T(i18n.KeyAttribution)) {
			t.Errorf("%q: нет атрибуции", lang)
		}
	}
}

func TestReportFallsBackToDefaultPrinter(t *testing.T) {
	message := Report(sunnyDay(t), Options{})
	if !strings.Contains(message, "What to wear") {
		t.Errorf("без принтера want английский:\n%s", message)
	}
}
