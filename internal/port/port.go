// Package port declares the interfaces the use cases talk to the world through.
package port

import (
	"context"
	"errors"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
)

// ForecastRequest is what the forecast provider is asked for.
type ForecastRequest struct {
	Place    domain.Location
	Timezone *time.Location
	Days     int
}

// ForecastProvider returns a forecast for a point.
type ForecastProvider interface {
	Forecast(ctx context.Context, request ForecastRequest) (domain.Forecast, error)
}

// SubscriberStore persists subscribers and their settings.
type SubscriberStore interface {
	Save(ctx context.Context, subscriber domain.Subscriber) error
	Get(ctx context.Context, chatID int64) (domain.Subscriber, error)
	All(ctx context.Context) ([]domain.Subscriber, error)
	Delete(ctx context.Context, chatID int64) error
	MarkSent(ctx context.Context, chatID int64, date string) error
	SetPending(ctx context.Context, chatID int64, pending domain.PendingAction) error
	Count(ctx context.Context) (int, error)
}

// ErrSubscriberNotFound is returned when the store holds no such subscriber.
var ErrSubscriberNotFound = errors.New("подписчик не найден")

// Place is a populated place returned by the geocoder.
type Place struct {
	Name    string
	Country string
	Admin   string
	TZName  string
	Place   domain.Location
}

// Geocoder searches places by name.
type Geocoder interface {
	Search(ctx context.Context, query string, limit int) ([]Place, error)
}

// TimezoneResolver resolves a timezone from coordinates, which is what a
// Telegram location share gives us instead of a city name.
type TimezoneResolver interface {
	ResolveTimezone(ctx context.Context, place domain.Location) (string, error)
}

// Notifier delivers a rendered message to a recipient.
type Notifier interface {
	Send(ctx context.Context, chatID int64, text string) error
}

// ReportRenderer turns domain data into a message in the subscriber language.
type ReportRenderer interface {
	Report(report domain.Report, subscriber domain.Subscriber) string
	Current(place domain.Location, current domain.CurrentPoint, subscriber domain.Subscriber) string
	Text(subscriber domain.Subscriber, key string, args ...any) string
}

// Clock reports the current time so that "now" can be faked in tests.
type Clock interface {
	Now() time.Time
}

// ClockFunc adapts a function to Clock.
type ClockFunc func() time.Time

// Now returns the current time.
func (f ClockFunc) Now() time.Time {
	return f()
}

// SystemClock returns a clock reporting system time in the given location.
func SystemClock(location *time.Location) Clock {
	return ClockFunc(func() time.Time {
		return time.Now().In(location)
	})
}
