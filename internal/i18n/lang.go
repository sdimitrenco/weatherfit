// Package i18n turns language-free domain data into localized text.
package i18n

import "strings"

// Lang is a supported interface language.
type Lang string

const (
	English Lang = "en"
	Russian Lang = "ru"
	German  Lang = "de"

	// Default is used when Telegram sends no language or an unsupported one.
	Default = English
)

// Supported lists languages in the order shown in the settings keyboard.
var Supported = []Lang{English, Russian, German}

// Names maps a language to its own endonym plus a flag.
var Names = map[Lang]string{
	English: "🇬🇧 English",
	Russian: "🇷🇺 Русский",
	German:  "🇩🇪 Deutsch",
}

// Parse maps a Telegram language_code such as "ru-RU" onto a supported
// language, falling back to Default.
func Parse(code string) Lang {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return Default
	}
	if base, _, found := strings.Cut(code, "-"); found {
		code = base
	}
	if base, _, found := strings.Cut(code, "_"); found {
		code = base
	}

	for _, lang := range Supported {
		if Lang(code) == lang {
			return lang
		}
	}
	return Default
}

// Valid reports whether the language is supported.
func Valid(lang Lang) bool {
	for _, supported := range Supported {
		if lang == supported {
			return true
		}
	}
	return false
}

// Name returns the display name of a language.
func Name(lang Lang) string {
	if name, ok := Names[lang]; ok {
		return name
	}
	return string(lang)
}
