package domain

import (
	"fmt"
	"time"
)

// PendingAction — чего бот ждёт от пользователя следующим сообщением.
type PendingAction string

const (
	PendingNone PendingAction = ""
	PendingCity PendingAction = "city"
	PendingTime PendingAction = "time"
)

// Subscriber — получатель рассылки со своими настройками.
type Subscriber struct {
	ChatID      int64
	Place       Location
	TZName      string
	ReportTime  DayTime
	ActiveHours HourWindow
	WindUnit    WindUnit
	Paused      bool
	// LastSentDate — дата последней успешной рассылки в таймзоне подписчика,
	// в формате 2006-01-02. Пустая строка означает, что рассылки ещё не было.
	LastSentDate string
	Pending      PendingAction
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Timezone загружает локацию подписчика, подставляя UTC при неизвестном имени.
func (s Subscriber) Timezone() *time.Location {
	location, err := time.LoadLocation(s.TZName)
	if err != nil {
		return time.UTC
	}
	return location
}

// Validate проверяет настройки подписчика.
func (s Subscriber) Validate() error {
	if s.ChatID == 0 {
		return fmt.Errorf("chat_id не задан")
	}
	if err := s.Place.Validate(); err != nil {
		return err
	}
	if _, err := time.LoadLocation(s.TZName); err != nil {
		return fmt.Errorf("неизвестная таймзона %q", s.TZName)
	}
	if s.ActiveHours.Start > s.ActiveHours.End {
		return fmt.Errorf("активное окно %s перевёрнуто", s.ActiveHours)
	}
	if _, err := ParseWindUnit(string(s.WindUnit)); err != nil {
		return err
	}
	return nil
}

// DueAt сообщает, пора ли отправлять утренний отчёт в момент now.
// Отчёт считается пропущенным и досылается, если время рассылки уже прошло,
// но день ещё не вышел за пределы активного окна.
func (s Subscriber) DueAt(now time.Time) bool {
	if s.Paused {
		return false
	}

	local := now.In(s.Timezone())
	if s.LastSentDate == local.Format(time.DateOnly) {
		return false
	}

	scheduled := s.ReportTime.On(local)
	if local.Before(scheduled) {
		return false
	}

	deadline := time.Date(local.Year(), local.Month(), local.Day(), s.ActiveHours.End, 59, 59, 0, local.Location())
	if scheduled.After(deadline) {
		deadline = scheduled.Add(time.Hour)
	}
	return !local.After(deadline)
}
