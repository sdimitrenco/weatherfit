// Package openmeteo is the Open-Meteo Forecast API client and its mapping into the domain.
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
	"github.com/sdimitrenco/weatherfit/internal/port"
)

const (
	// DefaultBaseURL is the public Forecast API endpoint.
	DefaultBaseURL = "https://api.open-meteo.com/v1/forecast"
	// DefaultTimeout bounds one request.
	DefaultTimeout = 10 * time.Second
	// DefaultUserAgent is sent with every request.
	DefaultUserAgent = "weatherfit-bot/1.0 (personal telegram weather bot)"

	// windSpeedUnit is the unit the domain stores wind speed in.
	windSpeedUnit = "ms"

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

var currentVariables = []string{
	"temperature_2m",
	"apparent_temperature",
	"precipitation",
	"weather_code",
	"wind_speed_10m",
	"wind_direction_10m",
	"wind_gusts_10m",
	"relative_humidity_2m",
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

// Options configures the client.
type Options struct {
	BaseURL    string
	UserAgent  string
	HTTPClient *http.Client
}

// Client queries Open-Meteo. One client serves any number of points.
type Client struct {
	baseURL    string
	userAgent  string
	httpClient *http.Client
}

// New builds a client, filling in defaults.
func New(options Options) *Client {
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
		baseURL:    options.BaseURL,
		userAgent:  options.UserAgent,
		httpClient: options.HTTPClient,
	}
}

// Forecast requests request.Days calendar days starting today.
func (c *Client) Forecast(ctx context.Context, request port.ForecastRequest) (domain.Forecast, error) {
	if request.Days < 1 || request.Days > 16 {
		return domain.Forecast{}, fmt.Errorf("openmeteo: forecast_days=%d is outside the range [1, 16]", request.Days)
	}
	if request.Timezone == nil {
		return domain.Forecast{}, errors.New("openmeteo: request timezone is not set")
	}
	if err := request.Place.Validate(); err != nil {
		return domain.Forecast{}, fmt.Errorf("openmeteo: %w", err)
	}

	httpRequest, err := c.newRequest(ctx, request)
	if err != nil {
		return domain.Forecast{}, err
	}

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return domain.Forecast{}, fmt.Errorf("openmeteo: request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, httpResponse.Body)
		_ = httpResponse.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(httpResponse.Body, maxBodyBytes))
	if err != nil {
		return domain.Forecast{}, fmt.Errorf("openmeteo: cannot read the response: %w", err)
	}

	if httpResponse.StatusCode != http.StatusOK {
		return domain.Forecast{}, fmt.Errorf("openmeteo: %w", statusError(httpResponse.StatusCode, body))
	}

	var parsed response
	if err := json.Unmarshal(body, &parsed); err != nil {
		return domain.Forecast{}, fmt.Errorf("openmeteo: cannot parse the response: %w", err)
	}

	return toDomain(parsed, request)
}

func (c *Client) newRequest(ctx context.Context, request port.ForecastRequest) (*http.Request, error) {
	query := url.Values{}
	query.Set("latitude", strconv.FormatFloat(request.Place.Latitude, 'f', -1, 64))
	query.Set("longitude", strconv.FormatFloat(request.Place.Longitude, 'f', -1, 64))
	query.Set("timezone", request.Timezone.String())
	query.Set("forecast_days", strconv.Itoa(request.Days))
	query.Set("wind_speed_unit", windSpeedUnit)
	query.Set("current", strings.Join(currentVariables, ","))
	query.Set("hourly", strings.Join(hourlyVariables, ","))
	query.Set("daily", strings.Join(dailyVariables, ","))

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("openmeteo: cannot build the request: %w", err)
	}
	httpRequest.Header.Set("User-Agent", c.userAgent)
	httpRequest.Header.Set("Accept", "application/json")
	return httpRequest, nil
}

func statusError(status int, body []byte) error {
	var apiError errorResponse
	if err := json.Unmarshal(body, &apiError); err == nil && apiError.Reason != "" {
		return fmt.Errorf("API returned %d: %s", status, apiError.Reason)
	}
	return fmt.Errorf("API returned %d", status)
}

// ResolveTimezone resolves a timezone name from coordinates via timezone=auto.
func (c *Client) ResolveTimezone(ctx context.Context, place domain.Location) (string, error) {
	if err := place.Validate(); err != nil {
		return "", fmt.Errorf("openmeteo: %w", err)
	}

	query := url.Values{}
	query.Set("latitude", strconv.FormatFloat(place.Latitude, 'f', -1, 64))
	query.Set("longitude", strconv.FormatFloat(place.Longitude, 'f', -1, 64))
	query.Set("timezone", "auto")
	query.Set("forecast_days", "1")
	query.Set("current", "temperature_2m")

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+query.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("openmeteo: cannot build the request: %w", err)
	}
	request.Header.Set("User-Agent", c.userAgent)
	request.Header.Set("Accept", "application/json")

	httpResponse, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("openmeteo: request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, httpResponse.Body)
		_ = httpResponse.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(httpResponse.Body, maxBodyBytes))
	if err != nil {
		return "", fmt.Errorf("openmeteo: cannot read the response: %w", err)
	}
	if httpResponse.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openmeteo: %w", statusError(httpResponse.StatusCode, body))
	}

	var parsed response
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("openmeteo: cannot parse the response: %w", err)
	}
	if parsed.Timezone == "" {
		return "", errors.New("openmeteo: the response carries no timezone")
	}
	if _, err := time.LoadLocation(parsed.Timezone); err != nil {
		return "", fmt.Errorf("openmeteo: unknown timezone %q", parsed.Timezone)
	}
	return parsed.Timezone, nil
}
