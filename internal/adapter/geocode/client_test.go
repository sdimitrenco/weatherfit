package geocode

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sdimitrenco/weatherfit/internal/port"
)

const dresdenResponse = `{
  "results": [
    {"id": 2935022, "name": "Дрезден", "latitude": 51.05089, "longitude": 13.73832,
     "timezone": "Europe/Berlin", "country": "Германия", "country_code": "DE", "admin1": "Саксония"},
    {"id": 3100127, "name": "Дрезденко", "latitude": 52.83831, "longitude": 15.83079,
     "timezone": "Europe/Warsaw", "country": "Польша", "country_code": "PL", "admin1": "Любушское воеводство"}
  ],
  "generationtime_ms": 0.5
}`

func serve(t *testing.T, status int, body string) (*httptest.Server, *[]*http.Request) {
	t.Helper()
	var seen []*http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, &seen
}

func TestSearch(t *testing.T) {
	server, seen := serve(t, http.StatusOK, dresdenResponse)
	client := New(Options{BaseURL: server.URL})

	places, err := client.Search(context.Background(), " Дрезден ", 5, "ru")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	query := (*seen)[0].URL.Query()
	if query.Get("name") != "Дрезден" {
		t.Errorf("name = %q, want Дрезден без пробелов", query.Get("name"))
	}
	if query.Get("count") != "5" || query.Get("language") != "ru" || query.Get("format") != "json" {
		t.Errorf("параметры запроса неверны: %v", query)
	}

	if len(places) != 2 {
		t.Fatalf("найдено = %d, want 2", len(places))
	}
	first := places[0]
	if first.Name != "Дрезден" || first.TZName != "Europe/Berlin" {
		t.Errorf("первый результат = %+v", first)
	}
	if first.Place.Latitude != 51.05089 || first.Place.Longitude != 13.73832 {
		t.Errorf("координаты = %v, %v", first.Place.Latitude, first.Place.Longitude)
	}
	if first.Place.Name != "Дрезден" {
		t.Errorf("название в domain.Location = %q", first.Place.Name)
	}
}

func TestSearchLimits(t *testing.T) {
	server, seen := serve(t, http.StatusOK, dresdenResponse)
	client := New(Options{BaseURL: server.URL})

	for _, limit := range []int{0, -5, 100} {
		if _, err := client.Search(context.Background(), "Дрезден", limit, "ru"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	counts := []string{(*seen)[0].URL.Query().Get("count"), (*seen)[1].URL.Query().Get("count"), (*seen)[2].URL.Query().Get("count")}
	if counts[0] != "1" || counts[1] != "1" || counts[2] != "10" {
		t.Errorf("count = %v, want [1 1 10]", counts)
	}
}

func TestSearchNothingFound(t *testing.T) {
	server, _ := serve(t, http.StatusOK, `{"generationtime_ms":0.1}`)
	client := New(Options{BaseURL: server.URL})

	_, err := client.Search(context.Background(), "Атлантида", 5, "ru")
	if !errors.Is(err, ErrNothingFound) {
		t.Errorf("ошибка = %v, expected ErrNothingFound", err)
	}
}

func TestSearchSkipsResultsWithUnknownTimezone(t *testing.T) {
	body := `{"results":[
	  {"name":"Марсополь","latitude":10,"longitude":10,"timezone":"Mars/Olympus","country":"Марс"},
	  {"name":"Дрезден","latitude":51.05,"longitude":13.74,"timezone":"Europe/Berlin","country":"Германия"}
	]}`
	server, _ := serve(t, http.StatusOK, body)
	client := New(Options{BaseURL: server.URL})

	places, err := client.Search(context.Background(), "город", 5, "ru")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(places) != 1 || places[0].Name != "Дрезден" {
		t.Errorf("результаты = %+v, want only Dresden", places)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	client := New(Options{BaseURL: "http://example.invalid"})
	if _, err := client.Search(context.Background(), "   ", 5, "ru"); err == nil {
		t.Error("expected an error для пустого запроса")
	}
}

func TestSearchAPIError(t *testing.T) {
	server, _ := serve(t, http.StatusBadRequest, `{"error":true,"reason":"Parameter name is required"}`)
	client := New(Options{BaseURL: server.URL})

	_, err := client.Search(context.Background(), "Дрезден", 5, "ru")
	if err == nil {
		t.Fatal("expected an error, got none")
	}
	if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "Parameter name is required") {
		t.Errorf("ошибка не содержит статус и причину: %v", err)
	}
}

func TestSearchBrokenJSON(t *testing.T) {
	server, _ := serve(t, http.StatusOK, `{"results":`)
	client := New(Options{BaseURL: server.URL})
	if _, err := client.Search(context.Background(), "Дрезден", 5, "ru"); err == nil {
		t.Error("expected an error разбора")
	}
}

func TestSearchRespectsContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	client := New(Options{BaseURL: server.URL})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.Search(ctx, "Дрезден", 5, "ru"); err == nil {
		t.Error("expected an error отменённого контекста")
	}
}

func TestTitle(t *testing.T) {
	tests := []struct {
		place port.Place
		want  string
	}{
		{place: port.Place{Name: "Дрезден", Admin: "Саксония", Country: "Германия"}, want: "Дрезден, Саксония, Германия"},
		{place: port.Place{Name: "Дрезден", Country: "Германия"}, want: "Дрезден, Германия"},
		{place: port.Place{Name: "Берлин", Admin: "Берлин", Country: "Германия"}, want: "Берлин, Германия"},
		{place: port.Place{Name: "Дрезден"}, want: "Дрезден"},
	}
	for _, tc := range tests {
		if got := Title(tc.place); got != tc.want {
			t.Errorf("Title(%+v) = %q, want %q", tc.place, got, tc.want)
		}
	}
}

func TestClientImplementsPort(t *testing.T) {
	var _ port.Geocoder = New(Options{})
}
