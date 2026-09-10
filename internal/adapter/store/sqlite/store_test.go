package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/port"
)

func newStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "nested", "weatherfit.db"))
	if err != nil {
		t.Fatalf("не удалось открыть базу: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("не удалось закрыть базу: %v", err)
		}
	})
	return store, ctx
}

func sample(chatID int64) domain.Subscriber {
	now := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	return domain.Subscriber{
		ChatID:      chatID,
		Place:       domain.Location{Name: "Дрезден", Latitude: 51.05, Longitude: 13.74},
		TZName:      "Europe/Berlin",
		ReportTime:  domain.DayTime{Hour: 7, Minute: 30},
		ActiveHours: domain.HourWindow{Start: 7, End: 22},
		WindUnit:    domain.WindUnitMS,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestSaveAndGet(t *testing.T) {
	store, ctx := newStore(t)
	want := sample(42)

	if err := store.Save(ctx, want); err != nil {
		t.Fatalf("не удалось сохранить: %v", err)
	}

	got, err := store.Get(ctx, 42)
	if err != nil {
		t.Fatalf("не удалось прочитать: %v", err)
	}
	if got.ChatID != want.ChatID || got.Place != want.Place || got.TZName != want.TZName {
		t.Errorf("подписчик прочитан иначе: %+v", got)
	}
	if got.ReportTime != want.ReportTime || got.ActiveHours != want.ActiveHours {
		t.Errorf("настройки времени = %v, %v", got.ReportTime, got.ActiveHours)
	}
	if got.WindUnit != domain.WindUnitMS || got.Paused || got.LastSentDate != "" || got.Pending != domain.PendingNone {
		t.Errorf("значения по умолчанию неверны: %+v", got)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("created_at = %v, ожидалось %v", got.CreatedAt, want.CreatedAt)
	}
}

func TestGetMissingSubscriber(t *testing.T) {
	store, ctx := newStore(t)
	_, err := store.Get(ctx, 777)
	if !errors.Is(err, port.ErrSubscriberNotFound) {
		t.Errorf("ошибка = %v, ожидалась ErrSubscriberNotFound", err)
	}
}

func TestSaveUpdatesExisting(t *testing.T) {
	store, ctx := newStore(t)
	original := sample(42)
	if err := store.Save(ctx, original); err != nil {
		t.Fatalf("не удалось сохранить: %v", err)
	}

	changed := original
	changed.Place = domain.Location{Name: "Прага", Latitude: 50.08, Longitude: 14.44}
	changed.TZName = "Europe/Prague"
	changed.ReportTime = domain.DayTime{Hour: 6}
	changed.WindUnit = domain.WindUnitKMH
	changed.Paused = true
	changed.UpdatedAt = original.UpdatedAt.Add(time.Hour)

	if err := store.Save(ctx, changed); err != nil {
		t.Fatalf("не удалось обновить: %v", err)
	}

	got, err := store.Get(ctx, 42)
	if err != nil {
		t.Fatalf("не удалось прочитать: %v", err)
	}
	if got.Place.Name != "Прага" || got.TZName != "Europe/Prague" {
		t.Errorf("город не обновился: %+v", got.Place)
	}
	if got.ReportTime.String() != "06:00" || got.WindUnit != domain.WindUnitKMH || !got.Paused {
		t.Errorf("настройки не обновились: %+v", got)
	}
	if !got.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("created_at изменился: %v", got.CreatedAt)
	}
	if count, err := store.Count(ctx); err != nil || count != 1 {
		t.Errorf("подписчиков = %d, %v, ожидался один", count, err)
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	store, ctx := newStore(t)
	broken := sample(42)
	broken.TZName = "Mars/Olympus"
	if err := store.Save(ctx, broken); err == nil {
		t.Error("ожидалась ошибка валидации")
	}
	if count, _ := store.Count(ctx); count != 0 {
		t.Errorf("подписчиков = %d, ожидался ноль", count)
	}
}

func TestAllAndDelete(t *testing.T) {
	store, ctx := newStore(t)
	for _, chatID := range []int64{3, 1, 2} {
		if err := store.Save(ctx, sample(chatID)); err != nil {
			t.Fatalf("не удалось сохранить %d: %v", chatID, err)
		}
	}

	all, err := store.All(ctx)
	if err != nil {
		t.Fatalf("не удалось прочитать всех: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("подписчиков = %d, ожидалось 3", len(all))
	}
	if all[0].ChatID != 1 || all[2].ChatID != 3 {
		t.Errorf("порядок = %d, %d, %d", all[0].ChatID, all[1].ChatID, all[2].ChatID)
	}

	if err := store.Delete(ctx, 2); err != nil {
		t.Fatalf("не удалось удалить: %v", err)
	}
	if count, _ := store.Count(ctx); count != 2 {
		t.Errorf("после удаления подписчиков = %d, ожидалось 2", count)
	}
	if err := store.Delete(ctx, 999); err != nil {
		t.Errorf("удаление отсутствующего не должно быть ошибкой: %v", err)
	}
}

func TestMarkSentAndPending(t *testing.T) {
	store, ctx := newStore(t)
	if err := store.Save(ctx, sample(42)); err != nil {
		t.Fatalf("не удалось сохранить: %v", err)
	}

	if err := store.MarkSent(ctx, 42, "2026-09-10"); err != nil {
		t.Fatalf("не удалось отметить рассылку: %v", err)
	}
	got, err := store.Get(ctx, 42)
	if err != nil {
		t.Fatalf("не удалось прочитать: %v", err)
	}
	if got.LastSentDate != "2026-09-10" {
		t.Errorf("дата рассылки = %q", got.LastSentDate)
	}

	if err := store.SetPending(ctx, 42, domain.PendingCity); err != nil {
		t.Fatalf("не удалось сохранить ожидание: %v", err)
	}
	got, _ = store.Get(ctx, 42)
	if got.Pending != domain.PendingCity {
		t.Errorf("ожидание = %q", got.Pending)
	}
	if got.LastSentDate != "2026-09-10" {
		t.Errorf("дата рассылки потерялась: %q", got.LastSentDate)
	}

	if err := store.SetPending(ctx, 42, domain.PendingNone); err != nil {
		t.Fatalf("не удалось сбросить ожидание: %v", err)
	}
	got, _ = store.Get(ctx, 42)
	if got.Pending != domain.PendingNone {
		t.Errorf("ожидание не сброшено: %q", got.Pending)
	}
}

func TestStoreSurvivesReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "weatherfit.db")

	first, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("не удалось открыть базу: %v", err)
	}
	if err := first.Save(ctx, sample(42)); err != nil {
		t.Fatalf("не удалось сохранить: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("не удалось закрыть базу: %v", err)
	}

	second, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("не удалось переоткрыть базу: %v", err)
	}
	defer func() { _ = second.Close() }()

	if _, err := second.Get(ctx, 42); err != nil {
		t.Errorf("подписчик не сохранился между запусками: %v", err)
	}
}

func TestStoreImplementsPort(t *testing.T) {
	store, _ := newStore(t)
	var _ port.SubscriberStore = store
}
