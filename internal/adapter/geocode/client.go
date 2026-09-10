// Package geocode searches cities through the Open-Meteo Geocoding API.
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
	// DefaultBaseURL is the search endpoint.
	DefaultBaseURL = "https://geocoding-api.open-meteo.com/v1/search"
	// DefaultTimeout bounds one request.
	DefaultTimeout = 10 * time.Second
	// DefaultUserAgent is sent with every request.
	DefaultUserAgent = "weatherfit-bot/1.0 (personal telegram weather bot)"
	// DefaultLanguage is used when the caller passes no language.
	DefaultLanguage = "en"

	maxBodyBytes = 1 << 20
)

// ErrNothingFound is returned when the query matches no place.
var ErrNothingFound = errors.New("geocode: nothing found")

// Options configures the client.
type Options struct {
	BaseURL    string
	UserAgent  string
	Language   string
	HTTPClient *http.Client
}

// Client searches places by name.
type Client struct {
	baseURL    string
	userAgent  string
	language   string
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

func (c *Client) languageOr(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		return c.language
	}
	return language
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

// Search looks up places by name, returning at most limit matches. Place
// names come back spelled in the requested language.
func (c *Client) Search(ctx context.Context, query string, limit int, language string) ([]port.Place, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("geocode: empty query")
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
	values.Set("language", c.languageOr(language))
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

// Title builds a readable place name such as "Dresden, Saxony, Germany".
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
		return searchResponse{}, fmt.Errorf("geocode: cannot build the request: %w", err)
	}
	request.Header.Set("User-Agent", c.userAgent)
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return searchResponse{}, fmt.Errorf("geocode: request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes))
	if err != nil {
		return searchResponse{}, fmt.Errorf("geocode: cannot read the response: %w", err)
	}

	var parsed searchResponse
	if response.StatusCode != http.StatusOK {
		if err := json.Unmarshal(body, &parsed); err == nil && parsed.Reason != "" {
			return searchResponse{}, fmt.Errorf("geocode: API returned %d: %s", response.StatusCode, parsed.Reason)
		}
		return searchResponse{}, fmt.Errorf("geocode: API returned %d", response.StatusCode)
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return searchResponse{}, fmt.Errorf("geocode: cannot parse the response: %w", err)
	}
	return parsed, nil
}
