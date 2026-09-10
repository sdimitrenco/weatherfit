package i18n

import (
	"strings"
	"testing"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
)

func TestParse(t *testing.T) {
	tests := []struct {
		code string
		want Lang
	}{
		{code: "", want: Default},
		{code: "en", want: English},
		{code: "en-US", want: English},
		{code: "ru", want: Russian},
		{code: "ru-RU", want: Russian},
		{code: "RU", want: Russian},
		{code: "ru_RU", want: Russian},
		{code: " de ", want: German},
		{code: "de-AT", want: German},
		{code: "fr", want: Default},
		{code: "zh-Hans", want: Default},
	}

	for _, tc := range tests {
		if got := Parse(tc.code); got != tc.want {
			t.Errorf("Parse(%q) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestDefaultIsEnglish(t *testing.T) {
	if Default != English {
		t.Errorf("язык по умолчанию = %q, want английский", Default)
	}
}

func TestCatalogsCoverEveryKey(t *testing.T) {
	for lang, catalog := range catalogs {
		if lang == English {
			continue
		}
		for key := range catalogEN {
			if _, ok := catalog[key]; !ok {
				t.Errorf("в каталоге %q нет ключа %q", lang, key)
			}
		}
		for key := range catalog {
			if _, ok := catalogEN[key]; !ok {
				t.Errorf("в каталоге %q лишний ключ %q", lang, key)
			}
		}
	}
}

func TestCatalogsKeepFormatPlaceholders(t *testing.T) {
	for lang, catalog := range catalogs {
		if lang == English {
			continue
		}
		for key, english := range catalogEN {
			if got := strings.Count(catalog[key], "%"); got != strings.Count(english, "%") {
				t.Errorf("ключ %q в каталоге %q имеет другое число подстановок: %q против %q",
					key, lang, catalog[key], english)
			}
		}
	}
}

func TestMissingKeyFallsBack(t *testing.T) {
	printer := For(Russian)
	if got := printer.T(Key("nope.nope")); got != "nope.nope" {
		t.Errorf("для неизвестного ключа = %q", got)
	}
}

func TestForUnknownLanguageFallsBack(t *testing.T) {
	if For(Lang("fr")).Lang() != Default {
		t.Error("неподдерживаемый язык должен падать на язык по умолчанию")
	}
}

func TestWeekdayAndMonth(t *testing.T) {
	date := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)

	tests := map[Lang]string{
		English: "Thursday, 10 September",
		Russian: "четверг, 10 сентября",
		German:  "Donnerstag, 10 September",
	}
	for lang, want := range tests {
		if got := For(lang).Date(date); got != want {
			t.Errorf("%q: дата = %q, want %q", lang, got, want)
		}
	}
}

func TestRoseAndWindLevel(t *testing.T) {
	point, _ := domain.CompassFor(domain.Some(225)).Get()

	if got := For(English).Rose(point.Rose); got != "SW" {
		t.Errorf("румб на английском = %q", got)
	}
	if got := For(Russian).Rose(point.Rose); got != "ЮЗ" {
		t.Errorf("румб на русском = %q", got)
	}
	if got := For(German).Rose(point.Rose); got != "SW" {
		t.Errorf("румб на немецком = %q", got)
	}
	if got := For(Russian).WindLevel(domain.WindStrong); got != "сильный" {
		t.Errorf("уровень ветра = %q", got)
	}
}

func TestCondition(t *testing.T) {
	condition := domain.ConditionFor(domain.Some(95), domain.Some(true))
	tests := map[Lang]string{English: "thunderstorm", Russian: "гроза", German: "Gewitter"}
	for lang, want := range tests {
		if got := For(lang).Condition(condition); got != want {
			t.Errorf("%q: описание = %q, want %q", lang, got, want)
		}
	}

	unknown := domain.ConditionFor(domain.Some(42), domain.Some(true))
	if got := For(Russian).Condition(unknown); got != "нет данных" {
		t.Errorf("неизвестный код = %q", got)
	}
}

func TestRainWindows(t *testing.T) {
	base := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	windows := []domain.RainWindow{
		{From: base.Add(14 * time.Hour), To: base.Add(17 * time.Hour)},
	}

	if got := For(English).RainWindows(windows); got != "14–17 h" {
		t.Errorf("окно на английском = %q", got)
	}
	if got := For(Russian).RainWindows(windows); got != "14–17 ч" {
		t.Errorf("окно на русском = %q", got)
	}
	if got := For(German).RainWindows(windows); got != "14–17 Uhr" {
		t.Errorf("окно на немецком = %q", got)
	}

	two := []domain.RainWindow{
		windows[0],
		{From: base.Add(20 * time.Hour), To: base.Add(20 * time.Hour)},
	}
	if got := For(Russian).RainWindows(two); got != "14–17 и 20 ч" {
		t.Errorf("два окна = %q", got)
	}
	if For(Russian).RainWindows(nil) != "" {
		t.Error("без окон want пустая строка")
	}
}

func TestAdviceText(t *testing.T) {
	advice := domain.OutfitAdvice{Items: []domain.AdviceItem{
		{Kind: domain.AdviceBase, Band: domain.BandHoodieJacket, TemperatureC: domain.Some(12.0)},
		{Kind: domain.AdviceRainCoat},
		{Kind: domain.AdviceWindproofGusts, WindLevel: domain.WindStrong, SpeedMS: domain.Some(16.0)},
	}}

	russian := For(Russian).AdviceText(advice, domain.WindUnitMS)
	for _, want := range []string{"около 12°", "худи и лёгкая куртка", "дождевик", "порывы до 16"} {
		if !strings.Contains(russian, want) {
			t.Errorf("в русском совете нет %q:\n%s", want, russian)
		}
	}

	english := For(English).AdviceText(advice, domain.WindUnitMS)
	for _, want := range []string{"Around 12°", "hoodie", "rain jacket", "gusts up to 16"} {
		if !strings.Contains(english, want) {
			t.Errorf("в английском совете нет %q:\n%s", want, english)
		}
	}

	german := For(German).AdviceText(advice, domain.WindUnitMS)
	for _, want := range []string{"gefühlt etwa 12°", "Hoodie", "Regenjacke", "Böen bis 16"} {
		if !strings.Contains(german, want) {
			t.Errorf("в немецком совете нет %q:\n%s", want, german)
		}
	}
}

func TestAdviceUsesConfiguredUnit(t *testing.T) {
	advice := domain.OutfitAdvice{Items: []domain.AdviceItem{
		{Kind: domain.AdviceWindproofGusts, WindLevel: domain.WindStrong, SpeedMS: domain.Some(16.0)},
	}}
	if got := For(English).AdviceText(advice, domain.WindUnitKMH); !strings.Contains(got, "58") {
		t.Errorf("в км/ч want 58: %q", got)
	}
}

func TestHeadline(t *testing.T) {
	base := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	windows := []domain.RainWindow{{From: base.Add(14 * time.Hour), To: base.Add(17 * time.Hour)}}

	tests := []struct {
		lang     Lang
		headline domain.Headline
		want     string
	}{
		{lang: English, headline: domain.Headline{Kind: domain.HeadlineRainCoat, Windows: windows}, want: "Take a rain jacket — rain 14–17 h"},
		{lang: Russian, headline: domain.Headline{Kind: domain.HeadlineRainCoat, Windows: windows}, want: "Бери дождевик — дождь 14–17 ч"},
		{lang: German, headline: domain.Headline{Kind: domain.HeadlineRainUmbrella, Windows: windows}, want: "Regenschirm einpacken — Regen 14–17 Uhr"},
		{lang: Russian, headline: domain.Headline{Kind: domain.HeadlineDry}, want: "Дождя не ожидается"},
		{lang: Russian, headline: domain.Headline{Kind: domain.HeadlineSnow}, want: "Снег, нужна зимняя обувь"},
		{lang: Russian, headline: domain.Headline{Kind: domain.HeadlineSnow, Windows: windows}, want: "Снег 14–17 ч, нужна зимняя обувь"},
		{
			lang:     English,
			headline: domain.Headline{Kind: domain.HeadlineOutfit, Band: domain.BandTShirtPants, TemperatureC: domain.Some(19.0)},
			want:     "Morning ~19°: t-shirt and light trousers",
		},
	}

	for _, tc := range tests {
		if got := For(tc.lang).Headline(tc.headline); got != tc.want {
			t.Errorf("%q: строка = %q, want %q", tc.lang, got, tc.want)
		}
	}
}

func TestTemperature(t *testing.T) {
	printer := For(English)
	if got := printer.Temperature(domain.Some(13.6)); got != "14°" {
		t.Errorf("температура = %q, want 14°", got)
	}
	if got := printer.Temperature(domain.Some(-0.2)); got != "0°" {
		t.Errorf("температура = %q, want 0° без минуса", got)
	}
	if got := printer.Temperature(domain.None[float64]()); got != Missing {
		t.Errorf("отсутствующая температура = %q", got)
	}
}

func TestNumber(t *testing.T) {
	tests := []struct {
		value    float64
		decimals int
		want     string
	}{
		{value: 14.5, decimals: 0, want: "15"},
		{value: 13.4, decimals: 0, want: "13"},
		{value: -0.4, decimals: 0, want: "0"},
		{value: 1.24, decimals: 1, want: "1.2"},
		{value: 2, decimals: 1, want: "2.0"},
	}
	for _, tc := range tests {
		if got := Number(tc.value, tc.decimals); got != tc.want {
			t.Errorf("Number(%v, %d) = %q, want %q", tc.value, tc.decimals, got, tc.want)
		}
	}
}

func TestSupportedLanguagesHaveNames(t *testing.T) {
	for _, lang := range Supported {
		if Name(lang) == string(lang) {
			t.Errorf("для языка %q нет человекочитаемого названия", lang)
		}
		if !Valid(lang) {
			t.Errorf("язык %q должен считаться поддерживаемым", lang)
		}
	}
	if Valid(Lang("fr")) {
		t.Error("французский пока не поддерживается")
	}
}
