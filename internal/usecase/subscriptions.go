package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sdimitrenco/weatherfit/internal/domain"
	"github.com/sdimitrenco/weatherfit/internal/i18n"
	"github.com/sdimitrenco/weatherfit/internal/port"
)

// Defaults are the settings a brand new subscriber starts with.
type Defaults struct {
	Place       domain.Location
	TZName      string
	ReportTime  domain.DayTime
	ActiveHours domain.HourWindow
	WindUnit    domain.WindUnit
	Lang        i18n.Lang
}

// Subscriptions owns everything a user can change about their subscription.
type Subscriptions struct {
	store     port.SubscriberStore
	geocoder  port.Geocoder
	timezones port.TimezoneResolver
	clock     port.Clock
	defaults  Defaults
}

// NewSubscriptions wires the use case.
func NewSubscriptions(
	store port.SubscriberStore,
	geocoder port.Geocoder,
	timezones port.TimezoneResolver,
	clock port.Clock,
	defaults Defaults,
) *Subscriptions {
	return &Subscriptions{
		store:     store,
		geocoder:  geocoder,
		timezones: timezones,
		clock:     clock,
		defaults:  defaults,
	}
}

// CityChoices is how many search results are offered to the user.
const CityChoices = 5

// Ensure returns the subscriber, creating one on first contact. The Telegram
// language_code is only used for a new subscriber, so a manual choice sticks.
func (s *Subscriptions) Ensure(ctx context.Context, chatID int64, languageCode string) (domain.Subscriber, error) {
	subscriber, err := s.store.Get(ctx, chatID)
	if err == nil {
		return subscriber, nil
	}
	if !errors.Is(err, port.ErrSubscriberNotFound) {
		return domain.Subscriber{}, err
	}

	now := s.clock.Now().UTC()
	lang := s.defaults.Lang
	if languageCode != "" {
		lang = i18n.Parse(languageCode)
	}

	subscriber = domain.Subscriber{
		ChatID:      chatID,
		Lang:        string(lang),
		Place:       s.defaults.Place,
		TZName:      s.defaults.TZName,
		ReportTime:  s.defaults.ReportTime,
		ActiveHours: s.defaults.ActiveHours,
		WindUnit:    s.defaults.WindUnit,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.store.Save(ctx, subscriber); err != nil {
		return domain.Subscriber{}, err
	}
	return subscriber, nil
}

// Get returns a stored subscriber.
func (s *Subscriptions) Get(ctx context.Context, chatID int64) (domain.Subscriber, error) {
	return s.store.Get(ctx, chatID)
}

// Count returns the number of subscribers.
func (s *Subscriptions) Count(ctx context.Context) (int, error) {
	return s.store.Count(ctx)
}

// SearchCities looks up places by name.
func (s *Subscriptions) SearchCities(ctx context.Context, query string) ([]port.Place, error) {
	return s.geocoder.Search(ctx, query, CityChoices)
}

// SetPlace stores a place found by the geocoder.
func (s *Subscriptions) SetPlace(ctx context.Context, chatID int64, place port.Place) (domain.Subscriber, error) {
	return s.update(ctx, chatID, func(subscriber *domain.Subscriber) error {
		subscriber.Place = place.Place
		subscriber.TZName = place.TZName
		return nil
	})
}

// SetCoordinates stores a shared location, resolving its timezone.
func (s *Subscriptions) SetCoordinates(ctx context.Context, chatID int64, place domain.Location) (domain.Subscriber, error) {
	if err := place.Validate(); err != nil {
		return domain.Subscriber{}, err
	}

	tzName, err := s.timezones.ResolveTimezone(ctx, place)
	if err != nil {
		return domain.Subscriber{}, err
	}

	return s.update(ctx, chatID, func(subscriber *domain.Subscriber) error {
		if place.Name == "" {
			place.Name = fmt.Sprintf("%.3f, %.3f", place.Latitude, place.Longitude)
		}
		subscriber.Place = place
		subscriber.TZName = tzName
		return nil
	})
}

// SetReportTime parses and stores the morning report time.
func (s *Subscriptions) SetReportTime(ctx context.Context, chatID int64, raw string) (domain.Subscriber, error) {
	reportTime, err := domain.ParseDayTime(raw)
	if err != nil {
		return domain.Subscriber{}, err
	}

	return s.update(ctx, chatID, func(subscriber *domain.Subscriber) error {
		subscriber.ReportTime = reportTime
		// A new time must be able to fire today, so forget today's delivery.
		if subscriber.LastSentDate == s.clock.Now().In(subscriber.Timezone()).Format(time.DateOnly) {
			subscriber.LastSentDate = ""
		}
		return nil
	})
}

// SetActiveHours parses and stores the window the report covers.
func (s *Subscriptions) SetActiveHours(ctx context.Context, chatID int64, raw string) (domain.Subscriber, error) {
	window, err := domain.ParseHourWindow(raw)
	if err != nil {
		return domain.Subscriber{}, err
	}

	return s.update(ctx, chatID, func(subscriber *domain.Subscriber) error {
		subscriber.ActiveHours = window
		return nil
	})
}

// SetLang stores the interface language.
func (s *Subscriptions) SetLang(ctx context.Context, chatID int64, lang i18n.Lang) (domain.Subscriber, error) {
	if !i18n.Valid(lang) {
		return domain.Subscriber{}, fmt.Errorf("usecase: язык %q не поддерживается", lang)
	}
	return s.update(ctx, chatID, func(subscriber *domain.Subscriber) error {
		subscriber.Lang = string(lang)
		return nil
	})
}

// ToggleWindUnit switches between metres per second and kilometres per hour.
func (s *Subscriptions) ToggleWindUnit(ctx context.Context, chatID int64) (domain.Subscriber, error) {
	return s.update(ctx, chatID, func(subscriber *domain.Subscriber) error {
		if subscriber.WindUnit == domain.WindUnitKMH {
			subscriber.WindUnit = domain.WindUnitMS
			return nil
		}
		subscriber.WindUnit = domain.WindUnitKMH
		return nil
	})
}

// SetPaused pauses or resumes the morning delivery.
func (s *Subscriptions) SetPaused(ctx context.Context, chatID int64, paused bool) (domain.Subscriber, error) {
	return s.update(ctx, chatID, func(subscriber *domain.Subscriber) error {
		subscriber.Paused = paused
		return nil
	})
}

// SetPending records what input the bot expects next.
func (s *Subscriptions) SetPending(ctx context.Context, chatID int64, pending domain.PendingAction) error {
	return s.store.SetPending(ctx, chatID, pending)
}

// Unsubscribe removes the subscriber.
func (s *Subscriptions) Unsubscribe(ctx context.Context, chatID int64) error {
	return s.store.Delete(ctx, chatID)
}

func (s *Subscriptions) update(ctx context.Context, chatID int64, change func(*domain.Subscriber) error) (domain.Subscriber, error) {
	subscriber, err := s.store.Get(ctx, chatID)
	if err != nil {
		return domain.Subscriber{}, err
	}
	if err := change(&subscriber); err != nil {
		return domain.Subscriber{}, err
	}

	subscriber.Pending = domain.PendingNone
	subscriber.UpdatedAt = s.clock.Now().UTC()
	if err := s.store.Save(ctx, subscriber); err != nil {
		return domain.Subscriber{}, err
	}
	return subscriber, nil
}
