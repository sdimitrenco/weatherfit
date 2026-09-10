package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/port"
)

type countingSender struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (c *countingSender) SendDue(context.Context) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.err != nil {
		return 0, c.err
	}
	return 1, nil
}

func (c *countingSender) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunSendsImmediatelyThenOnEveryTick(t *testing.T) {
	sender := &countingSender{}
	now := time.Date(2026, 9, 10, 6, 59, 30, 0, time.UTC)

	var delays []time.Duration
	ctx, cancel := context.WithCancel(context.Background())

	scheduler := New(Options{
		Sender: sender,
		Clock:  port.ClockFunc(func() time.Time { return now }),
		Sleep: func(_ context.Context, delay time.Duration) error {
			delays = append(delays, delay)
			now = now.Add(delay)
			if len(delays) == 3 {
				cancel()
				return context.Canceled
			}
			return nil
		},
		Log: quietLogger(),
	})

	if err := scheduler.Run(ctx); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	if sender.count() != 3 {
		t.Errorf("проверок = %d, ожидалось 3 (старт и два тика)", sender.count())
	}
	if delays[0] != 30*time.Second {
		t.Errorf("первая задержка = %v, ожидалось 30 s до ровной минуты", delays[0])
	}
	if delays[1] != time.Minute {
		t.Errorf("вторая задержка = %v, ожидалась минута", delays[1])
	}
}

func TestRunKeepsGoingAfterSenderError(t *testing.T) {
	sender := &countingSender{err: errors.New("база недоступна")}
	now := time.Date(2026, 9, 10, 7, 0, 0, 0, time.UTC)

	ticks := 0
	scheduler := New(Options{
		Sender: sender,
		Clock:  port.ClockFunc(func() time.Time { return now }),
		Sleep: func(_ context.Context, delay time.Duration) error {
			ticks++
			now = now.Add(delay)
			if ticks == 2 {
				return context.Canceled
			}
			return nil
		},
		Log: quietLogger(),
	})

	if err := scheduler.Run(context.Background()); err != nil {
		t.Fatalf("ошибка отправки не должна останавливать планировщик: %v", err)
	}
	if sender.count() != 2 {
		t.Errorf("проверок = %d, ожидалось 2", sender.count())
	}
}

func TestRunStopsOnCancelledContext(t *testing.T) {
	sender := &countingSender{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	scheduler := New(Options{
		Sender: sender,
		Clock:  port.ClockFunc(time.Now),
		Log:    quietLogger(),
	})

	done := make(chan error, 1)
	go func() { done <- scheduler.Run(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("ошибка = %v, ожидался выход без ошибки", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("планировщик не остановился по отмене контекста")
	}
}

func TestUntilNextTick(t *testing.T) {
	tests := []struct {
		now      time.Time
		interval time.Duration
		want     time.Duration
	}{
		{now: time.Date(2026, 9, 10, 6, 59, 30, 0, time.UTC), interval: time.Minute, want: 30 * time.Second},
		{now: time.Date(2026, 9, 10, 7, 0, 0, 0, time.UTC), interval: time.Minute, want: time.Minute},
		{now: time.Date(2026, 9, 10, 7, 0, 0, 1, time.UTC), interval: time.Minute, want: time.Minute - time.Nanosecond},
		{now: time.Date(2026, 9, 10, 7, 4, 0, 0, time.UTC), interval: 5 * time.Minute, want: time.Minute},
	}

	for _, tc := range tests {
		if got := untilNextTick(tc.now, tc.interval); got != tc.want {
			t.Errorf("untilNextTick(%v, %v) = %v, ожидалось %v", tc.now, tc.interval, got, tc.want)
		}
	}
}

func TestSleepContextRespectsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("ошибка = %v, ожидалась отмена", err)
	}
}
