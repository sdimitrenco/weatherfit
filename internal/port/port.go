// Package port описывает интерфейсы, через которые usecase общается с внешним миром.
package port

import (
	"context"
	"errors"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
)

// ForecastRequest — что именно запрашивается у провайдера прогноза.
type ForecastRequest struct {
	Place    domain.Location
	Timezone *time.Location
	Days     int
}

// ForecastProvider отдаёт прогноз для точки.
type ForecastProvider interface {
	Forecast(ctx context.Context, request ForecastRequest) (domain.Forecast, error)
}

// SubscriberStore хранит подписчиков и их настройки.
type SubscriberStore interface {
	Save(ctx context.Context, subscriber domain.Subscriber) error
	Get(ctx context.Context, chatID int64) (domain.Subscriber, error)
	All(ctx context.Context) ([]domain.Subscriber, error)
	Delete(ctx context.Context, chatID int64) error
	MarkSent(ctx context.Context, chatID int64, date string) error
	SetPending(ctx context.Context, chatID int64, pending domain.PendingAction) error
	Count(ctx context.Context) (int, error)
}

// ErrSubscriberNotFound возвращается, когда подписчика нет в хранилище.
var ErrSubscriberNotFound = errors.New("подписчик не найден")

// Place — найденный геокодером населённый пункт.
type Place struct {
	Name    string
	Country string
	Admin   string
	TZName  string
	Place   domain.Location
}

// Geocoder ищет населённые пункты по названию.
type Geocoder interface {
	Search(ctx context.Context, query string, limit int) ([]Place, error)
}

// TimezoneResolver определяет таймзону по координатам. Нужен для геопозиции,
// присланной из Telegram, у которой нет названия города.
type TimezoneResolver interface {
	ResolveTimezone(ctx context.Context, place domain.Location) (string, error)
}

// Notifier отправляет готовое сообщение получателю.
type Notifier interface {
	Send(ctx context.Context, chatID int64, text string) error
}

// Clock отдаёт текущее время, чтобы «сейчас» можно было подменить в тестах.
type Clock interface {
	Now() time.Time
}

// ClockFunc адаптирует функцию к интерфейсу Clock.
type ClockFunc func() time.Time

// Now возвращает текущее время.
func (f ClockFunc) Now() time.Time {
	return f()
}

// SystemClock возвращает часы, отдающие системное время в заданной локации.
func SystemClock(location *time.Location) Clock {
	return ClockFunc(func() time.Time {
		return time.Now().In(location)
	})
}
