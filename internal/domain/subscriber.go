package domain

import (
	"fmt"
	"time"
)

// PendingAction is what the bot expects in the user's next message.
type PendingAction string

const (
	PendingNone  PendingAction = ""
	PendingCity  PendingAction = "city"
	PendingTime  PendingAction = "time"
	PendingHours PendingAction = "hours"
)

// Subscriber is a recipient with personal settings.
type Subscriber struct {
	ChatID      int64
	Lang        string
	Place       Location
	TZName      string
	ReportTime  DayTime
	ActiveHours HourWindow
	WindUnit    WindUnit
	Paused      bool
	// LastSentDate is the date of the last delivered report in the subscriber
	// timezone, formatted 2006-01-02. Empty means nothing was sent yet.
	LastSentDate string
	Pending      PendingAction
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Timezone loads the subscriber location, falling back to UTC.
func (s Subscriber) Timezone() *time.Location {
	location, err := time.LoadLocation(s.TZName)
	if err != nil {
		return time.UTC
	}
	return location
}

// Validate checks the subscriber settings.
func (s Subscriber) Validate() error {
	if s.ChatID == 0 {
		return fmt.Errorf("chat_id is not set")
	}
	if err := s.Place.Validate(); err != nil {
		return err
	}
	if _, err := time.LoadLocation(s.TZName); err != nil {
		return fmt.Errorf("unknown timezone %q", s.TZName)
	}
	if s.ActiveHours.Start > s.ActiveHours.End {
		return fmt.Errorf("active window %s is inverted", s.ActiveHours)
	}
	if _, err := ParseWindUnit(string(s.WindUnit)); err != nil {
		return err
	}
	return nil
}

// DueAt reports whether the morning report should go out at now. A missed
// report is still delivered while the day has not passed the active window.
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
