package domain

// Condition — иконка и текст для кода погоды WMO.
type Condition struct {
	Code  int
	Icon  string
	Text  string
	Known bool
}

const (
	iconClearDay     = "☀️"
	iconClearNight   = "🌙"
	iconMostlyClear  = "🌤"
	iconPartlyCloudy = "⛅"
	iconOvercast     = "☁️"
	iconFog          = "🌫"
	iconDrizzle      = "🌦"
	iconFreezing     = "🌧🧊"
	iconRain         = "🌧"
	iconShowers      = "🌦"
	iconSnow         = "❄️"
	iconSnowShowers  = "🌨"
	iconThunderstorm = "⛈"
	iconUnknown      = "❔"
	textUnknown      = "нет данных"
)

var conditions = map[int]Condition{
	0:  {Icon: iconClearDay, Text: "ясно"},
	1:  {Icon: iconMostlyClear, Text: "преимущественно ясно"},
	2:  {Icon: iconPartlyCloudy, Text: "переменная облачность"},
	3:  {Icon: iconOvercast, Text: "пасмурно"},
	45: {Icon: iconFog, Text: "туман"},
	48: {Icon: iconFog, Text: "туман"},
	51: {Icon: iconDrizzle, Text: "морось"},
	53: {Icon: iconDrizzle, Text: "морось"},
	55: {Icon: iconDrizzle, Text: "сильная морось"},
	56: {Icon: iconFreezing, Text: "ледяная морось"},
	57: {Icon: iconFreezing, Text: "ледяная морось"},
	61: {Icon: iconRain, Text: "слабый дождь"},
	63: {Icon: iconRain, Text: "дождь"},
	65: {Icon: iconRain, Text: "сильный дождь"},
	66: {Icon: iconFreezing, Text: "ледяной дождь"},
	67: {Icon: iconFreezing, Text: "ледяной дождь"},
	71: {Icon: iconSnow, Text: "слабый снег"},
	73: {Icon: iconSnow, Text: "снег"},
	75: {Icon: iconSnow, Text: "сильный снег"},
	77: {Icon: iconSnow, Text: "снежные зёрна"},
	80: {Icon: iconShowers, Text: "ливневый дождь"},
	81: {Icon: iconShowers, Text: "ливневый дождь"},
	82: {Icon: iconShowers, Text: "сильный ливень"},
	85: {Icon: iconSnowShowers, Text: "снегопад"},
	86: {Icon: iconSnowShowers, Text: "сильный снегопад"},
	95: {Icon: iconThunderstorm, Text: "гроза"},
	96: {Icon: iconThunderstorm, Text: "гроза с градом"},
	99: {Icon: iconThunderstorm, Text: "гроза с градом"},
}

// ConditionFor возвращает иконку и текст по коду WMO. Для ясного неба иконка
// зависит от того, день сейчас или ночь. Неизвестный код помечается Known=false.
func ConditionFor(code Opt[int], isDay Opt[bool]) Condition {
	value, ok := code.Get()
	if !ok {
		return Condition{Icon: iconUnknown, Text: textUnknown}
	}

	condition, found := conditions[value]
	if !found {
		return Condition{Code: value, Icon: iconUnknown, Text: textUnknown}
	}

	condition.Code = value
	condition.Known = true
	if value == 0 && !isDay.Or(true) {
		condition.Icon = iconClearNight
	}
	return condition
}

// IsThunderstorm сообщает, что код описывает грозу.
func IsThunderstorm(code Opt[int]) bool {
	value, ok := code.Get()
	return ok && value >= 95 && value <= 99
}

// IsFreezing сообщает, что код описывает ледяные осадки, то есть риск гололёда.
func IsFreezing(code Opt[int]) bool {
	value, ok := code.Get()
	if !ok {
		return false
	}
	switch value {
	case 56, 57, 66, 67:
		return true
	default:
		return false
	}
}

// IsSnow сообщает, что код описывает снег.
func IsSnow(code Opt[int]) bool {
	value, ok := code.Get()
	if !ok {
		return false
	}
	switch value {
	case 71, 73, 75, 77, 85, 86:
		return true
	default:
		return false
	}
}
