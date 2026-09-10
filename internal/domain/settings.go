package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// WindUnit — единица, в которой пользователю показывают скорость ветра.
type WindUnit string

const (
	WindUnitMS  WindUnit = "ms"
	WindUnitKMH WindUnit = "kmh"
)

// ParseWindUnit разбирает единицу ветра из строки.
func ParseWindUnit(raw string) (WindUnit, error) {
	switch WindUnit(strings.ToLower(strings.TrimSpace(raw))) {
	case WindUnitMS:
		return WindUnitMS, nil
	case WindUnitKMH:
		return WindUnitKMH, nil
	default:
		return "", fmt.Errorf("%q не поддерживается, ожидается ms или kmh", raw)
	}
}

// Label возвращает подпись единицы для сообщения.
func (u WindUnit) Label() string {
	if u == WindUnitKMH {
		return "км/ч"
	}
	return "м/с"
}

// FromMS переводит скорость из м/с в выбранную единицу.
func (u WindUnit) FromMS(speedMS float64) float64 {
	if u == WindUnitKMH {
		return speedMS * 3.6
	}
	return speedMS
}

// DayTime — время суток без даты.
type DayTime struct {
	Hour   int
	Minute int
}

// ParseDayTime разбирает время в формате ЧЧ:ММ.
func ParseDayTime(raw string) (DayTime, error) {
	hour, minute, found := strings.Cut(strings.TrimSpace(raw), ":")
	if !found {
		return DayTime{}, fmt.Errorf("%q не похоже на время, ожидается формат ЧЧ:ММ", raw)
	}
	h, err := parseBounded(hour, 0, 23)
	if err != nil {
		return DayTime{}, fmt.Errorf("час: %w", err)
	}
	m, err := parseBounded(minute, 0, 59)
	if err != nil {
		return DayTime{}, fmt.Errorf("минуты: %w", err)
	}
	return DayTime{Hour: h, Minute: m}, nil
}

// String возвращает время как ЧЧ:ММ.
func (t DayTime) String() string {
	return fmt.Sprintf("%02d:%02d", t.Hour, t.Minute)
}

// On возвращает это время суток на заданной дате в её локации.
func (t DayTime) On(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), t.Hour, t.Minute, 0, 0, date.Location())
}

// HourWindow — активное окно часов суток, границы включительно.
type HourWindow struct {
	Start int
	End   int
}

// ParseHourWindow разбирает окно в формате ЧЧ-ЧЧ.
func ParseHourWindow(raw string) (HourWindow, error) {
	start, end, found := strings.Cut(strings.TrimSpace(raw), "-")
	if !found {
		return HourWindow{}, fmt.Errorf("%q не похоже на окно часов, ожидается формат ЧЧ-ЧЧ", raw)
	}
	from, err := parseBounded(start, 0, 23)
	if err != nil {
		return HourWindow{}, fmt.Errorf("начало окна: %w", err)
	}
	to, err := parseBounded(end, 0, 23)
	if err != nil {
		return HourWindow{}, fmt.Errorf("конец окна: %w", err)
	}
	if from > to {
		return HourWindow{}, fmt.Errorf("начало окна %02d позже конца %02d", from, to)
	}
	return HourWindow{Start: from, End: to}, nil
}

// Contains сообщает, попадает ли час суток в окно.
func (w HourWindow) Contains(hour int) bool {
	return hour >= w.Start && hour <= w.End
}

// String возвращает окно как ЧЧ-ЧЧ.
func (w HourWindow) String() string {
	return fmt.Sprintf("%02d-%02d", w.Start, w.End)
}

func parseBounded(raw string, minValue, maxValue int) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%q не похоже на целое число", raw)
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("значение %d вне диапазона [%d, %d]", value, minValue, maxValue)
	}
	return value, nil
}

// ErrLocationOutOfRange возвращается для координат вне допустимых пределов.
var ErrLocationOutOfRange = errors.New("координаты вне допустимого диапазона")

// Validate проверяет широту и долготу.
func (l Location) Validate() error {
	if l.Latitude < -90 || l.Latitude > 90 || l.Longitude < -180 || l.Longitude > 180 {
		return fmt.Errorf("%w: %v, %v", ErrLocationOutOfRange, l.Latitude, l.Longitude)
	}
	return nil
}
