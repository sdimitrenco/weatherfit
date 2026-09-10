package telegram

import (
	"strconv"
	"strings"

	"github.com/sdimitrenco/weatherfit/internal/i18n"
)

func itoa(value int) string {
	return strconv.Itoa(value)
}

// escape prepares dynamic values for Telegram HTML parse mode.
func escape(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	return strings.ReplaceAll(text, ">", "&gt;")
}

// buttonKeys are the reply-keyboard labels the dispatcher recognizes in every
// language, so a user who switches language mid-session keeps working buttons.
var buttonKeys = []i18n.Key{
	i18n.KeyButtonNow,
	i18n.KeyButtonToday,
	i18n.KeyButtonTomorrow,
	i18n.KeyButtonSettings,
	i18n.KeyButtonBack,
}

// matchButton reports which known button the text belongs to, in any language.
func matchButton(text string) (i18n.Key, bool) {
	text = strings.TrimSpace(text)
	for _, lang := range i18n.Supported {
		printer := i18n.For(lang)
		for _, key := range buttonKeys {
			if printer.T(key) == text {
				return key, true
			}
		}
	}
	return "", false
}

func commandList(printer *i18n.Printer) string {
	lines := []string{
		commandNow + " — " + printer.T(i18n.KeyButtonNow),
		commandToday + " — " + printer.T(i18n.KeyButtonToday),
		commandTomorrow + " — " + printer.T(i18n.KeyButtonTomorrow),
		commandSettings + " — " + printer.T(i18n.KeyButtonSettings),
		commandCity + " — " + printer.T(i18n.KeyButtonChangeCity),
		commandTime + " — " + printer.T(i18n.KeyButtonChangeTime),
		commandHours + " — " + printer.T(i18n.KeyButtonChangeHours),
		commandUnits + " — " + printer.T(i18n.KeyButtonToggleUnit),
		commandLanguage + " — " + printer.T(i18n.KeyButtonChangeLanguage),
		commandStop + " — /stop",
	}
	return strings.Join(lines, "\n")
}
