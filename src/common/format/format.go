// Package format provides locale-aware human-readable formatting for values
// displayed on user-facing surfaces.
package format

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/airports/src/common/i18n"
)

// Duration formats a duration using its largest fitting unit and at most one
// smaller unit. Negative durations are displayed as zero.
func Duration(lang string, duration time.Duration) string {
	seconds := int64(duration / time.Second)
	if seconds < 0 {
		seconds = 0
	}

	var first, second int64
	var firstKey, secondKey string
	switch {
	case seconds < 60:
		first, firstKey = seconds, "seconds"
	case seconds < 3600:
		first, second = seconds/60, seconds%60
		firstKey, secondKey = "minutes", "seconds"
	case seconds < 86400:
		first, second = seconds/3600, seconds%3600/60
		firstKey, secondKey = "hours", "minutes"
	default:
		first, second = seconds/86400, seconds%86400/3600
		firstKey, secondKey = "days", "hours"
	}

	result := durationUnit(lang, "duration", firstKey, first)
	if second > 0 {
		result += " " + durationUnit(lang, "duration", secondKey, second)
	}
	return result
}

// Size formats a byte count using 1024-based units through terabytes.
// Negative sizes are displayed as zero.
func Size(lang string, bytes int64) string {
	if bytes < 0 {
		bytes = 0
	}
	units := [...]string{"bytes", "kilobytes", "megabytes", "gigabytes", "terabytes"}
	value := float64(bytes)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}

	formatted := strconv.FormatFloat(math.Round(value*10)/10, 'f', 1, 64)
	formatted = strings.TrimSuffix(formatted, ".0")
	pluralCount := 2
	if math.Round(value*10)/10 == 1 {
		pluralCount = 1
	}
	name := i18n.TranslatePluralText(lang, "plurals.size."+units[unit], pluralCount)
	return formatted + " " + name
}

// Count formats an integer with the grouping conventions of the selected
// locale. Unsupported locale codes use English grouping.
func Count(lang string, count int64) string {
	separator := ","
	digits := []rune("0123456789")
	switch lang {
	case "es", "de", "it":
		separator = "."
	case "fr":
		separator = "\u202f"
	case "ar":
		separator = "٬"
		digits = []rune("٠١٢٣٤٥٦٧٨٩")
	case "ja", "zh", "en":
	default:
		lang = "en"
	}

	negative := count < 0
	var magnitude uint64
	if negative {
		magnitude = uint64(-(count + 1)) + 1
	} else {
		magnitude = uint64(count)
	}
	ascii := strconv.FormatUint(magnitude, 10)
	var grouped strings.Builder
	for index, digit := range ascii {
		if index > 0 && (len(ascii)-index)%3 == 0 {
			grouped.WriteString(separator)
		}
		if lang == "ar" {
			grouped.WriteRune(digits[digit-'0'])
		} else {
			grouped.WriteByte(byte(digit))
		}
	}
	if negative {
		return "-" + grouped.String()
	}
	return grouped.String()
}

func durationUnit(lang, family, unit string, count int64) string {
	name := i18n.TranslatePluralText(lang, fmt.Sprintf("plurals.%s.%s", family, unit), int(count))
	return Count(lang, count) + " " + name
}
