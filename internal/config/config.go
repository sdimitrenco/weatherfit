// Package config загружает и валидирует конфигурацию из переменных окружения.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
)

const chatIDsSeparator = ","

// Getenv читает значение переменной окружения по имени.
type Getenv func(key string) string

// Config — полная конфигурация приложения. Локация, время рассылки и активное
// окно задают значения по умолчанию для новых подписчиков; дальше каждый
// подписчик меняет их сам через бота.
type Config struct {
	TelegramBotToken string
	// AllowedChatIDs пуст в открытом режиме. Если список задан, бот отвечает
	// только этим chat_id.
	AllowedChatIDs []int64
	AdminChatIDs   []int64

	DefaultPlace       domain.Location
	DefaultTZName      string
	DefaultTimezone    *time.Location
	DefaultReportTime  domain.DayTime
	DefaultActiveHours domain.HourWindow
	DefaultWindUnit    domain.WindUnit

	DatabasePath string
	LogLevel     slog.Level
}

const (
	defaultLocationName = "Дрезден"
	defaultLatitude     = 51.05
	defaultLongitude    = 13.74
	defaultTZName       = "Europe/Berlin"
	defaultReportTime   = "07:00"
	defaultActiveHours  = "07-22"
	defaultWindUnit     = "ms"
	defaultLogLevel     = "info"
	defaultDatabasePath = "data/weatherfit.db"
)

// Load собирает конфигурацию, подставляя значения по умолчанию, и возвращает
// все найденные ошибки валидации сразу.
func Load(getenv Getenv) (*Config, error) {
	var problems []error
	fail := func(format string, args ...any) {
		problems = append(problems, fmt.Errorf(format, args...))
	}

	cfg := &Config{
		TelegramBotToken: strings.TrimSpace(getenv("TELEGRAM_BOT_TOKEN")),
		DefaultTZName:    valueOr(getenv("TZ_NAME"), defaultTZName),
		DatabasePath:     valueOr(getenv("DB_PATH"), defaultDatabasePath),
	}

	if cfg.TelegramBotToken == "" {
		fail("TELEGRAM_BOT_TOKEN: обязательная переменная не задана")
	}

	var err error
	cfg.AllowedChatIDs, err = parseChatIDs(getenv("TELEGRAM_ALLOWED_CHAT_IDS"))
	if err != nil {
		fail("TELEGRAM_ALLOWED_CHAT_IDS: %w", err)
	}

	cfg.AdminChatIDs, err = parseChatIDs(getenv("TELEGRAM_ADMIN_CHAT_IDS"))
	if err != nil {
		fail("TELEGRAM_ADMIN_CHAT_IDS: %w", err)
	}

	cfg.DefaultPlace.Name = valueOr(getenv("LOCATION_NAME"), defaultLocationName)
	cfg.DefaultPlace.Latitude, err = parseCoordinate(getenv("LOCATION_LAT"), defaultLatitude, 90)
	if err != nil {
		fail("LOCATION_LAT: %w", err)
	}
	cfg.DefaultPlace.Longitude, err = parseCoordinate(getenv("LOCATION_LON"), defaultLongitude, 180)
	if err != nil {
		fail("LOCATION_LON: %w", err)
	}

	cfg.DefaultTimezone, err = time.LoadLocation(cfg.DefaultTZName)
	if err != nil {
		fail("TZ_NAME: неизвестная таймзона %q", cfg.DefaultTZName)
	}

	cfg.DefaultReportTime, err = domain.ParseDayTime(valueOr(getenv("REPORT_TIME"), defaultReportTime))
	if err != nil {
		fail("REPORT_TIME: %w", err)
	}

	cfg.DefaultActiveHours, err = domain.ParseHourWindow(valueOr(getenv("ACTIVE_HOURS"), defaultActiveHours))
	if err != nil {
		fail("ACTIVE_HOURS: %w", err)
	}

	cfg.DefaultWindUnit, err = domain.ParseWindUnit(valueOr(getenv("WIND_UNIT"), defaultWindUnit))
	if err != nil {
		fail("WIND_UNIT: %w", err)
	}

	cfg.LogLevel, err = parseLogLevel(valueOr(getenv("LOG_LEVEL"), defaultLogLevel))
	if err != nil {
		fail("LOG_LEVEL: %w", err)
	}

	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return cfg, nil
}

// Private сообщает, что бот работает по списку разрешённых chat_id.
func (c *Config) Private() bool {
	return len(c.AllowedChatIDs) > 0
}

// Allows сообщает, разрешено ли этому chat_id пользоваться ботом.
func (c *Config) Allows(chatID int64) bool {
	if !c.Private() {
		return true
	}
	return contains(c.AllowedChatIDs, chatID)
}

// Admin сообщает, что chat_id указан в списке администраторов.
func (c *Config) Admin(chatID int64) bool {
	return contains(c.AdminChatIDs, chatID)
}

// NewSubscriber создаёт подписчика с настройками по умолчанию.
func (c *Config) NewSubscriber(chatID int64, now time.Time) domain.Subscriber {
	return domain.Subscriber{
		ChatID:      chatID,
		Place:       c.DefaultPlace,
		TZName:      c.DefaultTZName,
		ReportTime:  c.DefaultReportTime,
		ActiveHours: c.DefaultActiveHours,
		WindUnit:    c.DefaultWindUnit,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func contains(ids []int64, id int64) bool {
	for _, current := range ids {
		if current == id {
			return true
		}
	}
	return false
}

func valueOr(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func parseChatIDs(raw string) ([]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, chatIDsSeparator)
	ids := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q не похоже на chat_id, ожидается целое число", part)
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func parseCoordinate(raw string, fallback, limit float64) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%q не похоже на число", raw)
	}
	if value < -limit || value > limit {
		return 0, fmt.Errorf("значение %v вне диапазона [-%v, %v]", value, limit, limit)
	}
	return value, nil
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(raw) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("%q не поддерживается, ожидается debug, info, warn или error", raw)
	}
}
