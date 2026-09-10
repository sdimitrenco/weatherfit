package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/i18n"
	"github.com/sdimitrenco/weatherfit/internal/port"
)

type memoryStore struct {
	mu          sync.Mutex
	subscribers map[int64]domain.Subscriber
	saveErr     error
}

func newMemoryStore(subscribers ...domain.Subscriber) *memoryStore {
	store := &memoryStore{subscribers: map[int64]domain.Subscriber{}}
	for _, subscriber := range subscribers {
		store.subscribers[subscriber.ChatID] = subscriber
	}
	return store
}

func (m *memoryStore) Save(_ context.Context, subscriber domain.Subscriber) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	m.subscribers[subscriber.ChatID] = subscriber
	return nil
}

func (m *memoryStore) Get(_ context.Context, chatID int64) (domain.Subscriber, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	subscriber, ok := m.subscribers[chatID]
	if !ok {
		return domain.Subscriber{}, fmt.Errorf("memory: %w", port.ErrSubscriberNotFound)
	}
	return subscriber, nil
}

func (m *memoryStore) All(_ context.Context) ([]domain.Subscriber, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	all := make([]domain.Subscriber, 0, len(m.subscribers))
	for _, subscriber := range m.subscribers {
		all = append(all, subscriber)
	}
	return all, nil
}

func (m *memoryStore) Delete(_ context.Context, chatID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.subscribers, chatID)
	return nil
}

func (m *memoryStore) MarkSent(_ context.Context, chatID int64, date string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	subscriber := m.subscribers[chatID]
	subscriber.LastSentDate = date
	m.subscribers[chatID] = subscriber
	return nil
}

func (m *memoryStore) SetPending(_ context.Context, chatID int64, pending domain.PendingAction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	subscriber := m.subscribers[chatID]
	subscriber.Pending = pending
	m.subscribers[chatID] = subscriber
	return nil
}

func (m *memoryStore) Count(_ context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.subscribers), nil
}

type stubProvider struct {
	forecast domain.Forecast
	err      error
	failFor  int
	calls    int
}

func (s *stubProvider) Forecast(_ context.Context, _ port.ForecastRequest) (domain.Forecast, error) {
	s.calls++
	if s.calls <= s.failFor {
		return domain.Forecast{}, errors.New("open-meteo недоступен")
	}
	if s.err != nil {
		return domain.Forecast{}, s.err
	}
	return s.forecast, nil
}

type stubNotifier struct {
	sent    []string
	chatIDs []int64
	err     error
}

func (s *stubNotifier) Send(_ context.Context, chatID int64, text string) error {
	if s.err != nil {
		return s.err
	}
	s.chatIDs = append(s.chatIDs, chatID)
	s.sent = append(s.sent, text)
	return nil
}

type stubRenderer struct{}

func (stubRenderer) Report(report domain.Report, subscriber domain.Subscriber) string {
	return fmt.Sprintf("отчёт для %d на %s, часов %d", subscriber.ChatID,
		report.Date.Format(time.DateOnly), len(report.Hours))
}

func (stubRenderer) Current(place domain.Location, _ domain.CurrentPoint, _ domain.Subscriber) string {
	return "сейчас в " + place.Name
}

func (stubRenderer) Text(_ domain.Subscriber, key string, _ ...any) string {
	return key
}

type stubGeocoder struct {
	places   []port.Place
	err      error
	query    string
	language string
}

func (s *stubGeocoder) Search(_ context.Context, query string, _ int, language string) ([]port.Place, error) {
	s.query = query
	s.language = language
	return s.places, s.err
}

type stubTimezones struct {
	name string
	err  error
}

func (s *stubTimezones) ResolveTimezone(_ context.Context, _ domain.Location) (string, error) {
	return s.name, s.err
}

func fixedClock(moment time.Time) port.Clock {
	return port.ClockFunc(func() time.Time { return moment })
}

func berlin(t *testing.T) *time.Location {
	t.Helper()
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("не удалось загрузить таймзону: %v", err)
	}
	return location
}

func testSubscriber() domain.Subscriber {
	return domain.Subscriber{
		ChatID:      42,
		Lang:        string(i18n.Russian),
		Place:       domain.Location{Name: "Дрезден", Latitude: 51.05, Longitude: 13.74},
		TZName:      "Europe/Berlin",
		ReportTime:  domain.DayTime{Hour: 7},
		ActiveHours: domain.HourWindow{Start: 7, End: 22},
		WindUnit:    domain.WindUnitMS,
	}
}

func twoDayForecast(t *testing.T) domain.Forecast {
	t.Helper()
	location := berlin(t)

	hours := make([]domain.HourPoint, 0, 48)
	for day := 10; day <= 11; day++ {
		for hour := 0; hour < 24; hour++ {
			hours = append(hours, domain.HourPoint{
				Time:                     time.Date(2026, 9, day, hour, 0, 0, 0, location),
				TemperatureC:             domain.Some(float64(hour)),
				ApparentTemperatureC:     domain.Some(float64(hour) - 1),
				PrecipitationProbability: domain.Some(0),
				PrecipitationMM:          domain.Some(0.0),
				WeatherCode:              domain.Some(3),
				WindSpeedMS:              domain.Some(2.0),
				WindDirectionDeg:         domain.Some(270),
				WindGustsMS:              domain.Some(4.0),
				IsDay:                    domain.Some(hour >= 7 && hour <= 19),
			})
		}
	}

	return domain.Forecast{
		Location: domain.Location{Name: "Дрезден"},
		Timezone: location,
		Current: domain.Some(domain.CurrentPoint{
			Time:         time.Date(2026, 9, 10, 10, 15, 0, 0, location),
			TemperatureC: domain.Some(15.7),
			WeatherCode:  domain.Some(3),
		}),
		Days: []domain.DaySummary{
			{Date: time.Date(2026, 9, 10, 0, 0, 0, 0, location), TemperatureMaxC: domain.Some(22.0)},
			{Date: time.Date(2026, 9, 11, 0, 0, 0, 0, location), TemperatureMaxC: domain.Some(18.0)},
		},
		Hours: hours,
	}
}
