package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
)

func envMap(overrides map[string]string) Getenv {
	base := map[string]string{"TELEGRAM_BOT_TOKEN": "123:ABC"}
	for key, value := range overrides {
		base[key] = value
	}
	return func(key string) string { return base[key] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(envMap(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DefaultPlace.Name != "Дрезден" {
		t.Errorf("город = %q, want Дрезден", cfg.DefaultPlace.Name)
	}
	if cfg.DefaultPlace.Latitude != 51.05 || cfg.DefaultPlace.Longitude != 13.74 {
		t.Errorf("координаты = %v, %v", cfg.DefaultPlace.Latitude, cfg.DefaultPlace.Longitude)
	}
	if cfg.DefaultTimezone.String() != "Europe/Berlin" {
		t.Errorf("таймзона = %q", cfg.DefaultTimezone)
	}
	if cfg.DefaultReportTime.String() != "07:00" {
		t.Errorf("время рассылки = %q", cfg.DefaultReportTime)
	}
	if cfg.DefaultActiveHours.String() != "07-22" {
		t.Errorf("активное окно = %q", cfg.DefaultActiveHours)
	}
	if cfg.DefaultWindUnit != domain.WindUnitMS {
		t.Errorf("единица ветра = %q", cfg.DefaultWindUnit)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("уровень логов = %v", cfg.LogLevel)
	}
	if cfg.DatabasePath != "data/weatherfit.db" {
		t.Errorf("путь к базе = %q", cfg.DatabasePath)
	}
}

func TestLoadOpenModeByDefault(t *testing.T) {
	cfg, err := Load(envMap(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Private() {
		t.Error("без списка chat_id бот должен быть открыт для всех")
	}
	if !cfg.Allows(999) {
		t.Error("в открытом режиме разрешён любой chat_id")
	}
	if cfg.Admin(999) {
		t.Error("без списка админов админов быть не должно")
	}
}

func TestLoadPrivateMode(t *testing.T) {
	cfg, err := Load(envMap(map[string]string{
		"TELEGRAM_ALLOWED_CHAT_IDS": "111, 222 ,111",
		"TELEGRAM_ADMIN_CHAT_IDS":   "111",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Private() {
		t.Error("со списком chat_id бот должен быть приватным")
	}
	if len(cfg.AllowedChatIDs) != 2 {
		t.Errorf("разрешённых chat_id = %v, want two unique values", cfg.AllowedChatIDs)
	}
	if !cfg.Allows(111) || !cfg.Allows(222) || cfg.Allows(333) {
		t.Error("проверка whitelist работает неверно")
	}
	if !cfg.Admin(111) || cfg.Admin(222) {
		t.Error("проверка админов работает неверно")
	}
}

func TestLoadNegativeChatID(t *testing.T) {
	cfg, err := Load(envMap(map[string]string{"TELEGRAM_ALLOWED_CHAT_IDS": "-1001234567890"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Allows(-1001234567890) {
		t.Error("отрицательный chat_id группы должен разбираться")
	}
}

func TestLoadInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		mustSay string
	}{
		{name: "нет токена", env: map[string]string{"TELEGRAM_BOT_TOKEN": ""}, mustSay: "TELEGRAM_BOT_TOKEN"},
		{name: "chat_id не число", env: map[string]string{"TELEGRAM_ALLOWED_CHAT_IDS": "abc"}, mustSay: "TELEGRAM_ALLOWED_CHAT_IDS"},
		{name: "админ не число", env: map[string]string{"TELEGRAM_ADMIN_CHAT_IDS": "1,x"}, mustSay: "TELEGRAM_ADMIN_CHAT_IDS"},
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
				t.Fatal("expected an error, got none")
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
		t.Fatal("expected an error, got none")
	}
	for _, want := range []string{"TELEGRAM_BOT_TOKEN", "WIND_UNIT", "ACTIVE_HOURS"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ошибка не упоминает %q:\n%s", want, err)
		}
	}
}

func TestNewSubscriberUsesDefaults(t *testing.T) {
	cfg, err := Load(envMap(map[string]string{
		"LOCATION_NAME": "Прага",
		"LOCATION_LAT":  "50.08",
		"LOCATION_LON":  "14.44",
		"TZ_NAME":       "Europe/Prague",
		"REPORT_TIME":   "06:30",
		"ACTIVE_HOURS":  "08-20",
		"WIND_UNIT":     "kmh",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	subscriber := cfg.NewSubscriber(555, now)

	if subscriber.ChatID != 555 {
		t.Errorf("chat_id = %d", subscriber.ChatID)
	}
	if subscriber.Place.Name != "Прага" || subscriber.TZName != "Europe/Prague" {
		t.Errorf("город = %q, таймзона = %q", subscriber.Place.Name, subscriber.TZName)
	}
	if subscriber.ReportTime.String() != "06:30" || subscriber.ActiveHours.String() != "08-20" {
		t.Errorf("время = %q, окно = %q", subscriber.ReportTime, subscriber.ActiveHours)
	}
	if subscriber.WindUnit != domain.WindUnitKMH {
		t.Errorf("единица ветра = %q", subscriber.WindUnit)
	}
	if !subscriber.CreatedAt.Equal(now) || !subscriber.UpdatedAt.Equal(now) {
		t.Error("времена создания и обновления должны быть равны now")
	}
	if err := subscriber.Validate(); err != nil {
		t.Errorf("подписчик по умолчанию не проходит валидацию: %v", err)
	}
}
