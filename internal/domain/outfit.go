package domain

import (
	"math"
	"sort"
)

const (
	LayeringSpreadC  = 8.0
	UVGlassesIndex   = 3.0
	UVSunscreenIndex = 6.0

	maxAdviceItems = 4
)

// BandID identifies a clothing band for translation lookup.
type BandID string

const (
	BandTShirtShorts BandID = "tshirt_shorts"
	BandTShirtPants  BandID = "tshirt_pants"
	BandHoodieJacket BandID = "hoodie_jacket"
	BandSweaterCoat  BandID = "sweater_coat"
	BandWarmCoat     BandID = "warm_coat"
	BandWinterCoat   BandID = "winter_coat"
	BandDownJacket   BandID = "down_jacket"
)

// AdviceKind identifies one advice sentence for translation lookup.
type AdviceKind string

const (
	AdviceBase              AdviceKind = "base"
	AdviceBaseNoTemperature AdviceKind = "base_no_temperature"
	AdviceLayering          AdviceKind = "layering"
	AdviceLayeringWithOuter AdviceKind = "layering_with_outer"
	AdviceRainMaybe         AdviceKind = "rain_maybe"
	AdviceRainUmbrella      AdviceKind = "rain_umbrella"
	AdviceRainCoat          AdviceKind = "rain_coat"
	AdviceWaterproofShoes   AdviceKind = "waterproof_shoes"
	AdviceSnow              AdviceKind = "snow"
	AdviceHazardThunder     AdviceKind = "hazard_thunder"
	AdviceHazardIce         AdviceKind = "hazard_ice"
	AdviceHazardSnow        AdviceKind = "hazard_snow"
	AdviceWindproof         AdviceKind = "windproof"
	AdviceWindproofGusts    AdviceKind = "windproof_gusts"
	AdviceUVGlasses         AdviceKind = "uv_glasses"
	AdviceUVSunscreen       AdviceKind = "uv_sunscreen"
)

type outfitBand struct {
	fromApparentC float64
	id            BandID
	hasOuterLayer bool
}

var outfitBands = []outfitBand{
	{fromApparentC: 25, id: BandTShirtShorts},
	{fromApparentC: 18, id: BandTShirtPants},
	{fromApparentC: 12, id: BandHoodieJacket, hasOuterLayer: true},
	{fromApparentC: 5, id: BandSweaterCoat, hasOuterLayer: true},
	{fromApparentC: 0, id: BandWarmCoat, hasOuterLayer: true},
	{fromApparentC: -10, id: BandWinterCoat, hasOuterLayer: true},
	{fromApparentC: math.Inf(-1), id: BandDownJacket},
}

// AdviceItem carries everything a translation needs to render one sentence.
type AdviceItem struct {
	Kind         AdviceKind
	Band         BandID
	TemperatureC Opt[float64]
	SpeedMS      Opt[float64]
	UVIndex      Opt[float64]
	WindLevel    WindLevel
	Windows      []RainWindow
}

// OutfitInput holds the inputs the clothing rules work on.
type OutfitInput struct {
	ApparentMinC  Opt[float64]
	ApparentMaxC  Opt[float64]
	Wind          WindSummary
	Precipitation PrecipitationAnalysis
	UVIndexMax    Opt[float64]
}

// OutfitAdvice is an ordered list of at most four advice items.
type OutfitAdvice struct {
	Items []AdviceItem
}

type rankedItem struct {
	order    int
	priority int
	item     AdviceItem
}

// BuildOutfitAdvice turns the forecast summary into structured advice.
func BuildOutfitAdvice(in OutfitInput) OutfitAdvice {
	band := bandFor(in.ApparentMinC)
	windows := in.Precipitation.Windows

	var ranked []rankedItem
	add := func(order, priority int, item AdviceItem) {
		ranked = append(ranked, rankedItem{order: order, priority: priority, item: item})
	}

	if value, ok := in.ApparentMinC.Get(); ok {
		add(1, 1, AdviceItem{Kind: AdviceBase, Band: band.id, TemperatureC: Some(value)})
	} else {
		add(1, 1, AdviceItem{Kind: AdviceBaseNoTemperature, Band: band.id})
	}

	if item, ok := layeringItem(in, band); ok {
		add(2, 4, item)
	}
	for _, item := range precipitationItems(in, windows) {
		add(3, 2, item)
	}
	for _, item := range hazardItems(in) {
		add(4, 3, item)
	}
	if item, ok := windItem(in); ok {
		add(5, 5, item)
	}
	if item, ok := uvItem(in); ok {
		add(6, 6, item)
	}

	return OutfitAdvice{Items: pickItems(ranked)}
}

func pickItems(ranked []rankedItem) []AdviceItem {
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].priority < ranked[j].priority })
	if len(ranked) > maxAdviceItems {
		ranked = ranked[:maxAdviceItems]
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].order < ranked[j].order })

	items := make([]AdviceItem, 0, len(ranked))
	for _, entry := range ranked {
		items = append(items, entry.item)
	}
	return items
}

func bandFor(apparentMin Opt[float64]) outfitBand {
	value, ok := apparentMin.Get()
	if !ok {
		return outfitBands[2]
	}
	for _, band := range outfitBands {
		if value >= band.fromApparentC {
			return band
		}
	}
	return outfitBands[len(outfitBands)-1]
}

func layeringItem(in OutfitInput, band outfitBand) (AdviceItem, bool) {
	minimum, hasMin := in.ApparentMinC.Get()
	maximum, hasMax := in.ApparentMaxC.Get()
	if !hasMin || !hasMax || maximum-minimum < LayeringSpreadC {
		return AdviceItem{}, false
	}

	kind := AdviceLayering
	if band.hasOuterLayer {
		kind = AdviceLayeringWithOuter
	}
	return AdviceItem{Kind: kind, Band: band.id, TemperatureC: Some(maximum)}, true
}

func precipitationItems(in OutfitInput, windows []RainWindow) []AdviceItem {
	if in.Precipitation.Verdict == RainNotNeeded {
		return nil
	}

	if in.Precipitation.SnowDominant() {
		return []AdviceItem{{Kind: AdviceSnow, Windows: windows}}
	}

	var items []AdviceItem
	switch in.Precipitation.Verdict {
	case RainJustInCase:
		items = append(items, AdviceItem{Kind: AdviceRainMaybe, Windows: windows})
	case RainRequired:
		kind := AdviceRainUmbrella
		if in.Precipitation.UmbrellaUseless(in.Wind) {
			kind = AdviceRainCoat
		}
		items = append(items, AdviceItem{Kind: kind, Windows: windows})
		if in.Precipitation.Heavy {
			items = append(items, AdviceItem{Kind: AdviceWaterproofShoes})
		}
	case RainNotNeeded:
	}
	return items
}

func hazardItems(in OutfitInput) []AdviceItem {
	var items []AdviceItem
	if in.Precipitation.Thunderstorm {
		items = append(items, AdviceItem{Kind: AdviceHazardThunder})
	}
	if in.Precipitation.Freezing {
		items = append(items, AdviceItem{Kind: AdviceHazardIce})
	}
	if in.Precipitation.Snow && !in.Precipitation.SnowDominant() {
		items = append(items, AdviceItem{Kind: AdviceHazardSnow})
	}
	return items
}

func windItem(in OutfitInput) (AdviceItem, bool) {
	if !in.Wind.Level.NeedsWindproof() && !in.Wind.GustWarning {
		return AdviceItem{}, false
	}
	if gusts, ok := in.Wind.MaxGustsMS.Get(); ok && in.Wind.GustWarning {
		return AdviceItem{Kind: AdviceWindproofGusts, WindLevel: in.Wind.Level, SpeedMS: Some(gusts)}, true
	}
	return AdviceItem{Kind: AdviceWindproof, WindLevel: in.Wind.Level}, true
}

func uvItem(in OutfitInput) (AdviceItem, bool) {
	index, ok := in.UVIndexMax.Get()
	if !ok || index < UVGlassesIndex {
		return AdviceItem{}, false
	}
	kind := AdviceUVGlasses
	if index >= UVSunscreenIndex {
		kind = AdviceUVSunscreen
	}
	return AdviceItem{Kind: kind, UVIndex: Some(index)}, true
}
