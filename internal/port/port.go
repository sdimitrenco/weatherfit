// Package port описывает интерфейсы, через которые usecase общается с внешним миром.
package port

import (
	"context"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
)

// ForecastProvider отдаёт прогноз для точки, заданной при создании адаптера.
type ForecastProvider interface {
	Forecast(ctx context.Context, days int) (domain.Forecast, error)
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
