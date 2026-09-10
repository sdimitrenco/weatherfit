package i18n

import (
	"math"
	"strconv"
)

// Missing marks a value the API did not provide.
const Missing = "—"

// Number formats a float without a locale-specific decimal separator and
// without a negative zero.
func Number(value float64, decimals int) string {
	if decimals == 0 {
		value = math.Round(value)
	}
	text := strconv.FormatFloat(value, 'f', decimals, 64)
	if text == "-0" {
		return "0"
	}
	return text
}
