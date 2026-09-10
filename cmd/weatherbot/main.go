package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	_ "time/tzdata"

	"github.com/sdimitrenco/weatherfit/internal/adapter/geocode"
	"github.com/sdimitrenco/weatherfit/internal/adapter/openmeteo"
	"github.com/sdimitrenco/weatherfit/internal/adapter/render"
	"github.com/sdimitrenco/weatherfit/internal/adapter/scheduler"
	"github.com/sdimitrenco/weatherfit/internal/adapter/store/sqlite"
	"github.com/sdimitrenco/weatherfit/internal/adapter/telegram"
	"github.com/sdimitrenco/weatherfit/internal/config"
	"github.com/sdimitrenco/weatherfit/internal/port"
	"github.com/sdimitrenco/weatherfit/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("cannot read configuration:\n%w", err)
	}

	log := newLogger(cfg.LogLevel)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := sqlite.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Error("cannot close the database", slog.String("error", err.Error()))
		}
	}()

	layout, err := render.ParseLayout(cfg.HourlyLayout)
	if err != nil {
		return fmt.Errorf("HOURLY_LAYOUT: %w", err)
	}

	forecasts := openmeteo.New(openmeteo.Options{})
	geocoder := geocode.New(geocode.Options{})
	renderer := render.NewRenderer(layout)
	clock := port.SystemClock(cfg.DefaultTimezone)

	subscriptions := usecase.NewSubscriptions(store, geocoder, forecasts, clock, usecase.Defaults{
		Place:       cfg.DefaultPlace,
		TZName:      cfg.DefaultTZName,
		ReportTime:  cfg.DefaultReportTime,
		ActiveHours: cfg.DefaultActiveHours,
		WindUnit:    cfg.DefaultWindUnit,
		Lang:        cfg.DefaultLang,
	})
	reports := usecase.NewBuildReport(forecasts, clock, log)

	telegramBot, err := telegram.New(telegram.Options{
		Token:         cfg.TelegramBotToken,
		Access:        cfg,
		Subscriptions: subscriptions,
		Reports:       reports,
		Renderer:      renderer,
		Log:           log,
	})
	if err != nil {
		return err
	}

	sender := usecase.NewSendReports(store, reports, renderer, telegramBot, clock, usecase.DefaultRetry(), log)
	morning := scheduler.New(scheduler.Options{
		Sender: sender,
		Clock:  clock,
		Log:    log,
	})

	subscribers, err := store.Count(ctx)
	if err != nil {
		return err
	}
	log.Info("bot started",
		slog.String("default_place", cfg.DefaultPlace.Name),
		slog.String("default_tz", cfg.DefaultTZName),
		slog.String("default_report_time", cfg.DefaultReportTime.String()),
		slog.String("default_lang", string(cfg.DefaultLang)),
		slog.String("hourly_layout", cfg.HourlyLayout),
		slog.Bool("private", cfg.Private()),
		slog.Int("subscribers", subscribers),
	)

	var wait sync.WaitGroup
	var schedulerErr error

	wait.Add(2)
	go func() {
		defer wait.Done()
		telegramBot.Start(ctx)
	}()
	go func() {
		defer wait.Done()
		schedulerErr = morning.Run(ctx)
	}()

	<-ctx.Done()
	log.Info("shutdown signal received, stopping")
	wait.Wait()

	if schedulerErr != nil && !errors.Is(schedulerErr, context.Canceled) {
		return schedulerErr
	}
	return nil
}

func newLogger(level slog.Level) *slog.Logger {
	options := &slog.HandlerOptions{Level: level}
	if os.Getenv("LOG_FORMAT") == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, options))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, options))
}
