package render

import (
	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/i18n"
)

// Renderer implements port.ReportRenderer for one message layout.
type Renderer struct {
	layout Layout
}

// NewRenderer builds a renderer for the configured hourly layout.
func NewRenderer(layout Layout) *Renderer {
	if layout == "" {
		layout = LayoutLines
	}
	return &Renderer{layout: layout}
}

// Report renders the daily message in the subscriber language.
func (r *Renderer) Report(report domain.Report, subscriber domain.Subscriber) string {
	return Report(report, r.options(subscriber))
}

// Current renders the "right now" message in the subscriber language.
func (r *Renderer) Current(place domain.Location, current domain.CurrentPoint, subscriber domain.Subscriber) string {
	return Current(place, current, r.options(subscriber))
}

// Text renders a single catalog string in the subscriber language.
func (r *Renderer) Text(subscriber domain.Subscriber, key string, args ...any) string {
	return r.Printer(subscriber).T(i18n.Key(key), args...)
}

// Printer returns the printer for the subscriber language.
func (r *Renderer) Printer(subscriber domain.Subscriber) *i18n.Printer {
	return i18n.For(i18n.Lang(subscriber.Lang))
}

func (r *Renderer) options(subscriber domain.Subscriber) Options {
	return Options{
		Printer:  r.Printer(subscriber),
		WindUnit: subscriber.WindUnit,
		Layout:   r.layout,
	}
}
