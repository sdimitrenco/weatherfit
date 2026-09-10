package usecase

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestBuildMorningCoversWholeActiveWindow(t *testing.T) {
	provider := &stubProvider{forecast: twoDayForecast(t)}
	clock := fixedClock(time.Date(2026, 9, 10, 7, 0, 0, 0, berlin(t)))
	build := NewBuildReport(provider, clock, quietLogger())

	report, err := build.Build(context.Background(), testSubscriber(), ReportMorning)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	if len(report.Hours) != 16 {
		t.Errorf("часов = %d, ожидалось 16 (с 07 до 22)", len(report.Hours))
	}
	if first := report.Hours[0].Time.Hour(); first != 7 {
		t.Errorf("первый час = %d, ожидалось 7", first)
	}
	if last := report.Hours[len(report.Hours)-1].Time.Hour(); last != 22 {
		t.Errorf("последний час = %d, ожидалось 22", last)
	}
	if report.Date.Format(time.DateOnly) != "2026-09-10" {
		t.Errorf("дата = %q", report.Date.Format(time.DateOnly))
	}
	if maximum, _ := report.Day.TemperatureMaxC.Get(); maximum != 22 {
		t.Errorf("сводка дня не подхватилась: %v", maximum)
	}
}

func TestBuildTodayStartsFromCurrentHour(t *testing.T) {
	provider := &stubProvider{forecast: twoDayForecast(t)}
	clock := fixedClock(time.Date(2026, 9, 10, 15, 30, 0, 0, berlin(t)))
	build := NewBuildReport(provider, clock, quietLogger())

	report, err := build.Build(context.Background(), testSubscriber(), ReportToday)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	if first := report.Hours[0].Time.Hour(); first != 15 {
		t.Errorf("первый час = %d, ожидалось 15", first)
	}
	if len(report.Hours) != 8 {
		t.Errorf("часов = %d, ожидалось 8 (с 15 до 22)", len(report.Hours))
	}
}

func TestBuildTomorrowCoversNextDay(t *testing.T) {
	provider := &stubProvider{forecast: twoDayForecast(t)}
	clock := fixedClock(time.Date(2026, 9, 10, 20, 0, 0, 0, berlin(t)))
	build := NewBuildReport(provider, clock, quietLogger())

	report, err := build.Build(context.Background(), testSubscriber(), ReportTomorrow)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	if report.Date.Format(time.DateOnly) != "2026-09-11" {
		t.Errorf("дата = %q, ожидалось 2026-09-11", report.Date.Format(time.DateOnly))
	}
	if len(report.Hours) != 16 {
		t.Errorf("часов = %d, ожидалось 16", len(report.Hours))
	}
	if maximum, _ := report.Day.TemperatureMaxC.Get(); maximum != 18 {
		t.Errorf("сводка дня = %v, ожидалось 18 (завтра)", maximum)
	}
}

func TestBuildRespectsSubscriberWindow(t *testing.T) {
	subscriber := testSubscriber()
	subscriber.ActiveHours = domain.HourWindow{Start: 9, End: 12}

	provider := &stubProvider{forecast: twoDayForecast(t)}
	clock := fixedClock(time.Date(2026, 9, 10, 7, 0, 0, 0, berlin(t)))
	build := NewBuildReport(provider, clock, quietLogger())

	report, err := build.Build(context.Background(), subscriber, ReportMorning)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(report.Hours) != 4 {
		t.Errorf("часов = %d, ожидалось 4", len(report.Hours))
	}
}

func TestBuildUsesSubscriberTimezone(t *testing.T) {
	subscriber := testSubscriber()
	subscriber.TZName = "Asia/Tokyo"

	provider := &stubProvider{forecast: twoDayForecast(t)}
	clock := fixedClock(time.Date(2026, 9, 10, 7, 0, 0, 0, berlin(t)))
	build := NewBuildReport(provider, clock, quietLogger())

	if _, err := build.Build(context.Background(), subscriber, ReportMorning); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
}

func TestBuildFailsWhenProviderFails(t *testing.T) {
	provider := &stubProvider{failFor: 1}
	clock := fixedClock(time.Date(2026, 9, 10, 7, 0, 0, 0, berlin(t)))
	build := NewBuildReport(provider, clock, quietLogger())

	if _, err := build.Build(context.Background(), testSubscriber(), ReportMorning); err == nil {
		t.Fatal("ожидалась ошибка, её нет")
	}
}

func TestBuildFailsWhenNoHoursInWindow(t *testing.T) {
	subscriber := testSubscriber()
	subscriber.ActiveHours = domain.HourWindow{Start: 7, End: 8}

	provider := &stubProvider{forecast: twoDayForecast(t)}
	clock := fixedClock(time.Date(2026, 9, 10, 20, 0, 0, 0, berlin(t)))
	build := NewBuildReport(provider, clock, quietLogger())

	if _, err := build.Build(context.Background(), subscriber, ReportToday); err == nil {
		t.Fatal("ожидалась ошибка: в окне не осталось часов")
	}
}

func TestCurrentWeather(t *testing.T) {
	provider := &stubProvider{forecast: twoDayForecast(t)}
	clock := fixedClock(time.Date(2026, 9, 10, 10, 20, 0, 0, berlin(t)))
	build := NewBuildReport(provider, clock, quietLogger())

	current, err := build.Current(context.Background(), testSubscriber())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if temperature, _ := current.TemperatureC.Get(); temperature != 15.7 {
		t.Errorf("температура = %v, ожидалось 15.7", temperature)
	}
}

func TestCurrentWeatherMissing(t *testing.T) {
	forecast := twoDayForecast(t)
	forecast.Current = domain.None[domain.CurrentPoint]()

	provider := &stubProvider{forecast: forecast}
	clock := fixedClock(time.Date(2026, 9, 10, 10, 20, 0, 0, berlin(t)))
	build := NewBuildReport(provider, clock, quietLogger())

	if _, err := build.Current(context.Background(), testSubscriber()); err == nil {
		t.Fatal("ожидалась ошибка: текущей погоды нет в ответе")
	}
}
