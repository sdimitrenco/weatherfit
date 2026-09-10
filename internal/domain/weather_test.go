package domain

import "testing"

func TestConditionForKnownCodes(t *testing.T) {
	tests := []struct {
		code int
		icon string
		text string
	}{
		{code: 0, icon: "☀️", text: "ясно"},
		{code: 1, icon: "🌤", text: "преимущественно ясно"},
		{code: 2, icon: "⛅", text: "переменная облачность"},
		{code: 3, icon: "☁️", text: "пасмурно"},
		{code: 45, icon: "🌫", text: "туман"},
		{code: 48, icon: "🌫", text: "туман"},
		{code: 51, icon: "🌦", text: "морось"},
		{code: 53, icon: "🌦", text: "морось"},
		{code: 56, icon: "🌧🧊", text: "ледяная морось"},
		{code: 57, icon: "🌧🧊", text: "ледяная морось"},
		{code: 61, icon: "🌧", text: "слабый дождь"},
		{code: 63, icon: "🌧", text: "дождь"},
		{code: 65, icon: "🌧", text: "сильный дождь"},
		{code: 66, icon: "🌧🧊", text: "ледяной дождь"},
		{code: 67, icon: "🌧🧊", text: "ледяной дождь"},
		{code: 71, icon: "❄️", text: "слабый снег"},
		{code: 75, icon: "❄️", text: "сильный снег"},
		{code: 77, icon: "❄️", text: "снежные зёрна"},
		{code: 80, icon: "🌦", text: "ливневый дождь"},
		{code: 82, icon: "🌦", text: "сильный ливень"},
		{code: 85, icon: "🌨", text: "снегопад"},
		{code: 86, icon: "🌨", text: "сильный снегопад"},
		{code: 95, icon: "⛈", text: "гроза"},
		{code: 96, icon: "⛈", text: "гроза с градом"},
		{code: 99, icon: "⛈", text: "гроза с градом"},
	}

	for _, tc := range tests {
		condition := ConditionFor(Some(tc.code), Some(true))
		if !condition.Known {
			t.Errorf("код %d должен быть известен", tc.code)
		}
		if condition.Icon != tc.icon {
			t.Errorf("иконка кода %d = %q, ожидалось %q", tc.code, condition.Icon, tc.icon)
		}
		if condition.Text != tc.text {
			t.Errorf("текст кода %d = %q, ожидалось %q", tc.code, condition.Text, tc.text)
		}
		if condition.Code != tc.code {
			t.Errorf("код = %d, ожидалось %d", condition.Code, tc.code)
		}
	}
}

func TestConditionForClearSkyAtNight(t *testing.T) {
	if got := ConditionFor(Some(0), Some(false)).Icon; got != "🌙" {
		t.Errorf("ночная иконка = %q, ожидалось 🌙", got)
	}
	if got := ConditionFor(Some(0), Some(true)).Icon; got != "☀️" {
		t.Errorf("дневная иконка = %q, ожидалось ☀️", got)
	}
	if got := ConditionFor(Some(0), None[bool]()).Icon; got != "☀️" {
		t.Errorf("без is_day иконка = %q, ожидалось ☀️", got)
	}
	if got := ConditionFor(Some(3), Some(false)).Icon; got != "☁️" {
		t.Errorf("пасмурно ночью = %q, ожидалось ☁️", got)
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
			t.Errorf("IsThunderstorm(%d) = %v, ожидалось %v", tc.code, got, tc.thunder)
		}
		if got := IsFreezing(Some(tc.code)); got != tc.freeze {
			t.Errorf("IsFreezing(%d) = %v, ожидалось %v", tc.code, got, tc.freeze)
		}
		if got := IsSnow(Some(tc.code)); got != tc.snow {
			t.Errorf("IsSnow(%d) = %v, ожидалось %v", tc.code, got, tc.snow)
		}
	}

	if IsThunderstorm(None[int]()) || IsFreezing(None[int]()) || IsSnow(None[int]()) {
		t.Error("для отсутствующего кода предикаты должны быть false")
	}
}
