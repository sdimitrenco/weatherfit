package domain

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Пороги для советов по одежде.
const (
	LayeringSpreadC   = 8.0
	UVGlassesIndex    = 3.0
	UVSunscreenIndex  = 6.0
	maxAdviceSentence = 4
)

type outfitBand struct {
	fromApparentC float64
	kit           string
	outerLayer    string
}

var outfitBands = []outfitBand{
	{fromApparentC: 25, kit: "футболка, шорты и головной убор"},
	{fromApparentC: 18, kit: "футболка и лёгкие брюки"},
	{fromApparentC: 12, kit: "худи и лёгкая куртка", outerLayer: "куртку"},
	{fromApparentC: 5, kit: "свитер и демисезонная куртка", outerLayer: "куртку"},
	{fromApparentC: 0, kit: "тёплая куртка и шапка", outerLayer: "шапку"},
	{fromApparentC: -10, kit: "зимняя куртка, шапка, шарф и перчатки", outerLayer: "шарф"},
	{fromApparentC: math.Inf(-1), kit: "пуховик, термобельё и всё тёплое"},
}

// OutfitInput — данные, по которым собирается совет.
type OutfitInput struct {
	ApparentMinC  Opt[float64]
	ApparentMaxC  Opt[float64]
	Wind          WindSummary
	Precipitation PrecipitationAnalysis
	UVIndexMax    Opt[float64]
	// RainWindowText — человекочитаемое окно осадков, например «14–17 ч».
	RainWindowText string
}

// OutfitAdvice — готовый совет: от двух до четырёх коротких предложений.
type OutfitAdvice struct {
	Sentences []string
}

// Text склеивает предложения в один абзац.
func (a OutfitAdvice) Text() string {
	return strings.Join(a.Sentences, " ")
}

type advicePart struct {
	order    int
	priority int
	text     string
}

// BuildOutfitAdvice собирает совет по одежде из шаблонов.
func BuildOutfitAdvice(in OutfitInput) OutfitAdvice {
	var parts []advicePart
	add := func(order, priority int, text string) {
		parts = append(parts, advicePart{order: order, priority: priority, text: text})
	}

	band := bandFor(in.ApparentMinC)
	add(1, 1, baseSentence(in.ApparentMinC, band))

	if sentence, ok := layeringSentence(in, band); ok {
		add(2, 4, sentence)
	}
	if sentence, ok := rainSentence(in); ok {
		add(3, 2, sentence)
	}
	if sentence, ok := hazardSentence(in); ok {
		add(4, 3, sentence)
	}
	if sentence, ok := windSentence(in); ok {
		add(5, 5, sentence)
	}
	if sentence, ok := uvSentence(in); ok {
		add(6, 6, sentence)
	}

	return OutfitAdvice{Sentences: pickSentences(parts)}
}

func pickSentences(parts []advicePart) []string {
	sort.SliceStable(parts, func(i, j int) bool { return parts[i].priority < parts[j].priority })
	if len(parts) > maxAdviceSentence {
		parts = parts[:maxAdviceSentence]
	}
	sort.SliceStable(parts, func(i, j int) bool { return parts[i].order < parts[j].order })

	sentences := make([]string, 0, len(parts))
	for _, part := range parts {
		sentences = append(sentences, part.text)
	}
	return sentences
}

func bandFor(apparentMin Opt[float64]) outfitBand {
	value, ok := apparentMin.Get()
	if !ok {
		return outfitBands[2]
	}
	for _, band := range outfitBands {
		if value >= band.fromApparentC {
			return band
		}
	}
	return outfitBands[len(outfitBands)-1]
}

func baseSentence(apparentMin Opt[float64], band outfitBand) string {
	value, ok := apparentMin.Get()
	if !ok {
		return fmt.Sprintf("Температуры в ответе нет, по облачности ориентируйся сам: %s.", band.kit)
	}
	return fmt.Sprintf("Утром ощущается около %s — %s.", degrees(value), band.kit)
}

func layeringSentence(in OutfitInput, band outfitBand) (string, bool) {
	minimum, hasMin := in.ApparentMinC.Get()
	maximum, hasMax := in.ApparentMaxC.Get()
	if !hasMin || !hasMax || maximum-minimum < LayeringSpreadC {
		return "", false
	}
	if band.outerLayer == "" {
		return fmt.Sprintf("Днём до %s, одевайся слоями.", degrees(maximum)), true
	}
	return fmt.Sprintf("Днём до %s, одевайся слоями: %s можно снять.", degrees(maximum), band.outerLayer), true
}

func rainSentence(in OutfitInput) (string, bool) {
	window := ""
	if in.RainWindowText != "" {
		window = " " + in.RainWindowText
	}

	switch in.Precipitation.Verdict {
	case RainNotNeeded:
		return "", false
	case RainJustInCase:
		return fmt.Sprintf("Дождь возможен%s — зонт в сумку на всякий случай.", window), true
	case RainRequired:
		var sentence string
		if in.Precipitation.UmbrellaUseless(in.Wind) {
			sentence = fmt.Sprintf("Дождь%s при сильном ветре — дождевик или куртка с капюшоном, зонт бесполезен.", window)
		} else {
			sentence = fmt.Sprintf("Дождь%s — зонт обязательно.", window)
		}
		if in.Precipitation.Heavy {
			sentence += " Обувь лучше непромокаемую."
		}
		return sentence, true
	default:
		return "", false
	}
}

func hazardSentence(in OutfitInput) (string, bool) {
	var hazards []string
	if in.Precipitation.Thunderstorm {
		hazards = append(hazards, "ожидается гроза ⛈, лучше не планировать долгие прогулки")
	}
	if in.Precipitation.Freezing {
		hazards = append(hazards, "возможен гололёд, обувь с цепким протектором")
	}
	if in.Precipitation.Snow {
		hazards = append(hazards, "будет снег, нужна зимняя обувь")
	}
	if len(hazards) == 0 {
		return "", false
	}
	return capitalize(strings.Join(hazards, "; ")) + ".", true
}

func windSentence(in OutfitInput) (string, bool) {
	if !in.Wind.Level.NeedsWindproof() && !in.Wind.GustWarning {
		return "", false
	}
	if gusts, ok := in.Wind.MaxGustsMS.Get(); ok && in.Wind.GustWarning {
		return fmt.Sprintf("Ветер %s, порывы до %.0f м/с — нужен ветрозащитный верх с капюшоном.",
			in.Wind.Level.Label(), gusts), true
	}
	return fmt.Sprintf("Ветер %s — нужен ветрозащитный верх.", in.Wind.Level.Label()), true
}

func uvSentence(in OutfitInput) (string, bool) {
	index, ok := in.UVIndexMax.Get()
	if !ok || index < UVGlassesIndex {
		return "", false
	}
	if index >= UVSunscreenIndex {
		return fmt.Sprintf("УФ %.0f — солнцезащитные очки, крем SPF и головной убор.", index), true
	}
	return fmt.Sprintf("УФ %.0f — пригодятся солнцезащитные очки.", index), true
}

func degrees(value float64) string {
	return fmt.Sprintf("%.0f°", value)
}

func capitalize(text string) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return text
	}
	return strings.ToUpper(string(runes[0])) + string(runes[1:])
}
