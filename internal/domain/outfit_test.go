package domain

import (
	"strings"
	"testing"
)

func TestBuildOutfitAdviceBands(t *testing.T) {
	tests := []struct {
		apparent float64
		mustSay  string
	}{
		{apparent: 30, mustSay: "шорты"},
		{apparent: 25, mustSay: "шорты"},
		{apparent: 24, mustSay: "лёгкие брюки"},
		{apparent: 18, mustSay: "лёгкие брюки"},
		{apparent: 17, mustSay: "худи"},
		{apparent: 12, mustSay: "худи"},
		{apparent: 11, mustSay: "свитер"},
		{apparent: 5, mustSay: "свитер"},
		{apparent: 4, mustSay: "тёплая куртка"},
		{apparent: 0, mustSay: "тёплая куртка"},
		{apparent: -1, mustSay: "зимняя куртка"},
		{apparent: -10, mustSay: "зимняя куртка"},
		{apparent: -11, mustSay: "пуховик"},
		{apparent: -30, mustSay: "пуховик"},
	}

	for _, tc := range tests {
		advice := BuildOutfitAdvice(OutfitInput{
			ApparentMinC: Some(tc.apparent),
			ApparentMaxC: Some(tc.apparent),
		})
		if !strings.Contains(advice.Text(), tc.mustSay) {
			t.Errorf("для %v° совет %q не содержит %q", tc.apparent, advice.Text(), tc.mustSay)
		}
	}
}

func TestBuildOutfitAdviceSentenceCount(t *testing.T) {
	advice := BuildOutfitAdvice(OutfitInput{
		ApparentMinC:   Some[float64](11),
		ApparentMaxC:   Some[float64](21),
		Wind:           WindSummary{Level: WindStrong, MaxGustsMS: Some[float64](16), GustWarning: true},
		Precipitation:  PrecipitationAnalysis{Verdict: RainRequired, Heavy: true, Thunderstorm: true},
		UVIndexMax:     Some[float64](7),
		RainWindowText: "14–17 ч",
	})

	if len(advice.Sentences) < 2 || len(advice.Sentences) > 4 {
		t.Errorf("предложений = %d, ожидалось от 2 до 4:\n%s", len(advice.Sentences), advice.Text())
	}
	if !strings.Contains(advice.Text(), "дождевик") {
		t.Errorf("совет должен упоминать дождевик:\n%s", advice.Text())
	}
	if !strings.Contains(advice.Text(), "гроза") {
		t.Errorf("совет должен упоминать грозу:\n%s", advice.Text())
	}
}

func TestBuildOutfitAdviceLayering(t *testing.T) {
	withSpread := BuildOutfitAdvice(OutfitInput{ApparentMinC: Some[float64](12), ApparentMaxC: Some[float64](20)})
	if !strings.Contains(withSpread.Text(), "слоями") {
		t.Errorf("при разнице 8° нужен совет про слои:\n%s", withSpread.Text())
	}
	if !strings.Contains(withSpread.Text(), "куртку можно снять") {
		t.Errorf("нужен верхний слой, который можно снять:\n%s", withSpread.Text())
	}

	withoutSpread := BuildOutfitAdvice(OutfitInput{ApparentMinC: Some[float64](12), ApparentMaxC: Some[float64](19.9)})
	if strings.Contains(withoutSpread.Text(), "слоями") {
		t.Errorf("при разнице меньше 8° совета про слои быть не должно:\n%s", withoutSpread.Text())
	}
}

func TestBuildOutfitAdviceRainVariants(t *testing.T) {
	tests := []struct {
		name    string
		input   OutfitInput
		mustSay string
		mustNot string
	}{
		{
			name: "сухо",
			input: OutfitInput{
				ApparentMinC: Some[float64](15), ApparentMaxC: Some[float64](18),
				Precipitation: PrecipitationAnalysis{Verdict: RainNotNeeded},
			},
			mustNot: "зонт",
		},
		{
			name: "зонт на всякий случай",
			input: OutfitInput{
				ApparentMinC: Some[float64](15), ApparentMaxC: Some[float64](18),
				Precipitation:  PrecipitationAnalysis{Verdict: RainJustInCase},
				RainWindowText: "14–15 ч",
			},
			mustSay: "на всякий случай",
		},
		{
			name: "зонт обязательно",
			input: OutfitInput{
				ApparentMinC: Some[float64](15), ApparentMaxC: Some[float64](18),
				Precipitation:  PrecipitationAnalysis{Verdict: RainRequired},
				Wind:           WindSummary{Level: WindLight},
				RainWindowText: "14–17 ч",
			},
			mustSay: "зонт обязательно",
		},
		{
			name: "дождевик вместо зонта при сильном ветре",
			input: OutfitInput{
				ApparentMinC: Some[float64](15), ApparentMaxC: Some[float64](18),
				Precipitation:  PrecipitationAnalysis{Verdict: RainRequired},
				Wind:           WindSummary{Level: WindStrong},
				RainWindowText: "14–17 ч",
			},
			mustSay: "зонт бесполезен",
		},
		{
			name: "непромокаемая обувь при сильном дожде",
			input: OutfitInput{
				ApparentMinC: Some[float64](15), ApparentMaxC: Some[float64](18),
				Precipitation: PrecipitationAnalysis{Verdict: RainRequired, Heavy: true},
				Wind:          WindSummary{Level: WindLight},
			},
			mustSay: "непромокаемую",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text := BuildOutfitAdvice(tc.input).Text()
			if tc.mustSay != "" && !strings.Contains(text, tc.mustSay) {
				t.Errorf("совет %q не содержит %q", text, tc.mustSay)
			}
			if tc.mustNot != "" && strings.Contains(text, tc.mustNot) {
				t.Errorf("совет %q не должен содержать %q", text, tc.mustNot)
			}
		})
	}
}

func TestBuildOutfitAdviceUV(t *testing.T) {
	tests := []struct {
		uv      float64
		mustSay string
		mustNot string
	}{
		{uv: 2, mustNot: "УФ"},
		{uv: 3, mustSay: "очки"},
		{uv: 5, mustSay: "очки"},
		{uv: 6, mustSay: "SPF"},
		{uv: 9, mustSay: "SPF"},
	}

	for _, tc := range tests {
		text := BuildOutfitAdvice(OutfitInput{
			ApparentMinC: Some[float64](20),
			ApparentMaxC: Some[float64](22),
			UVIndexMax:   Some(tc.uv),
		}).Text()
		if tc.mustSay != "" && !strings.Contains(text, tc.mustSay) {
			t.Errorf("УФ %v: совет %q не содержит %q", tc.uv, text, tc.mustSay)
		}
		if tc.mustNot != "" && strings.Contains(text, tc.mustNot) {
			t.Errorf("УФ %v: совет %q не должен содержать %q", tc.uv, text, tc.mustNot)
		}
	}
}

func TestBuildOutfitAdviceWind(t *testing.T) {
	strong := BuildOutfitAdvice(OutfitInput{
		ApparentMinC: Some[float64](15), ApparentMaxC: Some[float64](17),
		Wind: WindSummary{Level: WindStrong, MaxSpeedMS: Some[float64](9)},
	}).Text()
	if !strings.Contains(strong, "ветрозащитный") {
		t.Errorf("при сильном ветре нужен ветрозащитный верх:\n%s", strong)
	}

	gusty := BuildOutfitAdvice(OutfitInput{
		ApparentMinC: Some[float64](15), ApparentMaxC: Some[float64](17),
		Wind: WindSummary{Level: WindModerate, MaxGustsMS: Some[float64](16), GustWarning: true},
	}).Text()
	if !strings.Contains(gusty, "порывы до 16 м/с") {
		t.Errorf("порывы должны попасть в совет:\n%s", gusty)
	}

	calm := BuildOutfitAdvice(OutfitInput{
		ApparentMinC: Some[float64](15), ApparentMaxC: Some[float64](17),
		Wind: WindSummary{Level: WindLight},
	}).Text()
	if strings.Contains(calm, "ветрозащитный") {
		t.Errorf("при слабом ветре совета про ветрозащиту быть не должно:\n%s", calm)
	}
}

func TestBuildOutfitAdviceWithoutTemperature(t *testing.T) {
	advice := BuildOutfitAdvice(OutfitInput{
		ApparentMinC: None[float64](),
		ApparentMaxC: None[float64](),
	})
	if len(advice.Sentences) == 0 {
		t.Fatal("совет не должен быть пустым")
	}
	if !strings.Contains(advice.Text(), "Температуры в ответе нет") {
		t.Errorf("совет должен честно сказать про отсутствие данных:\n%s", advice.Text())
	}
}
