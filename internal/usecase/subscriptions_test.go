package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/i18n"
	"github.com/sdimitrenco/weatherfit/internal/port"
)

func testDefaults() Defaults {
	return Defaults{
		Place:       domain.Location{Name: "Дрезден", Latitude: 51.05, Longitude: 13.74},
		TZName:      "Europe/Berlin",
		ReportTime:  domain.DayTime{Hour: 7},
		ActiveHours: domain.HourWindow{Start: 7, End: 22},
		WindUnit:    domain.WindUnitMS,
		Lang:        i18n.English,
	}
}

func newSubscriptions(t *testing.T, store *memoryStore, geocoder *stubGeocoder, timezones *stubTimezones, now time.Time) *Subscriptions {
	t.Helper()
	if geocoder == nil {
		geocoder = &stubGeocoder{}
	}
	if timezones == nil {
		timezones = &stubTimezones{name: "Europe/Berlin"}
	}
	return NewSubscriptions(store, geocoder, timezones, fixedClock(now), testDefaults())
}

func TestEnsureCreatesSubscriberWithTelegramLanguage(t *testing.T) {
	store := newMemoryStore()
	now := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	subscriptions := newSubscriptions(t, store, nil, nil, now)

	subscriber, err := subscriptions.Ensure(context.Background(), 42, "ru-RU")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if subscriber.Lang != string(i18n.Russian) {
		t.Errorf("язык = %q, want русский из Telegram", subscriber.Lang)
	}
	if subscriber.Place.Name != "Дрезден" || subscriber.TZName != "Europe/Berlin" {
		t.Errorf("настройки по умолчанию не применились: %+v", subscriber)
	}
	if !subscriber.CreatedAt.Equal(now) {
		t.Errorf("время создания = %v", subscriber.CreatedAt)
	}
	if count, _ := store.Count(context.Background()); count != 1 {
		t.Errorf("подписчиков = %d", count)
	}
}

func TestEnsureFallsBackToDefaultLanguage(t *testing.T) {
	store := newMemoryStore()
	subscriptions := newSubscriptions(t, store, nil, nil, time.Now())

	subscriber, err := subscriptions.Ensure(context.Background(), 42, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subscriber.Lang != string(i18n.English) {
		t.Errorf("язык = %q, want английский", subscriber.Lang)
	}

	unsupported, err := subscriptions.Ensure(context.Background(), 43, "fr-FR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if unsupported.Lang != string(i18n.English) {
		t.Errorf("язык = %q, want английский для неподдерживаемого кода", unsupported.Lang)
	}
}

func TestEnsureKeepsManualLanguageChoice(t *testing.T) {
	existing := testSubscriber()
	existing.Lang = string(i18n.German)
	store := newMemoryStore(existing)
	subscriptions := newSubscriptions(t, store, nil, nil, time.Now())

	subscriber, err := subscriptions.Ensure(context.Background(), 42, "ru")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subscriber.Lang != string(i18n.German) {
		t.Errorf("язык = %q, выбранный вручную не должен затираться", subscriber.Lang)
	}
}

func TestSetPlace(t *testing.T) {
	store := newMemoryStore(testSubscriber())
	subscriptions := newSubscriptions(t, store, nil, nil, time.Now())

	place := port.Place{
		Name:   "Прага",
		TZName: "Europe/Prague",
		Place:  domain.Location{Name: "Прага", Latitude: 50.08, Longitude: 14.44},
	}
	subscriber, err := subscriptions.SetPlace(context.Background(), 42, place)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subscriber.Place.Name != "Прага" || subscriber.TZName != "Europe/Prague" {
		t.Errorf("город не сохранился: %+v", subscriber)
	}
	if subscriber.Pending != domain.PendingNone {
		t.Errorf("ожидание ввода должно сбрасываться: %q", subscriber.Pending)
	}
}

func TestSetCoordinatesResolvesTimezone(t *testing.T) {
	store := newMemoryStore(testSubscriber())
	timezones := &stubTimezones{name: "Asia/Tokyo"}
	subscriptions := newSubscriptions(t, store, nil, timezones, time.Now())

	subscriber, err := subscriptions.SetCoordinates(context.Background(), 42,
		domain.Location{Latitude: 35.68, Longitude: 139.69})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subscriber.TZName != "Asia/Tokyo" {
		t.Errorf("таймзона = %q", subscriber.TZName)
	}
	if subscriber.Place.Name != "35.680, 139.690" {
		t.Errorf("без названия want координаты: %q", subscriber.Place.Name)
	}
}

func TestSetCoordinatesRejectsBadInput(t *testing.T) {
	store := newMemoryStore(testSubscriber())
	subscriptions := newSubscriptions(t, store, nil, nil, time.Now())

	if _, err := subscriptions.SetCoordinates(context.Background(), 42, domain.Location{Latitude: 100}); err == nil {
		t.Error("expected an error для широты вне диапазона")
	}
}

func TestSetCoordinatesFailsWhenTimezoneUnknown(t *testing.T) {
	store := newMemoryStore(testSubscriber())
	timezones := &stubTimezones{err: errors.New("нет связи")}
	subscriptions := newSubscriptions(t, store, nil, timezones, time.Now())

	if _, err := subscriptions.SetCoordinates(context.Background(), 42, domain.Location{Latitude: 51, Longitude: 13}); err == nil {
		t.Error("expected an error определения таймзоны")
	}
}

func TestSetReportTime(t *testing.T) {
	subscriber := testSubscriber()
	subscriber.LastSentDate = "2026-09-10"
	store := newMemoryStore(subscriber)
	now := time.Date(2026, 9, 10, 6, 0, 0, 0, berlin(t))
	subscriptions := newSubscriptions(t, store, nil, nil, now)

	updated, err := subscriptions.SetReportTime(context.Background(), 42, "6:30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.ReportTime.String() != "06:30" {
		t.Errorf("время = %q", updated.ReportTime)
	}
	if updated.LastSentDate != "" {
		t.Errorf("отметка о сегодняшней рассылке должна сбрасываться: %q", updated.LastSentDate)
	}

	if _, err := subscriptions.SetReportTime(context.Background(), 42, "утром"); err == nil {
		t.Error("expected an error разбора времени")
	}
}

func TestSetLang(t *testing.T) {
	store := newMemoryStore(testSubscriber())
	subscriptions := newSubscriptions(t, store, nil, nil, time.Now())

	subscriber, err := subscriptions.SetLang(context.Background(), 42, i18n.German)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subscriber.Lang != string(i18n.German) {
		t.Errorf("язык = %q", subscriber.Lang)
	}
	if _, err := subscriptions.SetLang(context.Background(), 42, i18n.Lang("fr")); err == nil {
		t.Error("expected an error для неподдерживаемого языка")
	}
}

func TestToggleWindUnit(t *testing.T) {
	store := newMemoryStore(testSubscriber())
	subscriptions := newSubscriptions(t, store, nil, nil, time.Now())

	first, err := subscriptions.ToggleWindUnit(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first.WindUnit != domain.WindUnitKMH {
		t.Errorf("единица = %q, want км/ч", first.WindUnit)
	}

	second, err := subscriptions.ToggleWindUnit(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if second.WindUnit != domain.WindUnitMS {
		t.Errorf("единица = %q, want м/с", second.WindUnit)
	}
}

func TestSetPausedAndUnsubscribe(t *testing.T) {
	store := newMemoryStore(testSubscriber())
	subscriptions := newSubscriptions(t, store, nil, nil, time.Now())

	paused, err := subscriptions.SetPaused(context.Background(), 42, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !paused.Paused {
		t.Error("рассылка должна встать на паузу")
	}

	if err := subscriptions.Unsubscribe(context.Background(), 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := subscriptions.Get(context.Background(), 42); !errors.Is(err, port.ErrSubscriberNotFound) {
		t.Errorf("ошибка = %v, expected no such subscriber", err)
	}
}

func TestSearchCitiesPassesQuery(t *testing.T) {
	geocoder := &stubGeocoder{places: []port.Place{{Name: "Дрезден"}}}
	subscriptions := newSubscriptions(t, newMemoryStore(), geocoder, nil, time.Now())

	places, err := subscriptions.SearchCities(context.Background(), "Dresden", "de")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if geocoder.query != "Dresden" || len(places) != 1 {
		t.Errorf("query = %q, found = %d", geocoder.query, len(places))
	}
	if geocoder.language != "de" {
		t.Errorf("language = %q, want de", geocoder.language)
	}
}

func TestSetPendingStoresAction(t *testing.T) {
	store := newMemoryStore(testSubscriber())
	subscriptions := newSubscriptions(t, store, nil, nil, time.Now())

	if err := subscriptions.SetPending(context.Background(), 42, domain.PendingCity); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	subscriber, _ := store.Get(context.Background(), 42)
	if subscriber.Pending != domain.PendingCity {
		t.Errorf("ожидание = %q", subscriber.Pending)
	}
}

func TestSetActiveHours(t *testing.T) {
	store := newMemoryStore(testSubscriber())
	subscriptions := newSubscriptions(t, store, nil, nil, time.Now())

	updated, err := subscriptions.SetActiveHours(context.Background(), 42, "9-18")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.ActiveHours.String() != "09-18" {
		t.Errorf("активные часы = %q", updated.ActiveHours)
	}
	if updated.Pending != domain.PendingNone {
		t.Errorf("ожидание ввода должно сбрасываться: %q", updated.Pending)
	}

	for _, raw := range []string{"22-07", "07-24", "0722", "утром", ""} {
		if _, err := subscriptions.SetActiveHours(context.Background(), 42, raw); err == nil {
			t.Errorf("%q: expected an error", raw)
		}
	}

	stored, _ := store.Get(context.Background(), 42)
	if stored.ActiveHours.String() != "09-18" {
		t.Errorf("неверный ввод не должен затирать настройку: %q", stored.ActiveHours)
	}
}
