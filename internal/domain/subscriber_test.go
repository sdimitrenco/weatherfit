package domain

import (
	"strings"
	"testing"
	"time"
)

func testSubscriber() Subscriber {
	return Subscriber{
		ChatID:      42,
		Place:       Location{Name: "Дрезден", Latitude: 51.05, Longitude: 13.74},
		TZName:      "Europe/Berlin",
		ReportTime:  DayTime{Hour: 7},
		ActiveHours: HourWindow{Start: 7, End: 22},
		WindUnit:    WindUnitMS,
	}
}

func atBerlin(t *testing.T, day, hour, minute int) time.Time {
	t.Helper()
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("не удалось загрузить таймзону: %v", err)
	}
	return time.Date(2026, 9, day, hour, minute, 0, 0, location)
}

func TestSubscriberDueAt(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*Subscriber)
		now      time.Time
		expected bool
	}{
		{name: "ровно время рассылки", now: atBerlin(t, 10, 7, 0), expected: true},
		{name: "минутой раньше", now: atBerlin(t, 10, 6, 59), expected: false},
		{name: "через час после, отчёт не отправлен", now: atBerlin(t, 10, 8, 0), expected: true},
		{name: "конец активного окна", now: atBerlin(t, 10, 22, 59), expected: true},
		{name: "после активного окна не досылаем", now: atBerlin(t, 10, 23, 0), expected: false},
		{
			name:     "уже отправлено сегодня",
			mutate:   func(s *Subscriber) { s.LastSentDate = "2026-09-10" },
			now:      atBerlin(t, 10, 9, 0),
			expected: false,
		},
		{
			name:     "отправлено вчера, сегодня пора",
			mutate:   func(s *Subscriber) { s.LastSentDate = "2026-09-09" },
			now:      atBerlin(t, 10, 7, 0),
			expected: true,
		},
		{
			name:     "пауза",
			mutate:   func(s *Subscriber) { s.Paused = true },
			now:      atBerlin(t, 10, 7, 0),
			expected: false,
		},
		{
			name:     "время рассылки позже активного окна",
			mutate:   func(s *Subscriber) { s.ReportTime = DayTime{Hour: 23, Minute: 30} },
			now:      atBerlin(t, 10, 23, 45),
			expected: true,
		},
		{
			name:     "время рассылки позже окна, досылка ещё в пределах часа",
			mutate:   func(s *Subscriber) { s.ReportTime = DayTime{Hour: 23, Minute: 0} },
			now:      atBerlin(t, 10, 23, 59),
			expected: true,
		},
		{
			name:     "рассылка в 20:00, конец окна ещё не прошёл",
			mutate:   func(s *Subscriber) { s.ReportTime = DayTime{Hour: 20} },
			now:      atBerlin(t, 10, 22, 30),
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			subscriber := testSubscriber()
			if tc.mutate != nil {
				tc.mutate(&subscriber)
			}
			if got := subscriber.DueAt(tc.now); got != tc.expected {
				t.Errorf("DueAt(%v) = %v, ожидалось %v", tc.now.Format(time.RFC3339), got, tc.expected)
			}
		})
	}
}

func TestSubscriberDueAtUsesOwnTimezone(t *testing.T) {
	subscriber := testSubscriber()
	subscriber.TZName = "Asia/Tokyo"

	tokyoSeven := atBerlin(t, 10, 0, 0)
	if !subscriber.DueAt(tokyoSeven) {
		t.Error("07:00 в Токио это 00:00 в Берлине, рассылка должна сработать")
	}

	subscriber.LastSentDate = "2026-09-10"
	if subscriber.DueAt(atBerlin(t, 10, 7, 0)) {
		t.Error("дата последней отправки считается в таймзоне подписчика, повтора быть не должно")
	}

	subscriber.LastSentDate = "2026-09-09"
	if !subscriber.DueAt(atBerlin(t, 10, 7, 0)) {
		t.Error("в Токио уже 10 сентября, отчёт за новый день должен уйти")
	}
}

func TestSubscriberDueAtSurvivesDSTSwitch(t *testing.T) {
	subscriber := testSubscriber()
	location := subscriber.Timezone()

	beforeSwitch := time.Date(2026, 10, 25, 7, 0, 0, 0, location)
	if !subscriber.DueAt(beforeSwitch) {
		t.Error("в день перехода на зимнее время рассылка должна сработать в 07:00 местного времени")
	}
	if _, offset := beforeSwitch.Zone(); offset != 3600 {
		t.Errorf("смещение после перехода = %d, ожидалось 3600", offset)
	}
}

func TestSubscriberTimezoneFallsBackToUTC(t *testing.T) {
	subscriber := testSubscriber()
	subscriber.TZName = "Europe/Atlantis"
	if subscriber.Timezone() != time.UTC {
		t.Error("для неизвестной таймзоны ожидается UTC")
	}
}

func TestSubscriberValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Subscriber)
		mustSay string
	}{
		{name: "валидный", mutate: func(*Subscriber) {}},
		{name: "нет chat_id", mutate: func(s *Subscriber) { s.ChatID = 0 }, mustSay: "chat_id"},
		{name: "широта вне диапазона", mutate: func(s *Subscriber) { s.Place.Latitude = 91 }, mustSay: "координаты"},
		{name: "долгота вне диапазона", mutate: func(s *Subscriber) { s.Place.Longitude = -181 }, mustSay: "координаты"},
		{name: "неизвестная таймзона", mutate: func(s *Subscriber) { s.TZName = "Mars/Olympus" }, mustSay: "таймзона"},
		{name: "перевёрнутое окно", mutate: func(s *Subscriber) { s.ActiveHours = HourWindow{Start: 20, End: 8} }, mustSay: "окно"},
		{name: "неизвестная единица", mutate: func(s *Subscriber) { s.WindUnit = "mph" }, mustSay: "mph"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			subscriber := testSubscriber()
			tc.mutate(&subscriber)
			err := subscriber.Validate()
			if tc.mustSay == "" {
				if err != nil {
					t.Fatalf("неожиданная ошибка: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ожидалась ошибка, её нет")
			}
			if !strings.Contains(err.Error(), tc.mustSay) {
				t.Errorf("ошибка %q не упоминает %q", err, tc.mustSay)
			}
		})
	}
}

func TestParseDayTime(t *testing.T) {
	tests := []struct {
		raw   string
		want  string
		error bool
	}{
		{raw: "07:00", want: "07:00"},
		{raw: "7:5", want: "07:05"},
		{raw: " 23:59 ", want: "23:59"},
		{raw: "0:00", want: "00:00"},
		{raw: "24:00", error: true},
		{raw: "07:60", error: true},
		{raw: "0700", error: true},
		{raw: "утром", error: true},
		{raw: "", error: true},
	}

	for _, tc := range tests {
		got, err := ParseDayTime(tc.raw)
		if tc.error {
			if err == nil {
				t.Errorf("%q: ожидалась ошибка", tc.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: неожиданная ошибка %v", tc.raw, err)
			continue
		}
		if got.String() != tc.want {
			t.Errorf("%q разобрано как %q, ожидалось %q", tc.raw, got, tc.want)
		}
	}
}

func TestDayTimeOn(t *testing.T) {
	date := atBerlin(t, 10, 15, 30)
	moment := DayTime{Hour: 7, Minute: 15}.On(date)
	if moment.Format("2006-01-02 15:04 -0700") != "2026-09-10 07:15 +0200" {
		t.Errorf("On = %q", moment.Format("2006-01-02 15:04 -0700"))
	}
}

func TestParseHourWindow(t *testing.T) {
	window, err := ParseHourWindow("07-22")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !window.Contains(7) || !window.Contains(22) || window.Contains(6) || window.Contains(23) {
		t.Error("границы окна работают неверно")
	}
	for _, raw := range []string{"22-07", "07-24", "0722", "", "a-b"} {
		if _, err := ParseHourWindow(raw); err == nil {
			t.Errorf("%q: ожидалась ошибка", raw)
		}
	}
}

func TestWindUnitConversion(t *testing.T) {
	if got := WindUnitMS.FromMS(9); got != 9 {
		t.Errorf("м/с = %v", got)
	}
	if got := WindUnitKMH.FromMS(10); got != 36 {
		t.Errorf("км/ч = %v, ожидалось 36", got)
	}
	if WindUnitMS.Label() != "м/с" || WindUnitKMH.Label() != "км/ч" {
		t.Error("подписи единиц неверны")
	}
	for _, raw := range []string{"ms", "MS", " kmh ", "KMH"} {
		if _, err := ParseWindUnit(raw); err != nil {
			t.Errorf("%q: неожиданная ошибка %v", raw, err)
		}
	}
	if _, err := ParseWindUnit("mph"); err == nil {
		t.Error("ожидалась ошибка для mph")
	}
}
