// Package telegram is the Telegram bot adapter: commands, buttons and delivery.
package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/sdimitrenco/weatherfit/internal/adapter/geocode"
	"github.com/sdimitrenco/weatherfit/internal/adapter/render"
	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/i18n"
	"github.com/sdimitrenco/weatherfit/internal/port"
	"github.com/sdimitrenco/weatherfit/internal/usecase"
)

// Access decides who may use the bot.
type Access interface {
	Allows(chatID int64) bool
	Admin(chatID int64) bool
}

// Options wires the bot adapter.
type Options struct {
	Token         string
	Access        Access
	Subscriptions *usecase.Subscriptions
	Reports       *usecase.BuildReport
	Renderer      *render.Renderer
	Log           *slog.Logger
	// BotOptions are passed through to go-telegram/bot, used by tests to
	// point the client at a stub server.
	BotOptions []bot.Option
}

// Bot serves Telegram updates over long polling.
type Bot struct {
	api           *bot.Bot
	access        Access
	subscriptions *usecase.Subscriptions
	reports       *usecase.BuildReport
	renderer      *render.Renderer
	log           *slog.Logger

	mu      sync.Mutex
	choices map[int64][]port.Place
}

// New creates the bot and registers the update handler.
func New(options Options) (*Bot, error) {
	if options.Token == "" {
		return nil, errors.New("telegram: не задан токен")
	}

	served := &Bot{
		access:        options.Access,
		subscriptions: options.Subscriptions,
		reports:       options.Reports,
		renderer:      options.Renderer,
		log:           options.Log,
		choices:       map[int64][]port.Place{},
	}

	botOptions := append([]bot.Option{
		bot.WithDefaultHandler(served.handle),
		bot.WithErrorsHandler(func(err error) {
			served.log.Error("ошибка Telegram", slog.String("error", err.Error()))
		}),
	}, options.BotOptions...)

	api, err := bot.New(options.Token, botOptions...)
	if err != nil {
		return nil, fmt.Errorf("telegram: не удалось создать бота: %w", err)
	}
	served.api = api
	return served, nil
}

// Start begins long polling and returns when the context is done.
func (b *Bot) Start(ctx context.Context) {
	b.registerCommands(ctx)
	b.api.Start(ctx)
}

// Send delivers a rendered message. A blocked recipient is reported as
// usecase.ErrBlocked so the caller can drop the subscription.
func (b *Bot) Send(ctx context.Context, chatID int64, text string) error {
	subscriber, err := b.subscriptions.Get(ctx, chatID)
	var markup models.ReplyMarkup
	if err == nil {
		markup = mainKeyboard(b.renderer.Printer(subscriber))
	}

	_, sendErr := b.api.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:             chatID,
		Text:               text,
		ParseMode:          models.ParseModeHTML,
		ReplyMarkup:        markup,
		LinkPreviewOptions: &models.LinkPreviewOptions{IsDisabled: bot.True()},
	})
	if sendErr != nil {
		if blocked(sendErr) {
			return errors.Join(usecase.ErrBlocked, sendErr)
		}
		return sendErr
	}
	return nil
}

func blocked(err error) bool {
	text := err.Error()
	return strings.Contains(text, "bot was blocked") ||
		strings.Contains(text, "user is deactivated") ||
		strings.Contains(text, "chat not found") ||
		strings.Contains(text, "Forbidden")
}

func (b *Bot) registerCommands(ctx context.Context) {
	printer := i18n.For(i18n.Default)
	commands := []models.BotCommand{
		{Command: "now", Description: printer.T(i18n.KeyButtonNow)},
		{Command: "today", Description: printer.T(i18n.KeyButtonToday)},
		{Command: "tomorrow", Description: printer.T(i18n.KeyButtonTomorrow)},
		{Command: "settings", Description: printer.T(i18n.KeyButtonSettings)},
		{Command: "city", Description: printer.T(i18n.KeyButtonChangeCity)},
		{Command: "time", Description: printer.T(i18n.KeyButtonChangeTime)},
		{Command: "hours", Description: printer.T(i18n.KeyButtonChangeHours)},
		{Command: "units", Description: printer.T(i18n.KeyButtonToggleUnit)},
		{Command: "language", Description: printer.T(i18n.KeyButtonChangeLanguage)},
		{Command: "help", Description: "help"},
		{Command: "stop", Description: "stop"},
	}

	if _, err := b.api.SetMyCommands(ctx, &bot.SetMyCommandsParams{Commands: commands}); err != nil {
		b.log.Warn("не удалось зарегистрировать команды", slog.String("error", err.Error()))
	}
}

func (b *Bot) handle(ctx context.Context, _ *bot.Bot, update *models.Update) {
	switch {
	case update.CallbackQuery != nil:
		b.handleCallback(ctx, update.CallbackQuery)
	case update.Message != nil:
		b.handleMessage(ctx, update.Message)
	}
}

func (b *Bot) handleMessage(ctx context.Context, message *models.Message) {
	chatID := message.Chat.ID

	if !b.access.Allows(chatID) {
		b.log.Info("сообщение от неизвестного chat_id",
			slog.Int64("chat_id", chatID),
			slog.String("text", message.Text),
		)
		b.reply(ctx, chatID, i18n.For(i18n.Default).T(i18n.KeyPrivateBot), nil)
		return
	}

	languageCode := ""
	if message.From != nil {
		languageCode = message.From.LanguageCode
	}

	subscriber, err := b.subscriptions.Ensure(ctx, chatID, languageCode)
	if err != nil {
		b.fail(ctx, chatID, "не удалось создать подписчика", err)
		return
	}

	if message.Location != nil {
		b.applyLocation(ctx, subscriber, message.Location)
		return
	}

	text := strings.TrimSpace(message.Text)
	if text == "" {
		return
	}

	if strings.HasPrefix(text, "/") {
		b.handleCommand(ctx, subscriber, text)
		return
	}
	if key, ok := matchButton(text); ok {
		b.handleButton(ctx, subscriber, key)
		return
	}
	if subscriber.Pending != domain.PendingNone {
		b.handlePendingInput(ctx, subscriber, text)
		return
	}

	b.reply(ctx, chatID, b.text(subscriber, i18n.KeyUnknownCommand), mainKeyboard(b.printer(subscriber)))
}

func (b *Bot) handleCommand(ctx context.Context, subscriber domain.Subscriber, text string) {
	command, argument := splitCommand(text)

	switch command {
	case commandStart:
		b.sendStart(ctx, subscriber)
	case commandHelp:
		b.reply(ctx, subscriber.ChatID,
			b.text(subscriber, i18n.KeyHelp, commandList(b.printer(subscriber))),
			mainKeyboard(b.printer(subscriber)))
	case commandNow:
		b.sendCurrent(ctx, subscriber)
	case commandToday:
		b.sendReport(ctx, subscriber, usecase.ReportToday)
	case commandTomorrow:
		b.sendReport(ctx, subscriber, usecase.ReportTomorrow)
	case commandSettings:
		b.sendSettings(ctx, subscriber)
	case commandCity:
		if argument == "" {
			b.askCity(ctx, subscriber)
			return
		}
		b.searchCity(ctx, subscriber, argument)
	case commandTime:
		if argument == "" {
			b.askTime(ctx, subscriber)
			return
		}
		b.applyTime(ctx, subscriber, argument)
	case commandHours:
		if argument == "" {
			b.askHours(ctx, subscriber)
			return
		}
		b.applyHours(ctx, subscriber, argument)
	case commandUnits:
		b.toggleUnit(ctx, subscriber)
	case commandLanguage:
		b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeyLanguageChoose), languageKeyboard())
	case commandPause:
		b.setPaused(ctx, subscriber, true)
	case commandResume:
		b.setPaused(ctx, subscriber, false)
	case commandStop:
		b.unsubscribe(ctx, subscriber)
	case commandStats:
		b.sendStats(ctx, subscriber)
	default:
		b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeyUnknownCommand), nil)
	}
}

func (b *Bot) handleButton(ctx context.Context, subscriber domain.Subscriber, key i18n.Key) {
	switch key {
	case i18n.KeyButtonNow:
		b.sendCurrent(ctx, subscriber)
	case i18n.KeyButtonToday:
		b.sendReport(ctx, subscriber, usecase.ReportToday)
	case i18n.KeyButtonTomorrow:
		b.sendReport(ctx, subscriber, usecase.ReportTomorrow)
	case i18n.KeyButtonSettings:
		b.sendSettings(ctx, subscriber)
	case i18n.KeyButtonBack:
		if err := b.subscriptions.SetPending(ctx, subscriber.ChatID, domain.PendingNone); err != nil {
			b.log.Warn("не удалось сбросить ожидание ввода", slog.String("error", err.Error()))
		}
		b.sendSettings(ctx, subscriber)
	}
}

func (b *Bot) handlePendingInput(ctx context.Context, subscriber domain.Subscriber, text string) {
	switch subscriber.Pending {
	case domain.PendingCity:
		b.searchCity(ctx, subscriber, text)
	case domain.PendingTime:
		b.applyTime(ctx, subscriber, text)
	case domain.PendingHours:
		b.applyHours(ctx, subscriber, text)
	case domain.PendingNone:
	}
}

func (b *Bot) handleCallback(ctx context.Context, query *models.CallbackQuery) {
	chatID := query.Message.Message.Chat.ID
	if !b.access.Allows(chatID) {
		return
	}

	b.answerCallback(ctx, query.ID)

	subscriber, err := b.subscriptions.Get(ctx, chatID)
	if err != nil {
		b.fail(ctx, chatID, "не удалось прочитать подписчика", err)
		return
	}

	data := query.Data
	switch {
	case data == callbackAskCity:
		b.askCity(ctx, subscriber)
	case data == callbackAskTime:
		b.askTime(ctx, subscriber)
	case data == callbackAskHours:
		b.askHours(ctx, subscriber)
	case data == callbackUnit:
		b.toggleUnit(ctx, subscriber)
	case data == callbackLangMenu:
		b.reply(ctx, chatID, b.text(subscriber, i18n.KeyLanguageChoose), languageKeyboard())
	case data == callbackPause:
		b.setPaused(ctx, subscriber, true)
	case data == callbackResume:
		b.setPaused(ctx, subscriber, false)
	case data == callbackSettings:
		b.sendSettings(ctx, subscriber)
	case strings.HasPrefix(data, callbackLang):
		b.applyLang(ctx, subscriber, i18n.Lang(strings.TrimPrefix(data, callbackLang)))
	case strings.HasPrefix(data, callbackCity):
		b.applyCityChoice(ctx, subscriber, strings.TrimPrefix(data, callbackCity))
	}
}

func (b *Bot) answerCallback(ctx context.Context, id string) {
	if _, err := b.api.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: id}); err != nil {
		b.log.Debug("не удалось подтвердить callback", slog.String("error", err.Error()))
	}
}

func splitCommand(text string) (string, string) {
	command, argument, _ := strings.Cut(text, " ")
	if at := strings.Index(command, "@"); at > 0 {
		command = command[:at]
	}
	return strings.ToLower(command), strings.TrimSpace(argument)
}

func (b *Bot) printer(subscriber domain.Subscriber) *i18n.Printer {
	return b.renderer.Printer(subscriber)
}

func (b *Bot) text(subscriber domain.Subscriber, key i18n.Key, args ...any) string {
	return b.printer(subscriber).T(key, args...)
}

func (b *Bot) reply(ctx context.Context, chatID int64, text string, markup models.ReplyMarkup) {
	_, err := b.api.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:             chatID,
		Text:               text,
		ParseMode:          models.ParseModeHTML,
		ReplyMarkup:        markup,
		LinkPreviewOptions: &models.LinkPreviewOptions{IsDisabled: bot.True()},
	})
	if err == nil {
		return
	}
	if blocked(err) {
		if deleteErr := b.subscriptions.Unsubscribe(ctx, chatID); deleteErr != nil {
			b.log.Error("не удалось удалить заблокировавшего подписчика", slog.String("error", deleteErr.Error()))
		}
		return
	}
	b.log.Error("не удалось отправить сообщение",
		slog.Int64("chat_id", chatID),
		slog.String("error", err.Error()),
	)
}

func (b *Bot) fail(ctx context.Context, chatID int64, what string, err error) {
	b.log.Error(what, slog.Int64("chat_id", chatID), slog.String("error", err.Error()))
	b.reply(ctx, chatID, i18n.For(i18n.Default).T(i18n.KeySomethingBroke), nil)
}

func (b *Bot) sendStart(ctx context.Context, subscriber domain.Subscriber) {
	printer := b.printer(subscriber)
	text := printer.T(i18n.KeyStart,
		subscriber.ReportTime.String(),
		escape(subscriber.Place.Name),
		commandList(printer),
	)
	b.reply(ctx, subscriber.ChatID, text, mainKeyboard(printer))
}

func (b *Bot) sendReport(ctx context.Context, subscriber domain.Subscriber, kind usecase.ReportKind) {
	report, err := b.reports.Build(ctx, subscriber, kind)
	if err != nil {
		b.log.Warn("не удалось собрать отчёт",
			slog.Int64("chat_id", subscriber.ChatID),
			slog.String("error", err.Error()),
		)
		b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeyForecastFailed), mainKeyboard(b.printer(subscriber)))
		return
	}
	b.reply(ctx, subscriber.ChatID, b.renderer.Report(report, subscriber), mainKeyboard(b.printer(subscriber)))
}

func (b *Bot) sendCurrent(ctx context.Context, subscriber domain.Subscriber) {
	current, err := b.reports.Current(ctx, subscriber)
	if err != nil {
		b.log.Warn("не удалось получить текущую погоду",
			slog.Int64("chat_id", subscriber.ChatID),
			slog.String("error", err.Error()),
		)
		b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeyForecastFailed), mainKeyboard(b.printer(subscriber)))
		return
	}
	b.reply(ctx, subscriber.ChatID,
		b.renderer.Current(subscriber.Place, current, subscriber),
		mainKeyboard(b.printer(subscriber)))
}

func (b *Bot) sendSettings(ctx context.Context, subscriber domain.Subscriber) {
	printer := b.printer(subscriber)

	delivery := printer.T(i18n.KeySettingsActive)
	if subscriber.Paused {
		delivery = printer.T(i18n.KeySettingsPaused)
	}

	lines := []string{
		printer.T(i18n.KeySettingsTitle),
		printer.T(i18n.KeySettingsCity, escape(subscriber.Place.Name)),
		printer.T(i18n.KeySettingsTime, subscriber.ReportTime.String(), escape(subscriber.TZName)),
		printer.T(i18n.KeySettingsHours, subscriber.ActiveHours.String()),
		printer.T(i18n.KeySettingsUnit, printer.Unit(subscriber.WindUnit)),
		printer.T(i18n.KeySettingsLanguage, i18n.Name(i18n.Lang(subscriber.Lang))),
		delivery,
	}

	b.reply(ctx, subscriber.ChatID, strings.Join(lines, "\n"), settingsKeyboard(printer, subscriber))
}

func (b *Bot) askCity(ctx context.Context, subscriber domain.Subscriber) {
	if err := b.subscriptions.SetPending(ctx, subscriber.ChatID, domain.PendingCity); err != nil {
		b.fail(ctx, subscriber.ChatID, "не удалось сохранить ожидание города", err)
		return
	}
	b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeyAskCity), locationKeyboard(b.printer(subscriber)))
}

func (b *Bot) askTime(ctx context.Context, subscriber domain.Subscriber) {
	if err := b.subscriptions.SetPending(ctx, subscriber.ChatID, domain.PendingTime); err != nil {
		b.fail(ctx, subscriber.ChatID, "не удалось сохранить ожидание времени", err)
		return
	}
	b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeyAskTime), nil)
}

func (b *Bot) askHours(ctx context.Context, subscriber domain.Subscriber) {
	if err := b.subscriptions.SetPending(ctx, subscriber.ChatID, domain.PendingHours); err != nil {
		b.fail(ctx, subscriber.ChatID, "не удалось сохранить ожидание активных часов", err)
		return
	}
	b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeyAskHours), nil)
}

func (b *Bot) applyHours(ctx context.Context, subscriber domain.Subscriber, raw string) {
	updated, err := b.subscriptions.SetActiveHours(ctx, subscriber.ChatID, raw)
	if err != nil {
		b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeyHoursInvalid), nil)
		return
	}
	b.reply(ctx, updated.ChatID,
		b.text(updated, i18n.KeyHoursSaved, updated.ActiveHours.String()),
		mainKeyboard(b.printer(updated)))
}

func (b *Bot) searchCity(ctx context.Context, subscriber domain.Subscriber, query string) {
	places, err := b.subscriptions.SearchCities(ctx, query)
	if err != nil {
		if errors.Is(err, geocode.ErrNothingFound) {
			b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeyCityNotFound), nil)
			return
		}
		b.log.Warn("поиск города не удался",
			slog.Int64("chat_id", subscriber.ChatID),
			slog.String("query", query),
			slog.String("error", err.Error()),
		)
		b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeySomethingBroke), nil)
		return
	}

	if len(places) == 1 {
		b.applyPlace(ctx, subscriber, places[0])
		return
	}

	b.rememberChoices(subscriber.ChatID, places)

	titles := make([]string, 0, len(places))
	for _, place := range places {
		titles = append(titles, geocode.Title(place))
	}
	b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeyCityChoose), cityKeyboard(titles))
}

func (b *Bot) applyCityChoice(ctx context.Context, subscriber domain.Subscriber, raw string) {
	index, err := strconv.Atoi(raw)
	if err != nil {
		return
	}

	places := b.recallChoices(subscriber.ChatID)
	if index < 0 || index >= len(places) {
		b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeyAskCity), locationKeyboard(b.printer(subscriber)))
		return
	}
	b.applyPlace(ctx, subscriber, places[index])
}

func (b *Bot) applyPlace(ctx context.Context, subscriber domain.Subscriber, place port.Place) {
	updated, err := b.subscriptions.SetPlace(ctx, subscriber.ChatID, place)
	if err != nil {
		b.fail(ctx, subscriber.ChatID, "не удалось сохранить город", err)
		return
	}
	b.forgetChoices(subscriber.ChatID)

	b.reply(ctx, updated.ChatID,
		b.text(updated, i18n.KeyCitySaved, escape(geocode.Title(place)), escape(updated.TZName)),
		mainKeyboard(b.printer(updated)))
}

func (b *Bot) applyLocation(ctx context.Context, subscriber domain.Subscriber, location *models.Location) {
	updated, err := b.subscriptions.SetCoordinates(ctx, subscriber.ChatID, domain.Location{
		Latitude:  location.Latitude,
		Longitude: location.Longitude,
	})
	if err != nil {
		b.log.Warn("не удалось применить геопозицию",
			slog.Int64("chat_id", subscriber.ChatID),
			slog.String("error", err.Error()),
		)
		b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeySomethingBroke), nil)
		return
	}

	b.reply(ctx, updated.ChatID,
		b.text(updated, i18n.KeyCitySaved, escape(updated.Place.Name), escape(updated.TZName)),
		mainKeyboard(b.printer(updated)))
}

func (b *Bot) applyTime(ctx context.Context, subscriber domain.Subscriber, raw string) {
	updated, err := b.subscriptions.SetReportTime(ctx, subscriber.ChatID, raw)
	if err != nil {
		b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeyTimeInvalid), nil)
		return
	}
	b.reply(ctx, updated.ChatID,
		b.text(updated, i18n.KeyTimeSaved, updated.ReportTime.String(), escape(updated.TZName)),
		mainKeyboard(b.printer(updated)))
}

func (b *Bot) toggleUnit(ctx context.Context, subscriber domain.Subscriber) {
	updated, err := b.subscriptions.ToggleWindUnit(ctx, subscriber.ChatID)
	if err != nil {
		b.fail(ctx, subscriber.ChatID, "не удалось переключить единицу ветра", err)
		return
	}
	printer := b.printer(updated)
	b.reply(ctx, updated.ChatID, printer.T(i18n.KeyUnitSaved, printer.Unit(updated.WindUnit)), nil)
}

func (b *Bot) applyLang(ctx context.Context, subscriber domain.Subscriber, lang i18n.Lang) {
	updated, err := b.subscriptions.SetLang(ctx, subscriber.ChatID, lang)
	if err != nil {
		b.fail(ctx, subscriber.ChatID, "не удалось сменить язык", err)
		return
	}
	b.reply(ctx, updated.ChatID,
		b.text(updated, i18n.KeyLanguageSaved, i18n.Name(lang)),
		mainKeyboard(b.printer(updated)))
}

func (b *Bot) setPaused(ctx context.Context, subscriber domain.Subscriber, paused bool) {
	updated, err := b.subscriptions.SetPaused(ctx, subscriber.ChatID, paused)
	if err != nil {
		b.fail(ctx, subscriber.ChatID, "не удалось изменить состояние рассылки", err)
		return
	}

	key := i18n.KeyResumedSaved
	if paused {
		key = i18n.KeyPausedSaved
	}
	b.reply(ctx, updated.ChatID, b.text(updated, key), mainKeyboard(b.printer(updated)))
}

func (b *Bot) unsubscribe(ctx context.Context, subscriber domain.Subscriber) {
	if err := b.subscriptions.Unsubscribe(ctx, subscriber.ChatID); err != nil {
		b.fail(ctx, subscriber.ChatID, "не удалось отписать", err)
		return
	}
	b.log.Info("подписчик отписался", slog.Int64("chat_id", subscriber.ChatID))
	b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeyStopped), nil)
}

func (b *Bot) sendStats(ctx context.Context, subscriber domain.Subscriber) {
	if !b.access.Admin(subscriber.ChatID) {
		b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeyUnknownCommand), nil)
		return
	}

	count, err := b.subscriptions.Count(ctx)
	if err != nil {
		b.fail(ctx, subscriber.ChatID, "не удалось посчитать подписчиков", err)
		return
	}
	b.reply(ctx, subscriber.ChatID, b.text(subscriber, i18n.KeyStats, count), nil)
}

func (b *Bot) rememberChoices(chatID int64, places []port.Place) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.choices[chatID] = places
}

func (b *Bot) recallChoices(chatID int64) []port.Place {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.choices[chatID]
}

func (b *Bot) forgetChoices(chatID int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.choices, chatID)
}
