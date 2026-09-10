// Package config загружает и валидирует конфигурацию из переменных окружения.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// WindUnit — единица измерения скорости ветра.
type WindUnit string

const (
	WindUnitMS  WindUnit = "ms"
	WindUnitKMH WindUnit = "kmh"
)

const chatIDsSeparator = ","

// Getenv читает значение переменной окружения по имени.
type Getenv func(key string) string

// ActiveHours — активное окно суток, включительно с обеих сторон.
type ActiveHours struct {
	Start int
	End   int
}

// Contains сообщает, попадает ли час суток в активное окно.
func (a ActiveHours) Contains(hour int) bool {
	return hour >= a.Start && hour <= a.End
}

// String возвращает окно в формате конфига, например "07-22".
func (a ActiveHours) String() string {
	return fmt.Sprintf("%02d-%02d", a.Start, a.End)
}

// ReportTime — время утренней рассылки в локальной таймзоне.
type ReportTime struct {
	Hour   int
	Minute int
}

// String возвращает время в формате конфига, например "07:00".
func (r ReportTime) String() string {
	return fmt.Sprintf("%02d:%02d", r.Hour, r.Minute)
}

// Config — полная конфигурация приложения.
type Config struct {
	TelegramBotToken string
	AllowedChatIDs   []int64
	LocationName     string
	Latitude         float64
	Longitude        float64
	TZName           string
	Location         *time.Location
	ReportTime       ReportTime
	ActiveHours      ActiveHours
	WindUnit         WindUnit
	LogLevel         slog.Level
	StateFile        string
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
	defaultStateFile    = "data/state.json"
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
		LocationName:     valueOr(getenv("LOCATION_NAME"), defaultLocationName),
		TZName:           valueOr(getenv("TZ_NAME"), defaultTZName),
		StateFile:        valueOr(getenv("STATE_FILE"), defaultStateFile),
	}

	if cfg.TelegramBotToken == "" {
		fail("TELEGRAM_BOT_TOKEN: обязательная переменная не задана")
	}

	chatIDs, err := parseChatIDs(valueOr(getenv("TELEGRAM_ALLOWED_CHAT_IDS"), ""))
	if err != nil {
		fail("TELEGRAM_ALLOWED_CHAT_IDS: %w", err)
	}
	cfg.AllowedChatIDs = chatIDs

	cfg.Latitude, err = parseCoordinate(valueOr(getenv("LOCATION_LAT"), ""), defaultLatitude, 90)
	if err != nil {
		fail("LOCATION_LAT: %w", err)
	}

	cfg.Longitude, err = parseCoordinate(valueOr(getenv("LOCATION_LON"), ""), defaultLongitude, 180)
	if err != nil {
		fail("LOCATION_LON: %w", err)
	}

	cfg.Location, err = time.LoadLocation(cfg.TZName)
	if err != nil {
		fail("TZ_NAME: неизвестная таймзона %q", cfg.TZName)
	}

	cfg.ReportTime, err = parseReportTime(valueOr(getenv("REPORT_TIME"), defaultReportTime))
	if err != nil {
		fail("REPORT_TIME: %w", err)
	}

	cfg.ActiveHours, err = parseActiveHours(valueOr(getenv("ACTIVE_HOURS"), defaultActiveHours))
	if err != nil {
		fail("ACTIVE_HOURS: %w", err)
	}

	cfg.WindUnit, err = parseWindUnit(valueOr(getenv("WIND_UNIT"), defaultWindUnit))
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

func valueOr(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func parseChatIDs(raw string) ([]int64, error) {
	if raw == "" {
		return nil, errors.New("обязательная переменная не задана, укажи свой chat_id")
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
	if len(ids) == 0 {
		return nil, errors.New("список пуст, укажи хотя бы один chat_id")
	}
	return ids, nil
}

func parseCoordinate(raw string, fallback, limit float64) (float64, error) {
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

func parseReportTime(raw string) (ReportTime, error) {
	hour, minute, found := strings.Cut(raw, ":")
	if !found {
		return ReportTime{}, fmt.Errorf("%q не похоже на время, ожидается формат ЧЧ:ММ", raw)
	}
	h, err := parseBoundedInt(hour, 0, 23)
	if err != nil {
		return ReportTime{}, fmt.Errorf("час: %w", err)
	}
	m, err := parseBoundedInt(minute, 0, 59)
	if err != nil {
		return ReportTime{}, fmt.Errorf("минуты: %w", err)
	}
	return ReportTime{Hour: h, Minute: m}, nil
}

func parseActiveHours(raw string) (ActiveHours, error) {
	start, end, found := strings.Cut(raw, "-")
	if !found {
		return ActiveHours{}, fmt.Errorf("%q не похоже на окно часов, ожидается формат ЧЧ-ЧЧ", raw)
	}
	from, err := parseBoundedInt(start, 0, 23)
	if err != nil {
		return ActiveHours{}, fmt.Errorf("начало окна: %w", err)
	}
	to, err := parseBoundedInt(end, 0, 23)
	if err != nil {
		return ActiveHours{}, fmt.Errorf("конец окна: %w", err)
	}
	if from > to {
		return ActiveHours{}, fmt.Errorf("начало окна %02d позже конца %02d", from, to)
	}
	return ActiveHours{Start: from, End: to}, nil
}

func parseBoundedInt(raw string, minValue, maxValue int) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%q не похоже на целое число", raw)
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("значение %d вне диапазона [%d, %d]", value, minValue, maxValue)
	}
	return value, nil
}

func parseWindUnit(raw string) (WindUnit, error) {
	switch WindUnit(strings.ToLower(raw)) {
	case WindUnitMS:
		return WindUnitMS, nil
	case WindUnitKMH:
		return WindUnitKMH, nil
	default:
		return "", fmt.Errorf("%q не поддерживается, ожидается ms или kmh", raw)
	}
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
