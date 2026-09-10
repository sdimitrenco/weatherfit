// Package openmeteo — HTTP-клиент Open-Meteo Forecast API и маппинг ответа в домен.
package openmeteo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
)

const (
	// DefaultBaseURL — публичный endpoint Forecast API.
	DefaultBaseURL = "https://api.open-meteo.com/v1/forecast"
	// DefaultTimeout — таймаут одного запроса.
	DefaultTimeout = 10 * time.Second
	// DefaultUserAgent отправляется с каждым запросом.
	DefaultUserAgent = "weatherfit-bot/1.0 (personal telegram weather bot)"

	// WindUnitMS и WindUnitKMH — поддерживаемые значения wind_speed_unit.
	WindUnitMS  = "ms"
	WindUnitKMH = "kmh"

	kmhPerMS      = 3.6
	maxBodyBytes  = 4 << 20
	localTimeForm = "2006-01-02T15:04"
)

var hourlyVariables = []string{
	"temperature_2m",
	"apparent_temperature",
	"precipitation_probability",
	"precipitation",
	"rain",
	"showers",
	"snowfall",
	"weather_code",
	"wind_speed_10m",
	"wind_direction_10m",
	"wind_gusts_10m",
	"uv_index",
	"is_day",
}

var dailyVariables = []string{
	"temperature_2m_max",
	"temperature_2m_min",
	"apparent_temperature_max",
	"apparent_temperature_min",
	"precipitation_sum",
	"precipitation_probability_max",
	"wind_speed_10m_max",
	"wind_gusts_10m_max",
	"wind_direction_10m_dominant",
	"uv_index_max",
	"sunrise",
	"sunset",
}

// Options — параметры создания клиента.
type Options struct {
	Location   domain.Location
	Timezone   *time.Location
	WindUnit   string
	BaseURL    string
	UserAgent  string
	HTTPClient *http.Client
}

// Client запрашивает прогноз для одной точки.
type Client struct {
	location   domain.Location
	timezone   *time.Location
	windUnit   string
	baseURL    string
	userAgent  string
	httpClient *http.Client
}

// New создаёт клиент, проверяя обязательные параметры.
func New(options Options) (*Client, error) {
	if options.Timezone == nil {
		return nil, errors.New("openmeteo: не задана таймзона")
	}
	switch options.WindUnit {
	case WindUnitMS, WindUnitKMH:
	case "":
		options.WindUnit = WindUnitMS
	default:
		return nil, fmt.Errorf("openmeteo: единица ветра %q не поддерживается", options.WindUnit)
	}
	if options.BaseURL == "" {
		options.BaseURL = DefaultBaseURL
	}
	if options.UserAgent == "" {
		options.UserAgent = DefaultUserAgent
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: DefaultTimeout}
	}

	return &Client{
		location:   options.Location,
		timezone:   options.Timezone,
		windUnit:   options.WindUnit,
		baseURL:    options.BaseURL,
		userAgent:  options.UserAgent,
		httpClient: options.HTTPClient,
	}, nil
}

// Forecast запрашивает прогноз на days календарных дней начиная с сегодняшнего.
func (c *Client) Forecast(ctx context.Context, days int) (domain.Forecast, error) {
	if days < 1 || days > 16 {
		return domain.Forecast{}, fmt.Errorf("openmeteo: forecast_days=%d вне диапазона [1, 16]", days)
	}

	request, err := c.newRequest(ctx, days)
	if err != nil {
		return domain.Forecast{}, err
	}

	httpResponse, err := c.httpClient.Do(request)
	if err != nil {
		return domain.Forecast{}, fmt.Errorf("openmeteo: запрос не удался: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, httpResponse.Body)
		_ = httpResponse.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(httpResponse.Body, maxBodyBytes))
	if err != nil {
		return domain.Forecast{}, fmt.Errorf("openmeteo: не удалось прочитать ответ: %w", err)
	}

	if httpResponse.StatusCode != http.StatusOK {
		return domain.Forecast{}, fmt.Errorf("openmeteo: %w", statusError(httpResponse.StatusCode, body))
	}

	var parsed response
	if err := json.Unmarshal(body, &parsed); err != nil {
		return domain.Forecast{}, fmt.Errorf("openmeteo: не удалось разобрать ответ: %w", err)
	}

	return c.toDomain(parsed)
}

func (c *Client) newRequest(ctx context.Context, days int) (*http.Request, error) {
	query := url.Values{}
	query.Set("latitude", strconv.FormatFloat(c.location.Latitude, 'f', -1, 64))
	query.Set("longitude", strconv.FormatFloat(c.location.Longitude, 'f', -1, 64))
	query.Set("timezone", c.timezone.String())
	query.Set("forecast_days", strconv.Itoa(days))
	query.Set("wind_speed_unit", c.windUnit)
	query.Set("hourly", strings.Join(hourlyVariables, ","))
	query.Set("daily", strings.Join(dailyVariables, ","))

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("openmeteo: не удалось собрать запрос: %w", err)
	}
	request.Header.Set("User-Agent", c.userAgent)
	request.Header.Set("Accept", "application/json")
	return request, nil
}

func statusError(status int, body []byte) error {
	var apiError errorResponse
	if err := json.Unmarshal(body, &apiError); err == nil && apiError.Reason != "" {
		return fmt.Errorf("API ответил %d: %s", status, apiError.Reason)
	}
	return fmt.Errorf("API ответил %d", status)
}
