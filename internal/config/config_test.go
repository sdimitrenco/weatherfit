package config

import (
	"log/slog"
	"strings"
	"testing"
)

func envMap(overrides map[string]string) Getenv {
	base := map[string]string{
		"TELEGRAM_BOT_TOKEN":        "123:ABC",
		"TELEGRAM_ALLOWED_CHAT_IDS": "111",
	}
	for key, value := range overrides {
		base[key] = value
	}
	return func(key string) string { return base[key] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(envMap(nil))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	if cfg.LocationName != "Дрезден" {
		t.Errorf("LocationName = %q, ожидалось Дрезден", cfg.LocationName)
	}
	if cfg.Latitude != 51.05 || cfg.Longitude != 13.74 {
		t.Errorf("координаты = %v, %v, ожидалось 51.05, 13.74", cfg.Latitude, cfg.Longitude)
	}
	if cfg.Location.String() != "Europe/Berlin" {
		t.Errorf("Location = %q, ожидалось Europe/Berlin", cfg.Location)
	}
	if cfg.ReportTime.String() != "07:00" {
		t.Errorf("ReportTime = %q, ожидалось 07:00", cfg.ReportTime)
	}
	if cfg.ActiveHours.String() != "07-22" {
		t.Errorf("ActiveHours = %q, ожидалось 07-22", cfg.ActiveHours)
	}
	if cfg.WindUnit != WindUnitMS {
		t.Errorf("WindUnit = %q, ожидалось ms", cfg.WindUnit)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, ожидалось info", cfg.LogLevel)
	}
	if cfg.StateFile != "data/state.json" {
		t.Errorf("StateFile = %q, ожидалось data/state.json", cfg.StateFile)
	}
}

func TestLoadChatIDs(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		want  []int64
		error bool
	}{
		{name: "один", raw: "111", want: []int64{111}},
		{name: "несколько с пробелами", raw: " 111 , 222 ", want: []int64{111, 222}},
		{name: "отрицательный id группы", raw: "-1001234567890", want: []int64{-1001234567890}},
		{name: "дубликаты сворачиваются", raw: "111,111", want: []int64{111}},
		{name: "висящая запятая", raw: "111,", want: []int64{111}},
		{name: "пусто", raw: "", error: true},
		{name: "только запятые", raw: " , ", error: true},
		{name: "не число", raw: "111,abc", error: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(envMap(map[string]string{"TELEGRAM_ALLOWED_CHAT_IDS": tc.raw}))
			if tc.error {
				if err == nil {
					t.Fatal("ожидалась ошибка, её нет")
				}
				return
			}
			if err != nil {
				t.Fatalf("неожиданная ошибка: %v", err)
			}
			if len(cfg.AllowedChatIDs) != len(tc.want) {
				t.Fatalf("AllowedChatIDs = %v, ожидалось %v", cfg.AllowedChatIDs, tc.want)
			}
			for i, want := range tc.want {
				if cfg.AllowedChatIDs[i] != want {
					t.Errorf("AllowedChatIDs[%d] = %d, ожидалось %d", i, cfg.AllowedChatIDs[i], want)
				}
			}
		})
	}
}

func TestLoadInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		mustSay string
	}{
		{name: "нет токена", env: map[string]string{"TELEGRAM_BOT_TOKEN": ""}, mustSay: "TELEGRAM_BOT_TOKEN"},
		{name: "широта не число", env: map[string]string{"LOCATION_LAT": "север"}, mustSay: "LOCATION_LAT"},
		{name: "широта вне диапазона", env: map[string]string{"LOCATION_LAT": "91"}, mustSay: "LOCATION_LAT"},
		{name: "долгота вне диапазона", env: map[string]string{"LOCATION_LON": "-181"}, mustSay: "LOCATION_LON"},
		{name: "неизвестная таймзона", env: map[string]string{"TZ_NAME": "Europe/Atlantis"}, mustSay: "TZ_NAME"},
		{name: "время без двоеточия", env: map[string]string{"REPORT_TIME": "0700"}, mustSay: "REPORT_TIME"},
		{name: "час вне диапазона", env: map[string]string{"REPORT_TIME": "24:00"}, mustSay: "REPORT_TIME"},
		{name: "минуты вне диапазона", env: map[string]string{"REPORT_TIME": "07:60"}, mustSay: "REPORT_TIME"},
		{name: "окно без дефиса", env: map[string]string{"ACTIVE_HOURS": "0722"}, mustSay: "ACTIVE_HOURS"},
		{name: "окно перевёрнуто", env: map[string]string{"ACTIVE_HOURS": "22-07"}, mustSay: "ACTIVE_HOURS"},
		{name: "окно вне диапазона", env: map[string]string{"ACTIVE_HOURS": "07-24"}, mustSay: "ACTIVE_HOURS"},
		{name: "единица ветра", env: map[string]string{"WIND_UNIT": "mph"}, mustSay: "WIND_UNIT"},
		{name: "уровень логов", env: map[string]string{"LOG_LEVEL": "trace"}, mustSay: "LOG_LEVEL"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(envMap(tc.env))
			if err == nil {
				t.Fatal("ожидалась ошибка, её нет")
			}
			if !strings.Contains(err.Error(), tc.mustSay) {
				t.Errorf("ошибка %q не упоминает %q", err, tc.mustSay)
			}
		})
	}
}

func TestLoadReportsAllProblemsAtOnce(t *testing.T) {
	_, err := Load(envMap(map[string]string{
		"TELEGRAM_BOT_TOKEN": "",
		"WIND_UNIT":          "mph",
		"ACTIVE_HOURS":       "22-07",
	}))
	if err == nil {
		t.Fatal("ожидалась ошибка, её нет")
	}
	for _, want := range []string{"TELEGRAM_BOT_TOKEN", "WIND_UNIT", "ACTIVE_HOURS"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ошибка не упоминает %q:\n%s", want, err)
		}
	}
}

func TestActiveHoursContains(t *testing.T) {
	window := ActiveHours{Start: 7, End: 22}
	tests := []struct {
		hour int
		want bool
	}{
		{hour: 6, want: false},
		{hour: 7, want: true},
		{hour: 15, want: true},
		{hour: 22, want: true},
		{hour: 23, want: false},
	}
	for _, tc := range tests {
		if got := window.Contains(tc.hour); got != tc.want {
			t.Errorf("Contains(%d) = %v, ожидалось %v", tc.hour, got, tc.want)
		}
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(envMap(map[string]string{
		"LOCATION_NAME": "Прага",
		"LOCATION_LAT":  "50.08",
		"LOCATION_LON":  "14.44",
		"TZ_NAME":       "Europe/Prague",
		"REPORT_TIME":   "06:30",
		"ACTIVE_HOURS":  "08-20",
		"WIND_UNIT":     "kmh",
		"LOG_LEVEL":     "debug",
		"STATE_FILE":    "/var/lib/weatherbot/state.json",
	}))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if cfg.LocationName != "Прага" || cfg.TZName != "Europe/Prague" {
		t.Errorf("локация = %q / %q", cfg.LocationName, cfg.TZName)
	}
	if cfg.ReportTime.String() != "06:30" || cfg.ActiveHours.String() != "08-20" {
		t.Errorf("время = %q, окно = %q", cfg.ReportTime, cfg.ActiveHours)
	}
	if cfg.WindUnit != WindUnitKMH || cfg.LogLevel != slog.LevelDebug {
		t.Errorf("ветер = %q, логи = %v", cfg.WindUnit, cfg.LogLevel)
	}
	if cfg.StateFile != "/var/lib/weatherbot/state.json" {
		t.Errorf("StateFile = %q", cfg.StateFile)
	}
}
