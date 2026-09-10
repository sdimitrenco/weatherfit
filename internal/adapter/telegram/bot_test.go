package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/sdimitrenco/weatherfit/internal/adapter/geocode"
	"github.com/sdimitrenco/weatherfit/internal/adapter/render"
	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/i18n"
	"github.com/sdimitrenco/weatherfit/internal/port"
	"github.com/sdimitrenco/weatherfit/internal/usecase"
)

const testToken = "111:test"

type sentMessage struct {
	ChatID      int64  `json:"chat_id"`
	Text        string `json:"text"`
	ParseMode   string `json:"parse_mode"`
	ReplyMarkup struct {
		Keyboard       [][]models.KeyboardButton       `json:"keyboard"`
		InlineKeyboard [][]models.InlineKeyboardButton `json:"inline_keyboard"`
	} `json:"reply_markup"`
}

type apiStub struct {
	mu       sync.Mutex
	messages []sentMessage
	sendErr  string
	server   *httptest.Server
}

func newAPIStub(t *testing.T) *apiStub {
	t.Helper()
	stub := &apiStub{}

	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]

		switch method {
		case "getMe":
			writeJSON(w, `{"ok":true,"result":{"id":111,"is_bot":true,"username":"weatherfit_bot"}}`)
		case "sendMessage":
			stub.mu.Lock()
			failure := stub.sendErr
			stub.mu.Unlock()

			if failure != "" {
				w.WriteHeader(http.StatusForbidden)
				writeJSON(w, fmt.Sprintf(`{"ok":false,"error_code":403,"description":%q}`, failure))
				return
			}

			message, err := parseSendMessage(r)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				writeJSON(w, fmt.Sprintf(`{"ok":false,"description":%q}`, err.Error()))
				return
			}

			stub.mu.Lock()
			stub.messages = append(stub.messages, message)
			stub.mu.Unlock()
			writeJSON(w, `{"ok":true,"result":{"message_id":1,"chat":{"id":42},"date":0}}`)
		default:
			writeJSON(w, `{"ok":true,"result":true}`)
		}
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

// parseSendMessage reads the multipart form the Telegram client sends.
func parseSendMessage(r *http.Request) (sentMessage, error) {
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		return sentMessage{}, err
	}

	message := sentMessage{
		Text:      r.FormValue("text"),
		ParseMode: r.FormValue("parse_mode"),
	}

	chatID, err := strconv.ParseInt(r.FormValue("chat_id"), 10, 64)
	if err != nil {
		return sentMessage{}, err
	}
	message.ChatID = chatID

	if markup := r.FormValue("reply_markup"); markup != "" {
		if err := json.Unmarshal([]byte(markup), &message.ReplyMarkup); err != nil {
			return sentMessage{}, err
		}
	}
	return message, nil
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

func (a *apiStub) sent() []sentMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]sentMessage(nil), a.messages...)
}

func (a *apiStub) last(t *testing.T) sentMessage {
	t.Helper()
	messages := a.sent()
	if len(messages) == 0 {
		t.Fatal("бот не отправил ни одного сообщения")
	}
	return messages[len(messages)-1]
}

func (a *apiStub) reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = nil
}

type memoryStore struct {
	mu          sync.Mutex
	subscribers map[int64]domain.Subscriber
}

func newMemoryStore(subscribers ...domain.Subscriber) *memoryStore {
	store := &memoryStore{subscribers: map[int64]domain.Subscriber{}}
	for _, subscriber := range subscribers {
		store.subscribers[subscriber.ChatID] = subscriber
	}
	return store
}

func (m *memoryStore) Save(_ context.Context, subscriber domain.Subscriber) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribers[subscriber.ChatID] = subscriber
	return nil
}

func (m *memoryStore) Get(_ context.Context, chatID int64) (domain.Subscriber, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	subscriber, ok := m.subscribers[chatID]
	if !ok {
		return domain.Subscriber{}, fmt.Errorf("memory: %w", port.ErrSubscriberNotFound)
	}
	return subscriber, nil
}

func (m *memoryStore) All(_ context.Context) ([]domain.Subscriber, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	all := make([]domain.Subscriber, 0, len(m.subscribers))
	for _, subscriber := range m.subscribers {
		all = append(all, subscriber)
	}
	return all, nil
}

func (m *memoryStore) Delete(_ context.Context, chatID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.subscribers, chatID)
	return nil
}

func (m *memoryStore) MarkSent(_ context.Context, chatID int64, date string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	subscriber := m.subscribers[chatID]
	subscriber.LastSentDate = date
	m.subscribers[chatID] = subscriber
	return nil
}

func (m *memoryStore) SetPending(_ context.Context, chatID int64, pending domain.PendingAction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	subscriber := m.subscribers[chatID]
	subscriber.Pending = pending
	m.subscribers[chatID] = subscriber
	return nil
}

func (m *memoryStore) Count(_ context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.subscribers), nil
}

type stubForecasts struct {
	forecast domain.Forecast
	err      error
}

func (s *stubForecasts) Forecast(_ context.Context, _ port.ForecastRequest) (domain.Forecast, error) {
	if s.err != nil {
		return domain.Forecast{}, s.err
	}
	return s.forecast, nil
}

type stubGeocoder struct {
	places   []port.Place
	err      error
	query    string
	language string
}

func (s *stubGeocoder) Search(_ context.Context, query string, _ int, language string) ([]port.Place, error) {
	s.query = query
	s.language = language
	return s.places, s.err
}

type stubTimezones struct {
	name string
	err  error
}

func (s *stubTimezones) ResolveTimezone(_ context.Context, _ domain.Location) (string, error) {
	return s.name, s.err
}

type openAccess struct{ admins map[int64]bool }

func (o openAccess) Allows(int64) bool   { return true }
func (o openAccess) Admin(id int64) bool { return o.admins[id] }

type privateAccess struct{ allowed int64 }

func (p privateAccess) Allows(id int64) bool { return id == p.allowed }
func (p privateAccess) Admin(int64) bool     { return false }

type harness struct {
	bot       *Bot
	api       *apiStub
	store     *memoryStore
	geocoder  *stubGeocoder
	timezones *stubTimezones
}

func newHarness(t *testing.T, access Access, store *memoryStore) *harness {
	t.Helper()

	api := newAPIStub(t)
	geocoder := &stubGeocoder{}
	timezones := &stubTimezones{name: "Europe/Berlin"}
	forecasts := &stubForecasts{forecast: sampleForecast(t)}
	clock := port.ClockFunc(func() time.Time {
		return time.Date(2026, 9, 10, 8, 0, 0, 0, berlin(t))
	})

	subscriptions := usecase.NewSubscriptions(store, geocoder, timezones, clock, usecase.Defaults{
		Place:       domain.Location{Name: "Дрезден", Latitude: 51.05, Longitude: 13.74},
		TZName:      "Europe/Berlin",
		ReportTime:  domain.DayTime{Hour: 7},
		ActiveHours: domain.HourWindow{Start: 7, End: 22},
		WindUnit:    domain.WindUnitMS,
		Lang:        i18n.English,
	})

	served, err := New(Options{
		Token:         testToken,
		Access:        access,
		Subscriptions: subscriptions,
		Reports:       usecase.NewBuildReport(forecasts, clock, quietLogger()),
		Renderer:      render.NewRenderer(render.LayoutLines),
		Log:           quietLogger(),
		BotOptions: []bot.Option{
			bot.WithServerURL(api.server.URL),
			bot.WithSkipGetMe(),
			bot.WithNotAsyncHandlers(),
		},
	})
	if err != nil {
		t.Fatalf("не удалось создать бота: %v", err)
	}

	return &harness{bot: served, api: api, store: store, geocoder: geocoder, timezones: timezones}
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func berlin(t *testing.T) *time.Location {
	t.Helper()
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("не удалось загрузить таймзону: %v", err)
	}
	return location
}

func sampleForecast(t *testing.T) domain.Forecast {
	t.Helper()
	location := berlin(t)

	hours := make([]domain.HourPoint, 0, 48)
	for day := 10; day <= 11; day++ {
		for hour := 0; hour < 24; hour++ {
			hours = append(hours, domain.HourPoint{
				Time:                     time.Date(2026, 9, day, hour, 0, 0, 0, location),
				TemperatureC:             domain.Some(15.0),
				ApparentTemperatureC:     domain.Some(14.0),
				PrecipitationProbability: domain.Some(0),
				PrecipitationMM:          domain.Some(0.0),
				WeatherCode:              domain.Some(3),
				WindSpeedMS:              domain.Some(2.0),
				WindDirectionDeg:         domain.Some(270),
				WindGustsMS:              domain.Some(4.0),
				IsDay:                    domain.Some(true),
			})
		}
	}

	return domain.Forecast{
		Location: domain.Location{Name: "Дрезден"},
		Timezone: location,
		Current: domain.Some(domain.CurrentPoint{
			Time:         time.Date(2026, 9, 10, 8, 0, 0, 0, location),
			TemperatureC: domain.Some(15.7),
			WeatherCode:  domain.Some(3),
			WindSpeedMS:  domain.Some(2.2),
		}),
		Days: []domain.DaySummary{
			{Date: time.Date(2026, 9, 10, 0, 0, 0, 0, location)},
			{Date: time.Date(2026, 9, 11, 0, 0, 0, 0, location)},
		},
		Hours: hours,
	}
}

func textMessage(chatID int64, text, languageCode string) *models.Update {
	return &models.Update{
		ID: 1,
		Message: &models.Message{
			ID:   1,
			Chat: models.Chat{ID: chatID},
			From: &models.User{ID: chatID, LanguageCode: languageCode},
			Text: text,
		},
	}
}

func callback(chatID int64, data string) *models.Update {
	return &models.Update{
		ID: 2,
		CallbackQuery: &models.CallbackQuery{
			ID:   "cb1",
			From: models.User{ID: chatID},
			Data: data,
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{ID: 1, Chat: models.Chat{ID: chatID}},
			},
		},
	}
}

func subscriber(chatID int64, lang i18n.Lang) domain.Subscriber {
	return domain.Subscriber{
		ChatID:      chatID,
		Lang:        string(lang),
		Place:       domain.Location{Name: "Дрезден", Latitude: 51.05, Longitude: 13.74},
		TZName:      "Europe/Berlin",
		ReportTime:  domain.DayTime{Hour: 7},
		ActiveHours: domain.HourWindow{Start: 7, End: 22},
		WindUnit:    domain.WindUnitMS,
	}
}

func TestStartCreatesSubscriberAndShowsKeyboard(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore())

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/start", "ru"))

	stored, err := harness.store.Get(context.Background(), 42)
	if err != nil {
		t.Fatalf("подписчик не создан: %v", err)
	}
	if stored.Lang != string(i18n.Russian) {
		t.Errorf("язык = %q, ожидался русский из Telegram", stored.Lang)
	}

	message := harness.api.last(t)
	if message.ChatID != 42 || message.ParseMode != "HTML" {
		t.Errorf("сообщение = %+v", message)
	}
	if len(message.ReplyMarkup.Keyboard) != 2 {
		t.Fatalf("клавиатура = %+v, ожидалось два ряда", message.ReplyMarkup.Keyboard)
	}
	if message.ReplyMarkup.Keyboard[0][0].Text != i18n.For(i18n.Russian).T(i18n.KeyButtonNow) {
		t.Errorf("первая кнопка = %q", message.ReplyMarkup.Keyboard[0][0].Text)
	}
	if !strings.Contains(message.Text, "Дрезден") {
		t.Errorf("в приветствии нет города:\n%s", message.Text)
	}
}

func TestPrivateBotRefusesStranger(t *testing.T) {
	harness := newHarness(t, privateAccess{allowed: 42}, newMemoryStore())

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(999, "/start", "en"))

	message := harness.api.last(t)
	if message.Text != i18n.For(i18n.Default).T(i18n.KeyPrivateBot) {
		t.Errorf("ответ чужому = %q", message.Text)
	}
	if count, _ := harness.store.Count(context.Background()); count != 0 {
		t.Errorf("подписчиков = %d, чужой не должен подписываться", count)
	}
}

func TestButtonsInEveryLanguage(t *testing.T) {
	for _, lang := range i18n.Supported {
		printer := i18n.For(lang)
		harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, lang)))

		buttons := map[string]string{
			printer.T(i18n.KeyButtonNow):      "Now",
			printer.T(i18n.KeyButtonToday):    "Today",
			printer.T(i18n.KeyButtonTomorrow): "Tomorrow",
			printer.T(i18n.KeyButtonSettings): "Settings",
		}

		for label, what := range buttons {
			harness.api.reset()
			harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, label, string(lang)))

			message := harness.api.last(t)
			if message.Text == printer.T(i18n.KeyUnknownCommand) {
				t.Errorf("%q: кнопка %s (%q) не распознана", lang, what, label)
			}
		}
	}
}

func TestTodayAndTomorrowReports(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/today", "ru"))
	today := harness.api.last(t)
	if !strings.Contains(today.Text, "По часам") {
		t.Errorf("в отчёте нет почасового блока:\n%s", today.Text)
	}

	harness.api.reset()
	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/tomorrow", "ru"))
	tomorrow := harness.api.last(t)
	if !strings.Contains(tomorrow.Text, "11 сентября") {
		t.Errorf("отчёт на завтра не про 11 сентября:\n%s", tomorrow.Text)
	}
}

func TestNowShowsCurrentWeather(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/now", "ru"))

	message := harness.api.last(t)
	if !strings.Contains(message.Text, "Сейчас в Дрезден") {
		t.Errorf("нет блока текущей погоды:\n%s", message.Text)
	}
	if !strings.Contains(message.Text, "16°") {
		t.Errorf("нет температуры:\n%s", message.Text)
	}
}

func TestSettingsShowsInlineKeyboard(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.English)))

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/settings", "en"))

	message := harness.api.last(t)
	if len(message.ReplyMarkup.InlineKeyboard) != 4 {
		t.Fatalf("inline-клавиатура = %+v", message.ReplyMarkup.InlineKeyboard)
	}
	for _, want := range []string{"Дрезден", "07:00", "Europe/Berlin", "m/s", "English"} {
		if !strings.Contains(message.Text, want) {
			t.Errorf("в настройках нет %q:\n%s", want, message.Text)
		}
	}
}

func TestChangeCityThroughSearch(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))
	harness.geocoder.places = []port.Place{
		{Name: "Прага", Country: "Чехия", TZName: "Europe/Prague",
			Place: domain.Location{Name: "Прага", Latitude: 50.08, Longitude: 14.44}},
		{Name: "Прага-Восток", Country: "Чехия", TZName: "Europe/Prague",
			Place: domain.Location{Name: "Прага-Восток", Latitude: 50.0, Longitude: 14.6}},
	}

	harness.bot.api.ProcessUpdate(context.Background(), callback(42, callbackAskCity))
	pending, _ := harness.store.Get(context.Background(), 42)
	if pending.Pending != domain.PendingCity {
		t.Fatalf("ожидание ввода = %q", pending.Pending)
	}

	harness.api.reset()
	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "Прага", "ru"))
	choices := harness.api.last(t)
	if len(choices.ReplyMarkup.InlineKeyboard) != 2 {
		t.Fatalf("варианты города = %+v", choices.ReplyMarkup.InlineKeyboard)
	}
	if harness.geocoder.query != "Прага" {
		t.Errorf("поисковый запрос = %q", harness.geocoder.query)
	}

	harness.api.reset()
	harness.bot.api.ProcessUpdate(context.Background(), callback(42, callbackCity+"0"))

	stored, _ := harness.store.Get(context.Background(), 42)
	if stored.Place.Name != "Прага" || stored.TZName != "Europe/Prague" {
		t.Errorf("город не сохранился: %+v", stored)
	}
	if stored.Pending != domain.PendingNone {
		t.Errorf("ожидание ввода не сброшено: %q", stored.Pending)
	}
	if !strings.Contains(harness.api.last(t).Text, "Europe/Prague") {
		t.Errorf("подтверждение без таймзоны:\n%s", harness.api.last(t).Text)
	}
}

func TestChangeCityWithSingleMatchSkipsChoice(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))
	harness.geocoder.places = []port.Place{
		{Name: "Прага", Country: "Чехия", TZName: "Europe/Prague",
			Place: domain.Location{Name: "Прага", Latitude: 50.08, Longitude: 14.44}},
	}

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/city Прага", "ru"))

	stored, _ := harness.store.Get(context.Background(), 42)
	if stored.Place.Name != "Прага" {
		t.Errorf("город = %q", stored.Place.Name)
	}
}

func TestChangeCityNotFound(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))
	harness.geocoder.err = fmt.Errorf("поиск: %w", geocode.ErrNothingFound)

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/city Атлантида", "ru"))

	if got := harness.api.last(t).Text; got != i18n.For(i18n.Russian).T(i18n.KeyCityNotFound) {
		t.Errorf("ответ = %q", got)
	}
}

func TestLocationSharingSetsPlaceAndTimezone(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))
	harness.timezones.name = "Asia/Tokyo"

	update := textMessage(42, "", "ru")
	update.Message.Location = &models.Location{Latitude: 35.68, Longitude: 139.69}
	harness.bot.api.ProcessUpdate(context.Background(), update)

	stored, _ := harness.store.Get(context.Background(), 42)
	if stored.TZName != "Asia/Tokyo" {
		t.Errorf("таймзона = %q", stored.TZName)
	}
	if stored.Place.Latitude != 35.68 {
		t.Errorf("широта = %v", stored.Place.Latitude)
	}
}

func TestChangeTime(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))

	harness.bot.api.ProcessUpdate(context.Background(), callback(42, callbackAskTime))
	pending, _ := harness.store.Get(context.Background(), 42)
	if pending.Pending != domain.PendingTime {
		t.Fatalf("ожидание ввода = %q", pending.Pending)
	}

	harness.api.reset()
	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "6:30", "ru"))

	stored, _ := harness.store.Get(context.Background(), 42)
	if stored.ReportTime.String() != "06:30" {
		t.Errorf("время рассылки = %q", stored.ReportTime)
	}
	if !strings.Contains(harness.api.last(t).Text, "06:30") {
		t.Errorf("подтверждение без времени:\n%s", harness.api.last(t).Text)
	}
}

func TestChangeTimeRejectsGarbage(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/time завтра", "ru"))

	if got := harness.api.last(t).Text; got != i18n.For(i18n.Russian).T(i18n.KeyTimeInvalid) {
		t.Errorf("ответ = %q", got)
	}
	stored, _ := harness.store.Get(context.Background(), 42)
	if stored.ReportTime.String() != "07:00" {
		t.Errorf("время не должно меняться: %q", stored.ReportTime)
	}
}

func TestToggleUnits(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))

	harness.bot.api.ProcessUpdate(context.Background(), callback(42, callbackUnit))
	stored, _ := harness.store.Get(context.Background(), 42)
	if stored.WindUnit != domain.WindUnitKMH {
		t.Errorf("единица = %q", stored.WindUnit)
	}

	harness.bot.api.ProcessUpdate(context.Background(), callback(42, callbackUnit))
	stored, _ = harness.store.Get(context.Background(), 42)
	if stored.WindUnit != domain.WindUnitMS {
		t.Errorf("единица = %q", stored.WindUnit)
	}
}

func TestChangeLanguage(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.English)))

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/language", "en"))
	menu := harness.api.last(t)
	if len(menu.ReplyMarkup.InlineKeyboard) != len(i18n.Supported) {
		t.Fatalf("в меню языков кнопок = %d", len(menu.ReplyMarkup.InlineKeyboard))
	}

	harness.api.reset()
	harness.bot.api.ProcessUpdate(context.Background(), callback(42, callbackLang+"de"))

	stored, _ := harness.store.Get(context.Background(), 42)
	if stored.Lang != string(i18n.German) {
		t.Errorf("язык = %q", stored.Lang)
	}
	if !strings.Contains(harness.api.last(t).Text, "Deutsch") {
		t.Errorf("подтверждение = %q", harness.api.last(t).Text)
	}

	harness.api.reset()
	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/settings", "en"))
	if !strings.Contains(harness.api.last(t).Text, "Einstellungen") {
		t.Errorf("настройки должны быть по-немецки:\n%s", harness.api.last(t).Text)
	}
}

func TestPauseAndResume(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))

	harness.bot.api.ProcessUpdate(context.Background(), callback(42, callbackPause))
	stored, _ := harness.store.Get(context.Background(), 42)
	if !stored.Paused {
		t.Error("рассылка должна встать на паузу")
	}

	harness.bot.api.ProcessUpdate(context.Background(), callback(42, callbackResume))
	stored, _ = harness.store.Get(context.Background(), 42)
	if stored.Paused {
		t.Error("рассылка должна возобновиться")
	}
}

func TestStopUnsubscribes(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/stop", "ru"))

	if count, _ := harness.store.Count(context.Background()); count != 0 {
		t.Errorf("подписчиков = %d, ожидался ноль", count)
	}
}

func TestStatsOnlyForAdmin(t *testing.T) {
	harness := newHarness(t, openAccess{admins: map[int64]bool{7: true}}, newMemoryStore(subscriber(42, i18n.Russian), subscriber(7, i18n.Russian)))

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/stats", "ru"))
	if got := harness.api.last(t).Text; got != i18n.For(i18n.Russian).T(i18n.KeyUnknownCommand) {
		t.Errorf("обычному пользователю ответ = %q", got)
	}

	harness.api.reset()
	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/stats", "ru"))
	harness.api.reset()
	harness.bot.api.ProcessUpdate(context.Background(), textMessage(7, "/stats", "ru"))
	if got := harness.api.last(t).Text; !strings.Contains(got, "2") {
		t.Errorf("админу ответ = %q", got)
	}
}

func TestUnknownCommand(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/danceparty", "ru"))

	if got := harness.api.last(t).Text; got != i18n.For(i18n.Russian).T(i18n.KeyUnknownCommand) {
		t.Errorf("ответ = %q", got)
	}
}

func TestCommandWithBotSuffix(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/today@weatherfit_bot", "ru"))

	if got := harness.api.last(t).Text; got == i18n.For(i18n.Russian).T(i18n.KeyUnknownCommand) {
		t.Error("команда с упоминанием бота должна распознаваться")
	}
}

func TestSendReportsBlockedRecipient(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))
	harness.api.sendErr = "Forbidden: bot was blocked by the user"

	err := harness.bot.Send(context.Background(), 42, "текст")
	if !errors.Is(err, usecase.ErrBlocked) {
		t.Errorf("ошибка = %v, ожидалась ErrBlocked", err)
	}
}

func TestSendAttachesKeyboard(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))

	if err := harness.bot.Send(context.Background(), 42, "утренний отчёт"); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	message := harness.api.last(t)
	if message.Text != "утренний отчёт" || message.ParseMode != "HTML" {
		t.Errorf("сообщение = %+v", message)
	}
	if len(message.ReplyMarkup.Keyboard) == 0 {
		t.Error("к рассылке должна прилагаться клавиатура")
	}
}

func TestNewRequiresToken(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Error("ожидалась ошибка для пустого токена")
	}
}

func TestMatchButton(t *testing.T) {
	for _, lang := range i18n.Supported {
		printer := i18n.For(lang)
		key, ok := matchButton(printer.T(i18n.KeyButtonTomorrow))
		if !ok || key != i18n.KeyButtonTomorrow {
			t.Errorf("%q: кнопка «завтра» не распознана", lang)
		}
	}
	if _, ok := matchButton("что-то ещё"); ok {
		t.Error("произвольный текст не должен считаться кнопкой")
	}
}

func TestChangeActiveHours(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))

	harness.bot.api.ProcessUpdate(context.Background(), callback(42, callbackAskHours))
	pending, _ := harness.store.Get(context.Background(), 42)
	if pending.Pending != domain.PendingHours {
		t.Fatalf("ожидание ввода = %q", pending.Pending)
	}

	harness.api.reset()
	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "09-18", "ru"))

	stored, _ := harness.store.Get(context.Background(), 42)
	if stored.ActiveHours.String() != "09-18" {
		t.Errorf("активные часы = %q", stored.ActiveHours)
	}
	if !strings.Contains(harness.api.last(t).Text, "09-18") {
		t.Errorf("подтверждение без часов:\n%s", harness.api.last(t).Text)
	}
}

func TestChangeActiveHoursRejectsGarbage(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.Russian)))

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/hours 22-07", "ru"))

	if got := harness.api.last(t).Text; got != i18n.For(i18n.Russian).T(i18n.KeyHoursInvalid) {
		t.Errorf("ответ = %q", got)
	}
	stored, _ := harness.store.Get(context.Background(), 42)
	if stored.ActiveHours.String() != "07-22" {
		t.Errorf("часы не должны меняться: %q", stored.ActiveHours)
	}
}

func TestActiveHoursNarrowReport(t *testing.T) {
	own := subscriber(42, i18n.Russian)
	own.ActiveHours = domain.HourWindow{Start: 9, End: 12}
	harness := newHarness(t, openAccess{}, newMemoryStore(own))

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/tomorrow", "ru"))

	message := harness.api.last(t)
	for _, hour := range []string{"09 ", "10 ", "11 ", "12 "} {
		if !strings.Contains(message.Text, hour) {
			t.Errorf("в отчёте нет часа %q:\n%s", hour, message.Text)
		}
	}
	if strings.Contains(message.Text, "\n13 ") || strings.Contains(message.Text, "\n08 ") {
		t.Errorf("в отчёте есть часы вне окна:\n%s", message.Text)
	}
}

func TestSettingsHasActiveHoursButton(t *testing.T) {
	harness := newHarness(t, openAccess{}, newMemoryStore(subscriber(42, i18n.English)))

	harness.bot.api.ProcessUpdate(context.Background(), textMessage(42, "/settings", "en"))

	message := harness.api.last(t)
	found := false
	for _, row := range message.ReplyMarkup.InlineKeyboard {
		for _, button := range row {
			if button.CallbackData == callbackAskHours {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("в настройках нет кнопки активных часов: %+v", message.ReplyMarkup.InlineKeyboard)
	}
}
