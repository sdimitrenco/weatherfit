// Package scheduler runs the morning delivery on a wall-clock tick.
package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/port"
)

// Sender is the delivery use case the scheduler drives.
type Sender interface {
	SendDue(ctx context.Context) (int, error)
}

// DefaultInterval is how often subscribers are checked. Every subscriber has
// their own report time and timezone, so the tick has to be minute-grained.
const DefaultInterval = time.Minute

// Options wires the scheduler.
type Options struct {
	Sender   Sender
	Clock    port.Clock
	Interval time.Duration
	Sleep    func(ctx context.Context, delay time.Duration) error
	Log      *slog.Logger
}

// Scheduler checks every minute who is due and sends their report.
type Scheduler struct {
	sender   Sender
	clock    port.Clock
	interval time.Duration
	sleep    func(ctx context.Context, delay time.Duration) error
	log      *slog.Logger
}

// New creates the scheduler, filling in defaults.
func New(options Options) *Scheduler {
	if options.Interval <= 0 {
		options.Interval = DefaultInterval
	}
	if options.Sleep == nil {
		options.Sleep = sleepContext
	}
	return &Scheduler{
		sender:   options.Sender,
		clock:    options.Clock,
		interval: options.Interval,
		sleep:    options.Sleep,
		log:      options.Log,
	}
}

// Run sends anything already due, then keeps checking until the context ends.
// The first pass is what delivers a report missed while the service was down.
func (s *Scheduler) Run(ctx context.Context) error {
	s.tick(ctx)

	for {
		delay := untilNextTick(s.clock.Now(), s.interval)
		if err := s.sleep(ctx, delay); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
		s.tick(ctx)
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	sent, err := s.sender.SendDue(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		s.log.Error("the delivery check failed", slog.String("error", err.Error()))
		return
	}
	if sent > 0 {
		s.log.Info("morning reports sent", slog.Int("count", sent))
	}
}

func untilNextTick(now time.Time, interval time.Duration) time.Duration {
	next := now.Truncate(interval).Add(interval)
	delay := next.Sub(now)
	if delay <= 0 {
		return interval
	}
	return delay
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
