// Package sqlite хранит подписчиков в файле SQLite.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/port"
)

const schema = `
CREATE TABLE IF NOT EXISTS subscribers (
	chat_id      INTEGER PRIMARY KEY,
	place_name   TEXT    NOT NULL,
	latitude     REAL    NOT NULL,
	longitude    REAL    NOT NULL,
	tz_name      TEXT    NOT NULL,
	report_time  TEXT    NOT NULL,
	active_hours TEXT    NOT NULL,
	wind_unit    TEXT    NOT NULL,
	paused       INTEGER NOT NULL DEFAULT 0,
	last_sent    TEXT    NOT NULL DEFAULT '',
	pending      TEXT    NOT NULL DEFAULT '',
	created_at   TEXT    NOT NULL,
	updated_at   TEXT    NOT NULL
);
`

// Store — реализация port.SubscriberStore на SQLite без cgo.
type Store struct {
	db *sql.DB
}

// Open открывает базу по пути, создавая каталог и схему при необходимости.
func Open(ctx context.Context, path string) (*Store, error) {
	if directory := filepath.Dir(path); directory != "" && directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return nil, fmt.Errorf("sqlite: не удалось создать каталог %q: %w", directory, err)
		}
	}

	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: не удалось открыть базу: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: база недоступна: %w", err)
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: не удалось создать схему: %w", err)
	}

	return &Store{db: db}, nil
}

// Close закрывает базу.
func (s *Store) Close() error {
	return s.db.Close()
}

// Save создаёт или обновляет подписчика целиком.
func (s *Store) Save(ctx context.Context, subscriber domain.Subscriber) error {
	if err := subscriber.Validate(); err != nil {
		return fmt.Errorf("sqlite: подписчик невалиден: %w", err)
	}

	const query = `
INSERT INTO subscribers (
	chat_id, place_name, latitude, longitude, tz_name, report_time, active_hours,
	wind_unit, paused, last_sent, pending, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(chat_id) DO UPDATE SET
	place_name = excluded.place_name,
	latitude = excluded.latitude,
	longitude = excluded.longitude,
	tz_name = excluded.tz_name,
	report_time = excluded.report_time,
	active_hours = excluded.active_hours,
	wind_unit = excluded.wind_unit,
	paused = excluded.paused,
	last_sent = excluded.last_sent,
	pending = excluded.pending,
	updated_at = excluded.updated_at`

	_, err := s.db.ExecContext(ctx, query,
		subscriber.ChatID,
		subscriber.Place.Name,
		subscriber.Place.Latitude,
		subscriber.Place.Longitude,
		subscriber.TZName,
		subscriber.ReportTime.String(),
		subscriber.ActiveHours.String(),
		string(subscriber.WindUnit),
		boolToInt(subscriber.Paused),
		subscriber.LastSentDate,
		string(subscriber.Pending),
		formatTime(subscriber.CreatedAt),
		formatTime(subscriber.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("sqlite: не удалось сохранить подписчика %d: %w", subscriber.ChatID, err)
	}
	return nil
}

// Get возвращает подписчика или port.ErrSubscriberNotFound.
func (s *Store) Get(ctx context.Context, chatID int64) (domain.Subscriber, error) {
	const query = `SELECT ` + columns + ` FROM subscribers WHERE chat_id = ?`

	row := s.db.QueryRowContext(ctx, query, chatID)
	subscriber, err := scanSubscriber(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Subscriber{}, fmt.Errorf("sqlite: %w: chat_id %d", port.ErrSubscriberNotFound, chatID)
	}
	if err != nil {
		return domain.Subscriber{}, fmt.Errorf("sqlite: не удалось прочитать подписчика %d: %w", chatID, err)
	}
	return subscriber, nil
}

// All возвращает всех подписчиков в порядке добавления.
func (s *Store) All(ctx context.Context) ([]domain.Subscriber, error) {
	const query = `SELECT ` + columns + ` FROM subscribers ORDER BY chat_id`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("sqlite: не удалось прочитать подписчиков: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var subscribers []domain.Subscriber
	for rows.Next() {
		subscriber, err := scanSubscriber(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: не удалось разобрать строку: %w", err)
		}
		subscribers = append(subscribers, subscriber)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: обход подписчиков прерван: %w", err)
	}
	return subscribers, nil
}

// Delete удаляет подписчика.
func (s *Store) Delete(ctx context.Context, chatID int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM subscribers WHERE chat_id = ?`, chatID); err != nil {
		return fmt.Errorf("sqlite: не удалось удалить подписчика %d: %w", chatID, err)
	}
	return nil
}

// MarkSent запоминает дату последней успешной рассылки.
func (s *Store) MarkSent(ctx context.Context, chatID int64, date string) error {
	const query = `UPDATE subscribers SET last_sent = ?, updated_at = ? WHERE chat_id = ?`
	if _, err := s.db.ExecContext(ctx, query, date, formatTime(time.Now().UTC()), chatID); err != nil {
		return fmt.Errorf("sqlite: не удалось отметить рассылку для %d: %w", chatID, err)
	}
	return nil
}

// SetPending запоминает, какого ввода бот ждёт от пользователя.
func (s *Store) SetPending(ctx context.Context, chatID int64, pending domain.PendingAction) error {
	const query = `UPDATE subscribers SET pending = ?, updated_at = ? WHERE chat_id = ?`
	if _, err := s.db.ExecContext(ctx, query, string(pending), formatTime(time.Now().UTC()), chatID); err != nil {
		return fmt.Errorf("sqlite: не удалось сохранить ожидаемый ввод для %d: %w", chatID, err)
	}
	return nil
}

// Count возвращает число подписчиков.
func (s *Store) Count(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscribers`).Scan(&count); err != nil {
		return 0, fmt.Errorf("sqlite: не удалось посчитать подписчиков: %w", err)
	}
	return count, nil
}

const columns = `chat_id, place_name, latitude, longitude, tz_name, report_time,
	active_hours, wind_unit, paused, last_sent, pending, created_at, updated_at`

type scanner interface {
	Scan(dest ...any) error
}

func scanSubscriber(row scanner) (domain.Subscriber, error) {
	var (
		subscriber  domain.Subscriber
		reportTime  string
		activeHours string
		windUnit    string
		paused      int
		pending     string
		createdAt   string
		updatedAt   string
	)

	err := row.Scan(
		&subscriber.ChatID,
		&subscriber.Place.Name,
		&subscriber.Place.Latitude,
		&subscriber.Place.Longitude,
		&subscriber.TZName,
		&reportTime,
		&activeHours,
		&windUnit,
		&paused,
		&subscriber.LastSentDate,
		&pending,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return domain.Subscriber{}, err
	}

	if subscriber.ReportTime, err = domain.ParseDayTime(reportTime); err != nil {
		return domain.Subscriber{}, fmt.Errorf("время рассылки %q: %w", reportTime, err)
	}
	if subscriber.ActiveHours, err = domain.ParseHourWindow(activeHours); err != nil {
		return domain.Subscriber{}, fmt.Errorf("активное окно %q: %w", activeHours, err)
	}
	if subscriber.WindUnit, err = domain.ParseWindUnit(windUnit); err != nil {
		return domain.Subscriber{}, fmt.Errorf("единица ветра %q: %w", windUnit, err)
	}
	subscriber.Paused = paused != 0
	subscriber.Pending = domain.PendingAction(pending)
	subscriber.CreatedAt = parseTime(createdAt)
	subscriber.UpdatedAt = parseTime(updatedAt)
	return subscriber, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func formatTime(moment time.Time) string {
	return moment.UTC().Format(time.RFC3339)
}

func parseTime(raw string) time.Time {
	moment, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return moment
}
