// Package usecase holds the application logic that glues ports together.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/port"
)

// ReportKind selects which slice of the forecast a report covers.
type ReportKind int

const (
	// ReportMorning covers the whole active window of today.
	ReportMorning ReportKind = iota
	// ReportToday covers today from the current hour on.
	ReportToday
	// ReportTomorrow covers the whole active window of tomorrow.
	ReportTomorrow
)

const forecastDays = 2

// BuildReport turns a subscriber plus a forecast into a report.
type BuildReport struct {
	forecasts port.ForecastProvider
	clock     port.Clock
	log       *slog.Logger
}

// NewBuildReport wires the use case.
func NewBuildReport(forecasts port.ForecastProvider, clock port.Clock, log *slog.Logger) *BuildReport {
	return &BuildReport{forecasts: forecasts, clock: clock, log: log}
}

// Build fetches the forecast and analyzes the hours the kind asks for.
func (b *BuildReport) Build(ctx context.Context, subscriber domain.Subscriber, kind ReportKind) (domain.Report, error) {
	forecast, err := b.fetch(ctx, subscriber)
	if err != nil {
		return domain.Report{}, err
	}

	location := subscriber.Timezone()
	now := b.clock.Now().In(location)

	date := now
	if kind == ReportTomorrow {
		date = now.AddDate(0, 0, 1)
	}

	day, _ := forecast.Day(date)
	hours := activeHours(forecast, subscriber, date, kind, now)
	if len(hours) == 0 {
		return domain.Report{}, fmt.Errorf("usecase: the forecast has no hours for %s", date.Format(time.DateOnly))
	}

	report := domain.Analyze(subscriber.Place, date, day, hours)
	if len(report.UnknownCodes) > 0 {
		b.log.Warn("unknown WMO weather codes",
			slog.Int64("chat_id", subscriber.ChatID),
			slog.Any("codes", report.UnknownCodes),
		)
	}
	return report, nil
}

// Current returns the observed weather for the subscriber place.
func (b *BuildReport) Current(ctx context.Context, subscriber domain.Subscriber) (domain.CurrentPoint, error) {
	forecast, err := b.fetch(ctx, subscriber)
	if err != nil {
		return domain.CurrentPoint{}, err
	}

	current, ok := forecast.Current.Get()
	if !ok {
		return domain.CurrentPoint{}, errors.New("usecase: the response carries no current weather")
	}
	return current, nil
}

func (b *BuildReport) fetch(ctx context.Context, subscriber domain.Subscriber) (domain.Forecast, error) {
	return b.forecasts.Forecast(ctx, port.ForecastRequest{
		Place:    subscriber.Place,
		Timezone: subscriber.Timezone(),
		Days:     forecastDays,
	})
}

func activeHours(forecast domain.Forecast, subscriber domain.Subscriber, date time.Time, kind ReportKind, now time.Time) []domain.HourPoint {
	window := subscriber.ActiveHours
	hours := make([]domain.HourPoint, 0, 24)

	for _, hour := range forecast.HoursOfDay(date) {
		if !window.Contains(hour.Time.Hour()) {
			continue
		}
		if kind == ReportToday && hour.Time.Hour() < now.Hour() {
			continue
		}
		hours = append(hours, hour)
	}
	return hours
}
