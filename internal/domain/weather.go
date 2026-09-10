package domain

// ConditionKind identifies a weather condition for translation lookup.
type ConditionKind string

const (
	ConditionUnknown          ConditionKind = "unknown"
	ConditionClear            ConditionKind = "clear"
	ConditionMostlyClear      ConditionKind = "mostly_clear"
	ConditionPartlyCloudy     ConditionKind = "partly_cloudy"
	ConditionOvercast         ConditionKind = "overcast"
	ConditionFog              ConditionKind = "fog"
	ConditionDrizzle          ConditionKind = "drizzle"
	ConditionHeavyDrizzle     ConditionKind = "heavy_drizzle"
	ConditionFreezingDrizzle  ConditionKind = "freezing_drizzle"
	ConditionLightRain        ConditionKind = "light_rain"
	ConditionRain             ConditionKind = "rain"
	ConditionHeavyRain        ConditionKind = "heavy_rain"
	ConditionFreezingRain     ConditionKind = "freezing_rain"
	ConditionLightSnow        ConditionKind = "light_snow"
	ConditionSnow             ConditionKind = "snow"
	ConditionHeavySnow        ConditionKind = "heavy_snow"
	ConditionSnowGrains       ConditionKind = "snow_grains"
	ConditionRainShowers      ConditionKind = "rain_showers"
	ConditionHeavyRainShowers ConditionKind = "heavy_rain_showers"
	ConditionSnowShowers      ConditionKind = "snow_showers"
	ConditionHeavySnowShowers ConditionKind = "heavy_snow_showers"
	ConditionThunderstorm     ConditionKind = "thunderstorm"
	ConditionThunderstormHail ConditionKind = "thunderstorm_hail"
)

// Condition holds the icon and the translation key for a WMO weather code.
type Condition struct {
	Code  int
	Icon  string
	Kind  ConditionKind
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
)

var conditions = map[int]Condition{
	0:  {Icon: iconClearDay, Kind: ConditionClear},
	1:  {Icon: iconMostlyClear, Kind: ConditionMostlyClear},
	2:  {Icon: iconPartlyCloudy, Kind: ConditionPartlyCloudy},
	3:  {Icon: iconOvercast, Kind: ConditionOvercast},
	45: {Icon: iconFog, Kind: ConditionFog},
	48: {Icon: iconFog, Kind: ConditionFog},
	51: {Icon: iconDrizzle, Kind: ConditionDrizzle},
	53: {Icon: iconDrizzle, Kind: ConditionDrizzle},
	55: {Icon: iconDrizzle, Kind: ConditionHeavyDrizzle},
	56: {Icon: iconFreezing, Kind: ConditionFreezingDrizzle},
	57: {Icon: iconFreezing, Kind: ConditionFreezingDrizzle},
	61: {Icon: iconRain, Kind: ConditionLightRain},
	63: {Icon: iconRain, Kind: ConditionRain},
	65: {Icon: iconRain, Kind: ConditionHeavyRain},
	66: {Icon: iconFreezing, Kind: ConditionFreezingRain},
	67: {Icon: iconFreezing, Kind: ConditionFreezingRain},
	71: {Icon: iconSnow, Kind: ConditionLightSnow},
	73: {Icon: iconSnow, Kind: ConditionSnow},
	75: {Icon: iconSnow, Kind: ConditionHeavySnow},
	77: {Icon: iconSnow, Kind: ConditionSnowGrains},
	80: {Icon: iconShowers, Kind: ConditionRainShowers},
	81: {Icon: iconShowers, Kind: ConditionRainShowers},
	82: {Icon: iconShowers, Kind: ConditionHeavyRainShowers},
	85: {Icon: iconSnowShowers, Kind: ConditionSnowShowers},
	86: {Icon: iconSnowShowers, Kind: ConditionHeavySnowShowers},
	95: {Icon: iconThunderstorm, Kind: ConditionThunderstorm},
	96: {Icon: iconThunderstorm, Kind: ConditionThunderstormHail},
	99: {Icon: iconThunderstorm, Kind: ConditionThunderstormHail},
}

// ConditionFor maps a WMO code to an icon and a translation key. Clear sky uses
// a moon icon at night. Unknown codes are reported with Known=false.
func ConditionFor(code Opt[int], isDay Opt[bool]) Condition {
	value, ok := code.Get()
	if !ok {
		return Condition{Icon: iconUnknown, Kind: ConditionUnknown}
	}

	condition, found := conditions[value]
	if !found {
		return Condition{Code: value, Icon: iconUnknown, Kind: ConditionUnknown}
	}

	condition.Code = value
	condition.Known = true
	if value == 0 && !isDay.Or(true) {
		condition.Icon = iconClearNight
	}
	return condition
}

// IsThunderstorm reports a thunderstorm code.
func IsThunderstorm(code Opt[int]) bool {
	value, ok := code.Get()
	return ok && value >= 95 && value <= 99
}

// IsFreezing reports freezing precipitation, meaning a risk of black ice.
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

// IsSnow reports a snow code.
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
