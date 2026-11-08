package appearance

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/arfush/yo/ui/resource"
)

// Format keys are private to appearance; they are resolved through the active
// language and never hardcoded in screens.
const (
	formatGroup         resource.TextKey = "format.group"
	formatDecimal       resource.TextKey = "format.decimal"
	formatPercent       resource.TextKey = "format.percent"
	formatDate          resource.TextKey = "format.date"
	formatDateTime      resource.TextKey = "format.dateTime"
	formatDurationMin   resource.TextKey = "format.duration.minutes"
	formatDurationHours resource.TextKey = "format.duration.hours"
)

// Number formats an integer with the locale grouping separator.
func (l Locale) Number(n int64) string {
	digits := strconv.FormatUint(magnitude(n), 10)
	grouped := groupDigits(digits, l.Text(formatGroup))
	if n < 0 {
		return "-" + grouped
	}
	return grouped
}

// Decimal formats v with exactly places fractional digits, using the locale
// grouping and decimal separators.
func (l Locale) Decimal(v float64, places int) string {
	if places < 0 {
		panic("appearance: negative decimal places")
	}
	formatted := strconv.FormatFloat(v, 'f', places, 64)
	switch formatted {
	case "NaN", "+Inf", "-Inf":
		return formatted
	}

	negative := strings.HasPrefix(formatted, "-")
	formatted = strings.TrimPrefix(formatted, "-")
	integer, fraction, hasFraction := strings.Cut(formatted, ".")
	integer = groupDigits(integer, l.Text(formatGroup))
	if hasFraction {
		integer += l.Text(formatDecimal) + fraction
	}
	if negative {
		return "-" + integer
	}
	return integer
}

// Percent formats ratio as a percentage, e.g. 0.126 -> "12.6%".
func (l Locale) Percent(ratio float64, places int) string {
	return fmt.Sprintf(l.Text(formatPercent), l.Decimal(ratio*100, places))
}

// Date formats t with the locale date pattern.
func (l Locale) Date(t time.Time) string {
	return t.Format(l.Text(formatDate))
}

// DateTime formats t with the locale date and time pattern.
func (l Locale) DateTime(t time.Time) string {
	return t.Format(l.Text(formatDateTime))
}

// Duration formats d as M:SS below one hour and H:MM:SS from one hour. Zero
// and negative durations render as 0:00.
func (l Locale) Duration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int64(d / time.Second)
	hours := total / 3600
	minutes := total / 60 % 60
	seconds := total % 60
	if hours > 0 {
		return fmt.Sprintf(l.Text(formatDurationHours), hours, minutes, seconds)
	}
	return fmt.Sprintf(l.Text(formatDurationMin), minutes, seconds)
}

// magnitude returns the unsigned magnitude of n without overflowing on
// math.MinInt64.
func magnitude(n int64) uint64 {
	if n < 0 {
		return uint64(-(n + 1)) + 1
	}
	return uint64(n)
}

// groupDigits inserts sep between groups of three digits.
func groupDigits(digits, sep string) string {
	if len(digits) <= 3 {
		return digits
	}
	first := len(digits) % 3
	if first == 0 {
		first = 3
	}
	var b strings.Builder
	b.Grow(len(digits) + (len(digits)-1)/3*len(sep))
	b.WriteString(digits[:first])
	for i := first; i < len(digits); i += 3 {
		b.WriteString(sep)
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}
