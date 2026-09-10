package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata"

	"github.com/sdimitrenco/weatherfit/internal/config"
)

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "не удалось прочитать конфигурацию:\n%v\n", err)
		os.Exit(1)
	}

	log := newLogger(cfg.LogLevel)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("бот запущен",
		slog.String("location", cfg.LocationName),
		slog.Float64("lat", cfg.Latitude),
		slog.Float64("lon", cfg.Longitude),
		slog.String("tz", cfg.TZName),
		slog.String("report_time", cfg.ReportTime.String()),
		slog.String("active_hours", cfg.ActiveHours.String()),
		slog.String("wind_unit", string(cfg.WindUnit)),
		slog.Int("recipients", len(cfg.AllowedChatIDs)),
	)

	<-ctx.Done()
	log.Info("получен сигнал остановки, завершаю работу")
}

func newLogger(level slog.Level) *slog.Logger {
	options := &slog.HandlerOptions{Level: level}
	if os.Getenv("LOG_FORMAT") == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, options))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, options))
}
