package domain

import "testing"

func kinds(advice OutfitAdvice) []AdviceKind {
	result := make([]AdviceKind, 0, len(advice.Items))
	for _, item := range advice.Items {
		result = append(result, item.Kind)
	}
	return result
}

func hasKind(advice OutfitAdvice, kind AdviceKind) bool {
	for _, item := range advice.Items {
		if item.Kind == kind {
			return true
		}
	}
	return false
}

func itemOf(advice OutfitAdvice, kind AdviceKind) (AdviceItem, bool) {
	for _, item := range advice.Items {
		if item.Kind == kind {
			return item, true
		}
	}
	return AdviceItem{}, false
}

func TestBuildOutfitAdviceBands(t *testing.T) {
	tests := []struct {
		apparent float64
		want     BandID
	}{
		{apparent: 30, want: BandTShirtShorts},
		{apparent: 25, want: BandTShirtShorts},
		{apparent: 24, want: BandTShirtPants},
		{apparent: 18, want: BandTShirtPants},
		{apparent: 17, want: BandHoodieJacket},
		{apparent: 12, want: BandHoodieJacket},
		{apparent: 11, want: BandSweaterCoat},
		{apparent: 5, want: BandSweaterCoat},
		{apparent: 4, want: BandWarmCoat},
		{apparent: 0, want: BandWarmCoat},
		{apparent: -1, want: BandWinterCoat},
		{apparent: -10, want: BandWinterCoat},
		{apparent: -11, want: BandDownJacket},
		{apparent: -30, want: BandDownJacket},
	}

	for _, tc := range tests {
		advice := BuildOutfitAdvice(OutfitInput{
			ApparentMinC: Some(tc.apparent),
			ApparentMaxC: Some(tc.apparent),
		})
		base, ok := itemOf(advice, AdviceBase)
		if !ok {
			t.Fatalf("для %v° нет базового совета: %v", tc.apparent, kinds(advice))
		}
		if base.Band != tc.want {
			t.Errorf("для %v° комплект = %q, want %q", tc.apparent, base.Band, tc.want)
		}
		if temperature, has := base.TemperatureC.Get(); !has || temperature != tc.apparent {
			t.Errorf("для %v° температура в совете = %v", tc.apparent, temperature)
		}
	}
}

func TestBuildOutfitAdviceLimitsItems(t *testing.T) {
	advice := BuildOutfitAdvice(OutfitInput{
		ApparentMinC:  Some(11.0),
		ApparentMaxC:  Some(21.0),
		Wind:          WindSummary{Level: WindStrong, MaxGustsMS: Some(16.0), GustWarning: true},
		Precipitation: PrecipitationAnalysis{Verdict: RainRequired, Heavy: true, Thunderstorm: true, LiquidMM: 5},
		UVIndexMax:    Some(7.0),
	})

	if len(advice.Items) < 2 || len(advice.Items) > maxAdviceItems {
		t.Errorf("советов = %d, want от 2 до %d: %v", len(advice.Items), maxAdviceItems, kinds(advice))
	}
	if advice.Items[0].Kind != AdviceBase {
		t.Errorf("первым идёт %q, want базовый совет", advice.Items[0].Kind)
	}
	if !hasKind(advice, AdviceRainCoat) {
		t.Errorf("нет совета про дождевик: %v", kinds(advice))
	}
	if !hasKind(advice, AdviceHazardThunder) {
		t.Errorf("нет предупреждения о грозе: %v", kinds(advice))
	}
}

func TestBuildOutfitAdviceKeepsOrder(t *testing.T) {
	advice := BuildOutfitAdvice(OutfitInput{
		ApparentMinC:  Some(12.0),
		ApparentMaxC:  Some(22.0),
		Precipitation: PrecipitationAnalysis{Verdict: RainJustInCase, LiquidMM: 1},
	})

	got := kinds(advice)
	want := []AdviceKind{AdviceBase, AdviceLayeringWithOuter, AdviceRainMaybe}
	if len(got) != len(want) {
		t.Fatalf("советы = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("совет %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBuildOutfitAdviceLayering(t *testing.T) {
	withOuter := BuildOutfitAdvice(OutfitInput{ApparentMinC: Some(12.0), ApparentMaxC: Some(20.0)})
	item, ok := itemOf(withOuter, AdviceLayeringWithOuter)
	if !ok {
		t.Fatalf("при разнице 8° нужен совет про слои: %v", kinds(withOuter))
	}
	if maximum, has := item.TemperatureC.Get(); !has || maximum != 20 {
		t.Errorf("в совете про слои максимум = %v, want 20", maximum)
	}

	noOuter := BuildOutfitAdvice(OutfitInput{ApparentMinC: Some(25.0), ApparentMaxC: Some(34.0)})
	if !hasKind(noOuter, AdviceLayering) {
		t.Errorf("для летнего комплекта want совет без верхнего слоя: %v", kinds(noOuter))
	}

	tooFlat := BuildOutfitAdvice(OutfitInput{ApparentMinC: Some(12.0), ApparentMaxC: Some(19.9)})
	if hasKind(tooFlat, AdviceLayering) || hasKind(tooFlat, AdviceLayeringWithOuter) {
		t.Errorf("при разнице меньше 8° совета про слои быть не должно: %v", kinds(tooFlat))
	}
}

func TestBuildOutfitAdvicePrecipitation(t *testing.T) {
	tests := []struct {
		name   string
		input  OutfitInput
		want   AdviceKind
		absent AdviceKind
	}{
		{
			name:   "сухо",
			input:  OutfitInput{ApparentMinC: Some(15.0), ApparentMaxC: Some(18.0)},
			absent: AdviceRainUmbrella,
		},
		{
			name: "зонт на всякий случай",
			input: OutfitInput{
				ApparentMinC: Some(15.0), ApparentMaxC: Some(18.0),
				Precipitation: PrecipitationAnalysis{Verdict: RainJustInCase, LiquidMM: 1},
			},
			want: AdviceRainMaybe,
		},
		{
			name: "зонт обязательно",
			input: OutfitInput{
				ApparentMinC: Some(15.0), ApparentMaxC: Some(18.0),
				Precipitation: PrecipitationAnalysis{Verdict: RainRequired, LiquidMM: 3},
				Wind:          WindSummary{Level: WindLight},
			},
			want: AdviceRainUmbrella,
		},
		{
			name: "дождевик при сильном ветре",
			input: OutfitInput{
				ApparentMinC: Some(15.0), ApparentMaxC: Some(18.0),
				Precipitation: PrecipitationAnalysis{Verdict: RainRequired, LiquidMM: 3},
				Wind:          WindSummary{Level: WindStrong},
			},
			want: AdviceRainCoat,
		},
		{
			name: "непромокаемая обувь",
			input: OutfitInput{
				ApparentMinC: Some(15.0), ApparentMaxC: Some(18.0),
				Precipitation: PrecipitationAnalysis{Verdict: RainRequired, Heavy: true, LiquidMM: 12},
				Wind:          WindSummary{Level: WindLight},
			},
			want: AdviceWaterproofShoes,
		},
		{
			name: "снег вместо дождя",
			input: OutfitInput{
				ApparentMinC: Some(-5.0), ApparentMaxC: Some(-2.0),
				Precipitation: PrecipitationAnalysis{Verdict: RainRequired, Snow: true, SnowfallCM: 2},
			},
			want:   AdviceSnow,
			absent: AdviceRainUmbrella,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			advice := BuildOutfitAdvice(tc.input)
			if tc.want != "" && !hasKind(advice, tc.want) {
				t.Errorf("нет совета %q: %v", tc.want, kinds(advice))
			}
			if tc.absent != "" && hasKind(advice, tc.absent) {
				t.Errorf("совет %q лишний: %v", tc.absent, kinds(advice))
			}
		})
	}
}

func TestBuildOutfitAdviceUV(t *testing.T) {
	tests := []struct {
		uv   float64
		want AdviceKind
	}{
		{uv: 2},
		{uv: 3, want: AdviceUVGlasses},
		{uv: 5, want: AdviceUVGlasses},
		{uv: 6, want: AdviceUVSunscreen},
		{uv: 9, want: AdviceUVSunscreen},
	}

	for _, tc := range tests {
		advice := BuildOutfitAdvice(OutfitInput{
			ApparentMinC: Some(20.0),
			ApparentMaxC: Some(22.0),
			UVIndexMax:   Some(tc.uv),
		})
		if tc.want == "" {
			if hasKind(advice, AdviceUVGlasses) || hasKind(advice, AdviceUVSunscreen) {
				t.Errorf("УФ %v: совета про УФ быть не должно: %v", tc.uv, kinds(advice))
			}
			continue
		}
		item, ok := itemOf(advice, tc.want)
		if !ok {
			t.Errorf("УФ %v: нет совета %q: %v", tc.uv, tc.want, kinds(advice))
			continue
		}
		if index, has := item.UVIndex.Get(); !has || index != tc.uv {
			t.Errorf("УФ %v: индекс в совете = %v", tc.uv, index)
		}
	}
}

func TestBuildOutfitAdviceWind(t *testing.T) {
	strong := BuildOutfitAdvice(OutfitInput{
		ApparentMinC: Some(15.0), ApparentMaxC: Some(17.0),
		Wind: WindSummary{Level: WindStrong, MaxSpeedMS: Some(9.0)},
	})
	if !hasKind(strong, AdviceWindproof) {
		t.Errorf("при сильном ветре нужен ветрозащитный верх: %v", kinds(strong))
	}

	gusty := BuildOutfitAdvice(OutfitInput{
		ApparentMinC: Some(15.0), ApparentMaxC: Some(17.0),
		Wind: WindSummary{Level: WindModerate, MaxGustsMS: Some(16.0), GustWarning: true},
	})
	item, ok := itemOf(gusty, AdviceWindproofGusts)
	if !ok {
		t.Fatalf("порывы должны попасть в совет: %v", kinds(gusty))
	}
	if speed, has := item.SpeedMS.Get(); !has || speed != 16 {
		t.Errorf("скорость порывов в совете = %v, want 16", speed)
	}
	if item.WindLevel != WindModerate {
		t.Errorf("уровень ветра в совете = %v", item.WindLevel)
	}

	calm := BuildOutfitAdvice(OutfitInput{
		ApparentMinC: Some(15.0), ApparentMaxC: Some(17.0),
		Wind: WindSummary{Level: WindLight},
	})
	if hasKind(calm, AdviceWindproof) || hasKind(calm, AdviceWindproofGusts) {
		t.Errorf("при слабом ветре совета про ветрозащиту быть не должно: %v", kinds(calm))
	}
}

func TestBuildOutfitAdviceWithoutTemperature(t *testing.T) {
	advice := BuildOutfitAdvice(OutfitInput{})
	if !hasKind(advice, AdviceBaseNoTemperature) {
		t.Errorf("без температуры want честный совет: %v", kinds(advice))
	}
}
