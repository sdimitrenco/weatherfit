package openmeteo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/port"
)

const fixtureDir = "../../../testdata"

func berlin(t *testing.T) *time.Location {
	t.Helper()
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("cannot load the timezone: %v", err)
	}
	return location
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(fixtureDir, name))
	if err != nil {
		t.Fatalf("cannot read the fixture %s: %v", name, err)
	}
	return body
}

func serveFixture(t *testing.T, name string) (*httptest.Server, *[]*http.Request) {
	t.Helper()
	body := fixture(t, name)
	var seen []*http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server, &seen
}

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	return New(Options{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
	})
}

func dresden(t *testing.T, days int) port.ForecastRequest {
	t.Helper()
	return port.ForecastRequest{
		Place:    domain.Location{Name: "Дрезден", Latitude: 51.05, Longitude: 13.74},
		Timezone: berlin(t),
		Days:     days,
	}
}

func TestForecastSendsExpectedQuery(t *testing.T) {
	server, seen := serveFixture(t, "openmeteo_dresden_real.json")
	client := newTestClient(t, server.URL)

	if _, err := client.Forecast(context.Background(), dresden(t, 2)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(*seen) != 1 {
		t.Fatalf("запросов = %d, want 1", len(*seen))
	}
	request := (*seen)[0]
	query := request.URL.Query()

	want := map[string]string{
		"latitude":        "51.05",
		"longitude":       "13.74",
		"timezone":        "Europe/Berlin",
		"forecast_days":   "2",
		"wind_speed_unit": "ms",
	}
	for key, value := range want {
		if got := query.Get(key); got != value {
			t.Errorf("%s = %q, want %q", key, got, value)
		}
	}

	for _, variable := range hourlyVariables {
		if !strings.Contains(query.Get("hourly"), variable) {
			t.Errorf("в hourly нет %q", variable)
		}
	}
	for _, variable := range dailyVariables {
		if !strings.Contains(query.Get("daily"), variable) {
			t.Errorf("в daily нет %q", variable)
		}
	}
	if agent := request.Header.Get("User-Agent"); !strings.Contains(agent, "weatherfit") {
		t.Errorf("User-Agent = %q", agent)
	}
}

func TestForecastMapsRealFixture(t *testing.T) {
	server, _ := serveFixture(t, "openmeteo_dresden_real.json")
	client := newTestClient(t, server.URL)

	forecast, err := client.Forecast(context.Background(), dresden(t, 2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(forecast.Hours) != 48 {
		t.Errorf("часов = %d, want 48", len(forecast.Hours))
	}
	if len(forecast.Days) != 2 {
		t.Errorf("дней = %d, want 2", len(forecast.Days))
	}
	if forecast.Location.Name != "Дрезден" {
		t.Errorf("локация = %q", forecast.Location.Name)
	}

	first := forecast.Hours[0]
	if got := first.Time.Format(time.RFC3339); got != "2026-09-10T00:00:00+02:00" {
		t.Errorf("первый час = %q, want локальное время со смещением +02:00", got)
	}
	if first.Time.Location().String() != "Europe/Berlin" {
		t.Errorf("таймзона часа = %q", first.Time.Location())
	}

	hour7 := forecast.Hours[7]
	if temperature, ok := hour7.TemperatureC.Get(); !ok || temperature != 13.6 {
		t.Errorf("температура 07:00 = %v, %v, want 13.6", temperature, ok)
	}
	if apparent, ok := hour7.ApparentTemperatureC.Get(); !ok || apparent != 12.2 {
		t.Errorf("ощущаемая 07:00 = %v, %v, want 12.2", apparent, ok)
	}
	if code, ok := hour7.WeatherCode.Get(); !ok || code != 3 {
		t.Errorf("код погоды 07:00 = %v, %v, want 3", code, ok)
	}
	if speed, ok := hour7.WindSpeedMS.Get(); !ok || speed != 2.45 {
		t.Errorf("ветер 07:00 = %v, %v, want 2.45", speed, ok)
	}
	if direction, ok := hour7.WindDirectionDeg.Get(); !ok || direction != 258 {
		t.Errorf("направление 07:00 = %v, %v, want 258", direction, ok)
	}
	if isDay, ok := hour7.IsDay.Get(); !ok || !isDay {
		t.Errorf("is_day 07:00 = %v, %v, want день", isDay, ok)
	}

	day := forecast.Days[0]
	if got := day.Date.Format(time.DateOnly); got != "2026-09-10" {
		t.Errorf("дата дня = %q", got)
	}
	sunrise, ok := day.Sunrise.Get()
	if !ok || sunrise.Format("15:04") != "06:32" {
		t.Errorf("рассвет = %v, %v, want 06:32", sunrise, ok)
	}
	if sunrise.Location().String() != "Europe/Berlin" {
		t.Errorf("таймзона рассвета = %q", sunrise.Location())
	}
}

func TestForecastMapsCurrentBlock(t *testing.T) {
	server, _ := serveFixture(t, "openmeteo_dresden_real.json")
	client := newTestClient(t, server.URL)

	forecast, err := client.Forecast(context.Background(), dresden(t, 2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	current, ok := forecast.Current.Get()
	if !ok {
		t.Fatal("блок current пуст")
	}
	if current.Time.Location().String() != "Europe/Berlin" {
		t.Errorf("таймзона current = %q", current.Time.Location())
	}
	if temperature, has := current.TemperatureC.Get(); !has || temperature != 15.7 {
		t.Errorf("температура сейчас = %v, %v, want 15.7", temperature, has)
	}
	if humidity, has := current.RelativeHumidity.Get(); !has || humidity != 68 {
		t.Errorf("влажность = %v, %v, want 68", humidity, has)
	}
	if isDay, has := current.IsDay.Get(); !has || !isDay {
		t.Errorf("is_day = %v, %v, want день", isDay, has)
	}
}

func TestForecastAlwaysRequestsMetersPerSecond(t *testing.T) {
	server, seen := serveFixture(t, "openmeteo_dresden_real.json")
	client := newTestClient(t, server.URL)

	if _, err := client.Forecast(context.Background(), dresden(t, 2)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := (*seen)[0].URL.Query().Get("wind_speed_unit"); got != "ms" {
		t.Errorf("wind_speed_unit = %q, want ms: домен всегда хранит м/с", got)
	}
	for _, variable := range currentVariables {
		if !strings.Contains((*seen)[0].URL.Query().Get("current"), variable) {
			t.Errorf("в current нет %q", variable)
		}
	}
}

func TestForecastKeepsMissingValuesEmpty(t *testing.T) {
	server, _ := serveFixture(t, "openmeteo_nulls.json")
	client := newTestClient(t, server.URL)

	forecast, err := client.Forecast(context.Background(), dresden(t, 1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	first, second := forecast.Hours[0], forecast.Hours[1]
	if first.ApparentTemperatureC.Valid() {
		t.Error("ощущаемая температура должна быть пустой")
	}
	if value := first.ApparentTemperatureC.Or(-999); value != -999 {
		t.Errorf("Or вернул %v вместо fallback", value)
	}
	if second.TemperatureC.Valid() || second.WeatherCode.Valid() || second.WindSpeedMS.Valid() {
		t.Error("пустые поля второго часа не должны быть заполнены")
	}
	if !second.PrecipitationProbability.Valid() {
		t.Error("вероятность осадков второго часа должна быть заполнена")
	}

	day := forecast.Days[0]
	if day.TemperatureMaxC.Valid() || day.Sunrise.Valid() || day.UVIndexMax.Valid() {
		t.Error("пустые поля дня не должны быть заполнены")
	}
	if minimum, ok := day.TemperatureMinC.Get(); !ok || minimum != 8.0 {
		t.Errorf("минимум = %v, %v, want 8.0", minimum, ok)
	}
	sunset, ok := day.Sunset.Get()
	if !ok || sunset.Format("15:04") != "19:36" {
		t.Errorf("закат = %v, %v, want 19:36", sunset, ok)
	}
}

func TestForecastAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":true,"reason":"Cannot initialize WeatherVariable from invalid String value foo"}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.Forecast(context.Background(), dresden(t, 2))
	if err == nil {
		t.Fatal("expected an error, got none")
	}
	if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "invalid String value") {
		t.Errorf("ошибка не содержит статус и причину: %v", err)
	}
}

func TestForecastServerErrorWithoutReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>gateway</html>"))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.Forecast(context.Background(), dresden(t, 2))
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Errorf("ошибка = %v, want a mention of 502", err)
	}
}

func TestForecastBrokenJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"hourly":`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	if _, err := client.Forecast(context.Background(), dresden(t, 2)); err == nil {
		t.Fatal("expected an error разбора, её нет")
	}
}

func TestForecastEmptyHourlyBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"timezone":"Europe/Berlin","hourly":{"time":[]},"daily":{"time":[]}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.Forecast(context.Background(), dresden(t, 2))
	if err == nil || !strings.Contains(err.Error(), "нет почасовых данных") {
		t.Errorf("ошибка = %v, want сообщение о пустых данных", err)
	}
}

func TestForecastRespectsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.Forecast(ctx, dresden(t, 2)); err == nil {
		t.Fatal("expected an error отменённого контекста, её нет")
	}
}

func TestForecastRejectsBadArguments(t *testing.T) {
	client := newTestClient(t, "http://example.invalid")
	for _, days := range []int{0, -1, 17} {
		if _, err := client.Forecast(context.Background(), dresden(t, days)); err == nil {
			t.Errorf("forecast_days=%d: expected an error", days)
		}
	}
}

func TestNewFillsDefaults(t *testing.T) {
	client := New(Options{})
	if client.baseURL != DefaultBaseURL {
		t.Errorf("базовый URL = %q", client.baseURL)
	}
	if client.userAgent != DefaultUserAgent {
		t.Errorf("User-Agent = %q", client.userAgent)
	}
	if client.httpClient.Timeout != DefaultTimeout {
		t.Errorf("таймаут = %v, want %v", client.httpClient.Timeout, DefaultTimeout)
	}
}

func TestForecastRejectsBadRequest(t *testing.T) {
	client := newTestClient(t, "http://example.invalid")

	noTimezone := dresden(t, 2)
	noTimezone.Timezone = nil
	if _, err := client.Forecast(context.Background(), noTimezone); err == nil {
		t.Error("expected an error для пустой таймзоны")
	}

	badPlace := dresden(t, 2)
	badPlace.Place.Latitude = 100
	if _, err := client.Forecast(context.Background(), badPlace); err == nil {
		t.Error("expected an error для широты вне диапазона")
	}
}

func TestResolveTimezone(t *testing.T) {
	server, seen := serveFixture(t, "openmeteo_dresden_real.json")
	client := newTestClient(t, server.URL)

	name, err := client.ResolveTimezone(context.Background(), domain.Location{Latitude: 51.05, Longitude: 13.74})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Europe/Berlin" {
		t.Errorf("таймзона = %q, want Europe/Berlin", name)
	}
	if got := (*seen)[0].URL.Query().Get("timezone"); got != "auto" {
		t.Errorf("timezone = %q, want auto", got)
	}
}

func TestResolveTimezoneRejectsBadCoordinates(t *testing.T) {
	client := newTestClient(t, "http://example.invalid")
	if _, err := client.ResolveTimezone(context.Background(), domain.Location{Latitude: 100}); err == nil {
		t.Error("expected an error для широты вне диапазона")
	}
}

func TestClientImplementsPorts(t *testing.T) {
	client := New(Options{})
	var _ port.ForecastProvider = client
	var _ port.TimezoneResolver = client
}
