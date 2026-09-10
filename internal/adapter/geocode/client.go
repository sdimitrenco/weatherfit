// Package geocode ищет города через Open-Meteo Geocoding API.
package geocode

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
	// DefaultBaseURL — endpoint поиска по названию.
	DefaultBaseURL = "https://geocoding-api.open-meteo.com/v1/search"
	// DefaultTimeout — таймаут одного запроса.
	DefaultTimeout = 10 * time.Second
	// DefaultUserAgent отправляется с каждым запросом.
	DefaultUserAgent = "weatherfit-bot/1.0 (personal telegram weather bot)"
	// DefaultLanguage — язык названий в ответе.
	DefaultLanguage = "ru"

	maxBodyBytes = 1 << 20
)

// ErrNothingFound возвращается, когда по запросу нет ни одного города.
var ErrNothingFound = errors.New("geocode: ничего не найдено")

// Options — параметры создания клиента.
type Options struct {
	BaseURL    string
	UserAgent  string
	Language   string
	HTTPClient *http.Client
}

// Client ищет города по названию.
type Client struct {
	baseURL    string
	userAgent  string
	language   string
	httpClient *http.Client
}

// New создаёт клиент, подставляя значения по умолчанию.
func New(options Options) *Client {
	if options.BaseURL == "" {
		options.BaseURL = DefaultBaseURL
	}
	if options.UserAgent == "" {
		options.UserAgent = DefaultUserAgent
	}
	if options.Language == "" {
		options.Language = DefaultLanguage
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: DefaultTimeout}
	}
	return &Client{
		baseURL:    options.BaseURL,
		userAgent:  options.UserAgent,
		language:   options.Language,
		httpClient: options.HTTPClient,
	}
}

type searchResponse struct {
	Results []struct {
		Name      string  `json:"name"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Timezone  string  `json:"timezone"`
		Country   string  `json:"country"`
		Admin1    string  `json:"admin1"`
	} `json:"results"`
	Reason string `json:"reason"`
}

// Search ищет города по названию, возвращая не больше limit вариантов.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]port.Place, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("geocode: пустой запрос")
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 10 {
		limit = 10
	}

	values := url.Values{}
	values.Set("name", query)
	values.Set("count", strconv.Itoa(limit))
	values.Set("language", c.language)
	values.Set("format", "json")

	parsed, err := c.get(ctx, values)
	if err != nil {
		return nil, err
	}

	places := make([]port.Place, 0, len(parsed.Results))
	for _, result := range parsed.Results {
		place := port.Place{
			Name:    result.Name,
			Country: result.Country,
			Admin:   result.Admin1,
			TZName:  result.Timezone,
			Place: domain.Location{
				Name:      result.Name,
				Latitude:  result.Latitude,
				Longitude: result.Longitude,
			},
		}
		if _, err := time.LoadLocation(place.TZName); err != nil {
			continue
		}
		if place.Place.Validate() != nil {
			continue
		}
		places = append(places, place)
	}

	if len(places) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrNothingFound, query)
	}
	return places, nil
}

// Title собирает человекочитаемое название вида «Дрезден, Саксония, Германия».
func Title(place port.Place) string {
	parts := make([]string, 0, 3)
	for _, part := range []string{place.Name, place.Admin, place.Country} {
		part = strings.TrimSpace(part)
		if part == "" || contains(parts, part) {
			continue
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

func contains(parts []string, value string) bool {
	for _, part := range parts {
		if part == value {
			return true
		}
	}
	return false
}

func (c *Client) get(ctx context.Context, values url.Values) (searchResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+values.Encode(), nil)
	if err != nil {
		return searchResponse{}, fmt.Errorf("geocode: не удалось собрать запрос: %w", err)
	}
	request.Header.Set("User-Agent", c.userAgent)
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return searchResponse{}, fmt.Errorf("geocode: запрос не удался: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes))
	if err != nil {
		return searchResponse{}, fmt.Errorf("geocode: не удалось прочитать ответ: %w", err)
	}

	var parsed searchResponse
	if response.StatusCode != http.StatusOK {
		if err := json.Unmarshal(body, &parsed); err == nil && parsed.Reason != "" {
			return searchResponse{}, fmt.Errorf("geocode: API ответил %d: %s", response.StatusCode, parsed.Reason)
		}
		return searchResponse{}, fmt.Errorf("geocode: API ответил %d", response.StatusCode)
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return searchResponse{}, fmt.Errorf("geocode: не удалось разобрать ответ: %w", err)
	}
	return parsed, nil
}
