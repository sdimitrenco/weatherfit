package telegram

import (
	"github.com/go-telegram/bot/models"

	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/i18n"
)

// Command names, kept out of the catalogs because Telegram commands are ASCII.
const (
	commandStart    = "/start"
	commandHelp     = "/help"
	commandNow      = "/now"
	commandToday    = "/today"
	commandTomorrow = "/tomorrow"
	commandSettings = "/settings"
	commandCity     = "/city"
	commandTime     = "/time"
	commandLanguage = "/language"
	commandUnits    = "/units"
	commandPause    = "/pause"
	commandResume   = "/resume"
	commandStop     = "/stop"
	commandStats    = "/stats"
)

// Callback data prefixes for inline buttons.
const (
	callbackCity     = "city:"
	callbackLang     = "lang:"
	callbackUnit     = "unit"
	callbackPause    = "pause"
	callbackResume   = "resume"
	callbackAskCity  = "ask_city"
	callbackAskTime  = "ask_time"
	callbackLangMenu = "lang_menu"
	callbackSettings = "settings"
)

func mainKeyboard(printer *i18n.Printer) *models.ReplyKeyboardMarkup {
	return &models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: printer.T(i18n.KeyButtonNow)},
				{Text: printer.T(i18n.KeyButtonToday)},
			},
			{
				{Text: printer.T(i18n.KeyButtonTomorrow)},
				{Text: printer.T(i18n.KeyButtonSettings)},
			},
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func locationKeyboard(printer *i18n.Printer) *models.ReplyKeyboardMarkup {
	return &models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{{Text: printer.T(i18n.KeyButtonSendLocation), RequestLocation: true}},
			{{Text: printer.T(i18n.KeyButtonBack)}},
		},
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
	}
}

func settingsKeyboard(printer *i18n.Printer, subscriber domain.Subscriber) *models.InlineKeyboardMarkup {
	pauseButton := models.InlineKeyboardButton{
		Text:         printer.T(i18n.KeyButtonPause),
		CallbackData: callbackPause,
	}
	if subscriber.Paused {
		pauseButton = models.InlineKeyboardButton{
			Text:         printer.T(i18n.KeyButtonResume),
			CallbackData: callbackResume,
		}
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: printer.T(i18n.KeyButtonChangeCity), CallbackData: callbackAskCity}},
			{{Text: printer.T(i18n.KeyButtonChangeTime), CallbackData: callbackAskTime}},
			{
				{Text: printer.T(i18n.KeyButtonToggleUnit), CallbackData: callbackUnit},
				{Text: printer.T(i18n.KeyButtonChangeLanguage), CallbackData: callbackLangMenu},
			},
			{pauseButton},
		},
	}
}

func languageKeyboard() *models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, len(i18n.Supported)+1)
	for _, lang := range i18n.Supported {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         i18n.Name(lang),
			CallbackData: callbackLang + string(lang),
		}})
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func cityKeyboard(titles []string) *models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, len(titles))
	for index, title := range titles {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         title,
			CallbackData: callbackCity + itoa(index),
		}})
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}
