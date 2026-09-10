package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/i18n"
	"github.com/sdimitrenco/weatherfit/internal/port"
)

// Retry describes the backoff used when Open-Meteo is unavailable.
type Retry struct {
	Attempts int
	Base     time.Duration
	Sleep    func(ctx context.Context, delay time.Duration) error
}

// DefaultRetry gives five attempts spread over about fifteen minutes.
func DefaultRetry() Retry {
	return Retry{Attempts: 5, Base: time.Minute, Sleep: sleepContext}
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// ErrBlocked marks a recipient that can no longer receive messages.
var ErrBlocked = errors.New("the recipient blocked the bot")

// SendReports delivers the morning report to every subscriber that is due.
type SendReports struct {
	store    port.SubscriberStore
	reports  *BuildReport
	renderer port.ReportRenderer
	notifier port.Notifier
	clock    port.Clock
	retry    Retry
	log      *slog.Logger
}

// NewSendReports wires the use case.
func NewSendReports(
	store port.SubscriberStore,
	reports *BuildReport,
	renderer port.ReportRenderer,
	notifier port.Notifier,
	clock port.Clock,
	retry Retry,
	log *slog.Logger,
) *SendReports {
	if retry.Attempts < 1 {
		retry = DefaultRetry()
	}
	if retry.Sleep == nil {
		retry.Sleep = sleepContext
	}
	return &SendReports{
		store:    store,
		reports:  reports,
		renderer: renderer,
		notifier: notifier,
		clock:    clock,
		retry:    retry,
		log:      log,
	}
}

// SendDue sends the report to everyone whose local report time has come and
// who has not received today's report yet. It returns how many were sent.
func (s *SendReports) SendDue(ctx context.Context) (int, error) {
	subscribers, err := s.store.All(ctx)
	if err != nil {
		return 0, err
	}

	now := s.clock.Now()
	sent := 0
	for _, subscriber := range subscribers {
		if err := ctx.Err(); err != nil {
			return sent, err
		}
		if !subscriber.DueAt(now) {
			continue
		}
		if err := s.sendOne(ctx, subscriber); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return sent, err
			}
			s.log.Error("morning delivery failed",
				slog.Int64("chat_id", subscriber.ChatID),
				slog.String("error", err.Error()),
			)
			continue
		}
		sent++
	}
	return sent, nil
}

func (s *SendReports) sendOne(ctx context.Context, subscriber domain.Subscriber) error {
	report, err := s.buildWithRetries(ctx, subscriber)
	if err != nil {
		if notifyErr := s.notifier.Send(ctx, subscriber.ChatID,
			s.renderer.Text(subscriber, string(i18n.KeyMorningFailed))); notifyErr != nil {
			s.dropIfBlocked(ctx, subscriber, notifyErr)
			return fmt.Errorf("no forecast (%w) and the failure notice did not go out either: %w", err, notifyErr)
		}
		return err
	}

	if err := s.notifier.Send(ctx, subscriber.ChatID, s.renderer.Report(report, subscriber)); err != nil {
		s.dropIfBlocked(ctx, subscriber, err)
		return err
	}

	date := s.clock.Now().In(subscriber.Timezone()).Format(time.DateOnly)
	if err := s.store.MarkSent(ctx, subscriber.ChatID, date); err != nil {
		return err
	}

	s.log.Info("morning report sent",
		slog.Int64("chat_id", subscriber.ChatID),
		slog.String("place", subscriber.Place.Name),
		slog.String("date", date),
	)
	return nil
}

func (s *SendReports) buildWithRetries(ctx context.Context, subscriber domain.Subscriber) (domain.Report, error) {
	delay := s.retry.Base
	var lastErr error

	for attempt := 1; attempt <= s.retry.Attempts; attempt++ {
		report, err := s.reports.Build(ctx, subscriber, ReportMorning)
		if err == nil {
			return report, nil
		}
		lastErr = err

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return domain.Report{}, err
		}
		if attempt == s.retry.Attempts {
			break
		}

		s.log.Warn("no forecast, retrying",
			slog.Int64("chat_id", subscriber.ChatID),
			slog.Int("attempt", attempt),
			slog.Duration("delay", delay),
			slog.String("error", err.Error()),
		)
		if err := s.retry.Sleep(ctx, delay); err != nil {
			return domain.Report{}, err
		}
		delay *= 2
	}
	return domain.Report{}, fmt.Errorf("no forecast after %d attempts: %w", s.retry.Attempts, lastErr)
}

func (s *SendReports) dropIfBlocked(ctx context.Context, subscriber domain.Subscriber, err error) {
	if !errors.Is(err, ErrBlocked) {
		return
	}
	if deleteErr := s.store.Delete(ctx, subscriber.ChatID); deleteErr != nil {
		s.log.Error("cannot delete the subscriber who blocked the bot",
			slog.Int64("chat_id", subscriber.ChatID),
			slog.String("error", deleteErr.Error()),
		)
		return
	}
	s.log.Info("subscriber removed: the bot is blocked", slog.Int64("chat_id", subscriber.ChatID))
}
