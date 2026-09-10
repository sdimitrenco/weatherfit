package openmeteo

type response struct {
	Latitude         float64 `json:"latitude"`
	Longitude        float64 `json:"longitude"`
	Timezone         string  `json:"timezone"`
	UTCOffsetSeconds int     `json:"utc_offset_seconds"`
	Hourly           hourly  `json:"hourly"`
	Daily            daily   `json:"daily"`
}

type errorResponse struct {
	Error  bool   `json:"error"`
	Reason string `json:"reason"`
}

type hourly struct {
	Time                     []string   `json:"time"`
	Temperature2m            []*float64 `json:"temperature_2m"`
	ApparentTemperature      []*float64 `json:"apparent_temperature"`
	PrecipitationProbability []*int     `json:"precipitation_probability"`
	Precipitation            []*float64 `json:"precipitation"`
	Rain                     []*float64 `json:"rain"`
	Showers                  []*float64 `json:"showers"`
	Snowfall                 []*float64 `json:"snowfall"`
	WeatherCode              []*int     `json:"weather_code"`
	WindSpeed10m             []*float64 `json:"wind_speed_10m"`
	WindDirection10m         []*int     `json:"wind_direction_10m"`
	WindGusts10m             []*float64 `json:"wind_gusts_10m"`
	UVIndex                  []*float64 `json:"uv_index"`
	IsDay                    []*int     `json:"is_day"`
}

type daily struct {
	Time                        []string   `json:"time"`
	Temperature2mMax            []*float64 `json:"temperature_2m_max"`
	Temperature2mMin            []*float64 `json:"temperature_2m_min"`
	ApparentTemperatureMax      []*float64 `json:"apparent_temperature_max"`
	ApparentTemperatureMin      []*float64 `json:"apparent_temperature_min"`
	PrecipitationSum            []*float64 `json:"precipitation_sum"`
	PrecipitationProbabilityMax []*int     `json:"precipitation_probability_max"`
	WindSpeed10mMax             []*float64 `json:"wind_speed_10m_max"`
	WindGusts10mMax             []*float64 `json:"wind_gusts_10m_max"`
	WindDirection10mDominant    []*int     `json:"wind_direction_10m_dominant"`
	UVIndexMax                  []*float64 `json:"uv_index_max"`
	Sunrise                     []*string  `json:"sunrise"`
	Sunset                      []*string  `json:"sunset"`
}
