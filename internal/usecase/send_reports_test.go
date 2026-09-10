package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/i18n"
)

func noSleep() (func(context.Context, time.Duration) error, *[]time.Duration) {
	var delays []time.Duration
	return func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}, &delays
}

func TestSendDueSendsOnlyDueSubscribers(t *testing.T) {
	due := testSubscriber()
	notDue := testSubscriber()
	notDue.ChatID = 43
	notDue.ReportTime = domain.DayTime{Hour: 23}
	alreadySent := testSubscriber()
	alreadySent.ChatID = 44
	alreadySent.LastSentDate = "2026-09-10"
	paused := testSubscriber()
	paused.ChatID = 45
	paused.Paused = true

	store := newMemoryStore(due, notDue, alreadySent, paused)
	provider := &stubProvider{forecast: twoDayForecast(t)}
	clock := fixedClock(time.Date(2026, 9, 10, 7, 0, 0, 0, berlin(t)))
	notifier := &stubNotifier{}

	sleep, _ := noSleep()
	sender := NewSendReports(store, NewBuildReport(provider, clock, quietLogger()), stubRenderer{},
		notifier, clock, Retry{Attempts: 3, Base: time.Minute, Sleep: sleep}, quietLogger())

	sent, err := sender.SendDue(context.Background())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if sent != 1 {
		t.Errorf("отправлено = %d, ожидалось 1", sent)
	}
	if len(notifier.chatIDs) != 1 || notifier.chatIDs[0] != 42 {
		t.Errorf("получатели = %v, ожидался только 42", notifier.chatIDs)
	}
	if !strings.Contains(notifier.sent[0], "отчёт для 42") {
		t.Errorf("текст сообщения = %q", notifier.sent[0])
	}
}

func TestSendDueMarksDeliveryDate(t *testing.T) {
	store := newMemoryStore(testSubscriber())
	provider := &stubProvider{forecast: twoDayForecast(t)}
	clock := fixedClock(time.Date(2026, 9, 10, 7, 0, 0, 0, berlin(t)))

	sleep, _ := noSleep()
	sender := NewSendReports(store, NewBuildReport(provider, clock, quietLogger()), stubRenderer{},
		&stubNotifier{}, clock, Retry{Attempts: 1, Base: time.Minute, Sleep: sleep}, quietLogger())

	if _, err := sender.SendDue(context.Background()); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	subscriber, err := store.Get(context.Background(), 42)
	if err != nil {
		t.Fatalf("не удалось прочитать подписчика: %v", err)
	}
	if subscriber.LastSentDate != "2026-09-10" {
		t.Errorf("дата рассылки = %q", subscriber.LastSentDate)
	}

	sent, err := sender.SendDue(context.Background())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if sent != 0 {
		t.Errorf("повторная рассылка = %d, ожидалось 0", sent)
	}
}

func TestSendDueRetriesWithExponentialBackoff(t *testing.T) {
	store := newMemoryStore(testSubscriber())
	provider := &stubProvider{forecast: twoDayForecast(t), failFor: 3}
	clock := fixedClock(time.Date(2026, 9, 10, 7, 0, 0, 0, berlin(t)))
	notifier := &stubNotifier{}

	sleep, delays := noSleep()
	sender := NewSendReports(store, NewBuildReport(provider, clock, quietLogger()), stubRenderer{},
		notifier, clock, Retry{Attempts: 5, Base: time.Minute, Sleep: sleep}, quietLogger())

	sent, err := sender.SendDue(context.Background())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if sent != 1 {
		t.Errorf("отправлено = %d, ожидалось 1 после ретраев", sent)
	}
	if provider.calls != 4 {
		t.Errorf("запросов к API = %d, ожидалось 4", provider.calls)
	}

	want := []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute}
	if len(*delays) != len(want) {
		t.Fatalf("задержки = %v, ожидалось %v", *delays, want)
	}
	for i, delay := range want {
		if (*delays)[i] != delay {
			t.Errorf("задержка %d = %v, ожидалось %v", i, (*delays)[i], delay)
		}
	}
}

func TestSendDueGivesUpAndWarnsUser(t *testing.T) {
	store := newMemoryStore(testSubscriber())
	provider := &stubProvider{failFor: 99}
	clock := fixedClock(time.Date(2026, 9, 10, 7, 0, 0, 0, berlin(t)))
	notifier := &stubNotifier{}

	sleep, delays := noSleep()
	sender := NewSendReports(store, NewBuildReport(provider, clock, quietLogger()), stubRenderer{},
		notifier, clock, Retry{Attempts: 5, Base: time.Minute, Sleep: sleep}, quietLogger())

	sent, err := sender.SendDue(context.Background())
	if err != nil {
		t.Fatalf("рассылка не должна возвращать ошибку: %v", err)
	}
	if sent != 0 {
		t.Errorf("отправлено = %d, ожидалось 0", sent)
	}
	if provider.calls != 5 {
		t.Errorf("попыток = %d, ожидалось 5", provider.calls)
	}
	if len(*delays) != 4 {
		t.Errorf("задержек = %d, ожидалось 4", len(*delays))
	}
	if len(notifier.sent) != 1 || notifier.sent[0] != string(i18n.KeyMorningFailed) {
		t.Errorf("пользователю не пришло сообщение об ошибке: %v", notifier.sent)
	}

	subscriber, _ := store.Get(context.Background(), 42)
	if subscriber.LastSentDate != "" {
		t.Errorf("при неудаче дата рассылки не должна меняться: %q", subscriber.LastSentDate)
	}
}

func TestSendDueDeletesBlockedSubscriber(t *testing.T) {
	store := newMemoryStore(testSubscriber())
	provider := &stubProvider{forecast: twoDayForecast(t)}
	clock := fixedClock(time.Date(2026, 9, 10, 7, 0, 0, 0, berlin(t)))
	notifier := &stubNotifier{err: ErrBlocked}

	sleep, _ := noSleep()
	sender := NewSendReports(store, NewBuildReport(provider, clock, quietLogger()), stubRenderer{},
		notifier, clock, Retry{Attempts: 1, Base: time.Minute, Sleep: sleep}, quietLogger())

	if _, err := sender.SendDue(context.Background()); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if count, _ := store.Count(context.Background()); count != 0 {
		t.Errorf("подписчиков = %d, заблокировавший должен быть удалён", count)
	}
}

func TestSendDueStopsOnCancelledContext(t *testing.T) {
	store := newMemoryStore(testSubscriber())
	provider := &stubProvider{forecast: twoDayForecast(t)}
	clock := fixedClock(time.Date(2026, 9, 10, 7, 0, 0, 0, berlin(t)))

	sleep, _ := noSleep()
	sender := NewSendReports(store, NewBuildReport(provider, clock, quietLogger()), stubRenderer{},
		&stubNotifier{}, clock, Retry{Attempts: 1, Base: time.Minute, Sleep: sleep}, quietLogger())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := sender.SendDue(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("ошибка = %v, ожидалась отмена контекста", err)
	}
}

func TestSendDueUsesSubscriberTimezoneForDate(t *testing.T) {
	subscriber := testSubscriber()
	subscriber.TZName = "Pacific/Auckland"
	store := newMemoryStore(subscriber)

	provider := &stubProvider{forecast: twoDayForecast(t)}
	clock := fixedClock(time.Date(2026, 9, 10, 21, 0, 0, 0, time.UTC))

	sleep, _ := noSleep()
	sender := NewSendReports(store, NewBuildReport(provider, clock, quietLogger()), stubRenderer{},
		&stubNotifier{}, clock, Retry{Attempts: 1, Base: time.Minute, Sleep: sleep}, quietLogger())

	if _, err := sender.SendDue(context.Background()); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	stored, _ := store.Get(context.Background(), 42)
	if stored.LastSentDate != "2026-09-11" {
		t.Errorf("дата рассылки = %q, в Окленде уже 11 сентября", stored.LastSentDate)
	}
}

func TestDefaultRetrySpansAboutFifteenMinutes(t *testing.T) {
	retry := DefaultRetry()
	total := time.Duration(0)
	delay := retry.Base
	for attempt := 1; attempt < retry.Attempts; attempt++ {
		total += delay
		delay *= 2
	}
	if total < 14*time.Minute || total > 16*time.Minute {
		t.Errorf("суммарная задержка = %v, ожидалось около 15 минут", total)
	}
}
