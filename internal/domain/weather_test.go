package domain

import "testing"

func TestConditionForKnownCodes(t *testing.T) {
	tests := []struct {
		code int
		icon string
		kind ConditionKind
	}{
		{code: 0, icon: "☀️", kind: ConditionClear},
		{code: 1, icon: "🌤", kind: ConditionMostlyClear},
		{code: 2, icon: "⛅", kind: ConditionPartlyCloudy},
		{code: 3, icon: "☁️", kind: ConditionOvercast},
		{code: 45, icon: "🌫", kind: ConditionFog},
		{code: 48, icon: "🌫", kind: ConditionFog},
		{code: 51, icon: "🌦", kind: ConditionDrizzle},
		{code: 53, icon: "🌦", kind: ConditionDrizzle},
		{code: 56, icon: "🌧🧊", kind: ConditionFreezingDrizzle},
		{code: 57, icon: "🌧🧊", kind: ConditionFreezingDrizzle},
		{code: 61, icon: "🌧", kind: ConditionLightRain},
		{code: 63, icon: "🌧", kind: ConditionRain},
		{code: 65, icon: "🌧", kind: ConditionHeavyRain},
		{code: 66, icon: "🌧🧊", kind: ConditionFreezingRain},
		{code: 67, icon: "🌧🧊", kind: ConditionFreezingRain},
		{code: 71, icon: "❄️", kind: ConditionLightSnow},
		{code: 75, icon: "❄️", kind: ConditionHeavySnow},
		{code: 77, icon: "❄️", kind: ConditionSnowGrains},
		{code: 80, icon: "🌦", kind: ConditionRainShowers},
		{code: 82, icon: "🌦", kind: ConditionHeavyRainShowers},
		{code: 85, icon: "🌨", kind: ConditionSnowShowers},
		{code: 86, icon: "🌨", kind: ConditionHeavySnowShowers},
		{code: 95, icon: "⛈", kind: ConditionThunderstorm},
		{code: 96, icon: "⛈", kind: ConditionThunderstormHail},
		{code: 99, icon: "⛈", kind: ConditionThunderstormHail},
	}

	for _, tc := range tests {
		condition := ConditionFor(Some(tc.code), Some(true))
		if !condition.Known {
			t.Errorf("код %d должен быть известен", tc.code)
		}
		if condition.Icon != tc.icon {
			t.Errorf("иконка кода %d = %q, want %q", tc.code, condition.Icon, tc.icon)
		}
		if condition.Kind != tc.kind {
			t.Errorf("вид кода %d = %q, want %q", tc.code, condition.Kind, tc.kind)
		}
		if condition.Code != tc.code {
			t.Errorf("код = %d, want %d", condition.Code, tc.code)
		}
	}
}

func TestConditionForClearSkyAtNight(t *testing.T) {
	if got := ConditionFor(Some(0), Some(false)).Icon; got != "🌙" {
		t.Errorf("ночная иконка = %q, want 🌙", got)
	}
	if got := ConditionFor(Some(0), Some(true)).Icon; got != "☀️" {
		t.Errorf("дневная иконка = %q, want ☀️", got)
	}
	if got := ConditionFor(Some(0), None[bool]()).Icon; got != "☀️" {
		t.Errorf("без is_day иконка = %q, want ☀️", got)
	}
	if got := ConditionFor(Some(3), Some(false)).Icon; got != "☁️" {
		t.Errorf("пасмурно ночью = %q, want ☁️", got)
	}
}

func TestConditionForUnknownAndMissing(t *testing.T) {
	unknown := ConditionFor(Some(42), Some(true))
	if unknown.Known {
		t.Error("код 42 не должен считаться известным")
	}
	if unknown.Icon != "❔" || unknown.Code != 42 {
		t.Errorf("неизвестный код = %+v", unknown)
	}

	missing := ConditionFor(None[int](), Some(true))
	if missing.Known || missing.Icon != "❔" {
		t.Errorf("отсутствующий код = %+v", missing)
	}
}

func TestHazardPredicates(t *testing.T) {
	tests := []struct {
		code    int
		thunder bool
		freeze  bool
		snow    bool
	}{
		{code: 0},
		{code: 56, freeze: true},
		{code: 57, freeze: true},
		{code: 66, freeze: true},
		{code: 67, freeze: true},
		{code: 71, snow: true},
		{code: 73, snow: true},
		{code: 75, snow: true},
		{code: 77, snow: true},
		{code: 85, snow: true},
		{code: 86, snow: true},
		{code: 94},
		{code: 95, thunder: true},
		{code: 96, thunder: true},
		{code: 99, thunder: true},
		{code: 100},
	}

	for _, tc := range tests {
		if got := IsThunderstorm(Some(tc.code)); got != tc.thunder {
			t.Errorf("IsThunderstorm(%d) = %v, want %v", tc.code, got, tc.thunder)
		}
		if got := IsFreezing(Some(tc.code)); got != tc.freeze {
			t.Errorf("IsFreezing(%d) = %v, want %v", tc.code, got, tc.freeze)
		}
		if got := IsSnow(Some(tc.code)); got != tc.snow {
			t.Errorf("IsSnow(%d) = %v, want %v", tc.code, got, tc.snow)
		}
	}

	if IsThunderstorm(None[int]()) || IsFreezing(None[int]()) || IsSnow(None[int]()) {
		t.Error("для отсутствующего кода предикаты должны быть false")
	}
}
