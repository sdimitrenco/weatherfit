package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// WindUnit is the unit wind speed is shown in.
type WindUnit string

const (
	WindUnitMS  WindUnit = "ms"
	WindUnitKMH WindUnit = "kmh"
)

// ParseWindUnit reads a wind unit from a string.
func ParseWindUnit(raw string) (WindUnit, error) {
	switch WindUnit(strings.ToLower(strings.TrimSpace(raw))) {
	case WindUnitMS:
		return WindUnitMS, nil
	case WindUnitKMH:
		return WindUnitKMH, nil
	default:
		return "", fmt.Errorf("%q is not supported, want ms or kmh", raw)
	}
}

// Label is kept for logs; user-facing labels come from the i18n layer.
func (u WindUnit) Label() string {
	if u == WindUnitKMH {
		return "km/h"
	}
	return "m/s"
}

// FromMS converts a speed from m/s into this unit.
func (u WindUnit) FromMS(speedMS float64) float64 {
	if u == WindUnitKMH {
		return speedMS * 3.6
	}
	return speedMS
}

// DayTime is a time of day without a date.
type DayTime struct {
	Hour   int
	Minute int
}

// ParseDayTime reads a time in HH:MM form.
func ParseDayTime(raw string) (DayTime, error) {
	hour, minute, found := strings.Cut(strings.TrimSpace(raw), ":")
	if !found {
		return DayTime{}, fmt.Errorf("%q does not look like a time, want HH:MM", raw)
	}
	h, err := parseBounded(hour, 0, 23)
	if err != nil {
		return DayTime{}, fmt.Errorf("hour: %w", err)
	}
	m, err := parseBounded(minute, 0, 59)
	if err != nil {
		return DayTime{}, fmt.Errorf("minute: %w", err)
	}
	return DayTime{Hour: h, Minute: m}, nil
}

// String renders the time as HH:MM.
func (t DayTime) String() string {
	return fmt.Sprintf("%02d:%02d", t.Hour, t.Minute)
}

// On returns this time of day on the given date, in that date location.
func (t DayTime) On(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), t.Hour, t.Minute, 0, 0, date.Location())
}

// HourWindow is the active window of the day, both bounds inclusive.
type HourWindow struct {
	Start int
	End   int
}

// ParseHourWindow reads a window in HH-HH form.
func ParseHourWindow(raw string) (HourWindow, error) {
	start, end, found := strings.Cut(strings.TrimSpace(raw), "-")
	if !found {
		return HourWindow{}, fmt.Errorf("%q does not look like an hour window, want HH-HH", raw)
	}
	from, err := parseBounded(start, 0, 23)
	if err != nil {
		return HourWindow{}, fmt.Errorf("window start: %w", err)
	}
	to, err := parseBounded(end, 0, 23)
	if err != nil {
		return HourWindow{}, fmt.Errorf("window end: %w", err)
	}
	if from > to {
		return HourWindow{}, fmt.Errorf("window start %02d is later than the end %02d", from, to)
	}
	return HourWindow{Start: from, End: to}, nil
}

// Contains reports whether an hour falls inside the window.
func (w HourWindow) Contains(hour int) bool {
	return hour >= w.Start && hour <= w.End
}

// String renders the window as HH-HH.
func (w HourWindow) String() string {
	return fmt.Sprintf("%02d-%02d", w.Start, w.End)
}

func parseBounded(raw string, minValue, maxValue int) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%q does not look like an integer", raw)
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("value %d is outside the range [%d, %d]", value, minValue, maxValue)
	}
	return value, nil
}

// ErrLocationOutOfRange is returned for coordinates outside the valid range.
var ErrLocationOutOfRange = errors.New("coordinates outside the valid range")

// Validate checks latitude and longitude.
func (l Location) Validate() error {
	if l.Latitude < -90 || l.Latitude > 90 || l.Longitude < -180 || l.Longitude > 180 {
		return fmt.Errorf("%w: %v, %v", ErrLocationOutOfRange, l.Latitude, l.Longitude)
	}
	return nil
}
