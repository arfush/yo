package appearance_test

import (
	"testing"
	"time"

	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/resource"
)

func TestNumber(t *testing.T) {
	t.Parallel()

	en, ru := testLocales(t)
	tests := []struct {
		name string
		loc  appearance.Locale
		n    int64
		want string
	}{
		{"en zero", en, 0, "0"},
		{"en below group", en, 999, "999"},
		{"en thousands", en, 1234567, "1,234,567"},
		{"en negative", en, -1234567, "-1,234,567"},
		{"en min int64", en, -9223372036854775808, "-9,223,372,036,854,775,808"},
		{"ru thousands", ru, 1234567, "1\u00a0234\u00a0567"},
		{"ru negative", ru, -1234567, "-1\u00a0234\u00a0567"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.loc.Number(tt.n); got != tt.want {
				t.Fatalf("Number(%d) = %q; want %q", tt.n, got, tt.want)
			}
		})
	}
}

func TestDecimal(t *testing.T) {
	t.Parallel()

	en, ru := testLocales(t)
	tests := []struct {
		name   string
		loc    appearance.Locale
		v      float64
		places int
		want   string
	}{
		{"en two places", en, 1234.5, 2, "1,234.50"},
		{"en zero places", en, 1234.56, 0, "1,235"},
		{"en negative", en, -1234.5, 1, "-1,234.5"},
		{"ru two places", ru, 1234.5, 2, "1\u00a0234,50"},
		{"ru negative", ru, -1234.5, 1, "-1\u00a0234,5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.loc.Decimal(tt.v, tt.places); got != tt.want {
				t.Fatalf("Decimal(%v, %d) = %q; want %q", tt.v, tt.places, got, tt.want)
			}
		})
	}
}

func TestPercent(t *testing.T) {
	t.Parallel()

	en, ru := testLocales(t)
	tests := []struct {
		name string
		loc  appearance.Locale
		v    float64
		want string
	}{
		{"en", en, 0.126, "12.6%"},
		{"ru", ru, 0.126, "12,6\u00a0%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.loc.Percent(tt.v, 1); got != tt.want {
				t.Fatalf("Percent(%v, 1) = %q; want %q", tt.v, got, tt.want)
			}
		})
	}
}

func TestDate(t *testing.T) {
	t.Parallel()

	en, ru := testLocales(t)
	at := time.Date(2026, time.January, 2, 15, 4, 5, 0, time.UTC)
	tests := []struct {
		name string
		loc  appearance.Locale
		date string
		dt   string
	}{
		{"en", en, "01/02/2026", "01/02/2026 15:04"},
		{"ru", ru, "02.01.2026", "02.01.2026 15:04"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.loc.Date(at); got != tt.date {
				t.Fatalf("Date() = %q; want %q", got, tt.date)
			}
			if got := tt.loc.DateTime(at); got != tt.dt {
				t.Fatalf("DateTime() = %q; want %q", got, tt.dt)
			}
		})
	}
}

func TestDuration(t *testing.T) {
	t.Parallel()

	en, _ := testLocales(t)
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0:00"},
		{"negative", -5 * time.Second, "0:00"},
		{"seconds", 59 * time.Second, "0:59"},
		{"minute", time.Minute, "1:00"},
		{"minutes", 59*time.Minute + 59*time.Second, "59:59"},
		{"hour", time.Hour, "1:00:00"},
		{"hour minutes seconds", time.Hour + 2*time.Minute + 3*time.Second, "1:02:03"},
		{"many hours", 25*time.Hour + 3*time.Minute + 4*time.Second, "25:03:04"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := en.Duration(tt.d); got != tt.want {
				t.Fatalf("Duration(%v) = %q; want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestNegativeDecimalPlacesPanics(t *testing.T) {
	t.Parallel()

	en, _ := testLocales(t)
	assertPanics(t, func() { en.Decimal(1, -1) })
	assertPanics(t, func() { en.Percent(1, -1) })
}

func testLocales(t *testing.T) (appearance.Locale, appearance.Locale) {
	t.Helper()
	app := appearance.New(appearance.Options{
		Resources: resource.New(),
		Language:  "en",
	})
	en := app.Locale()
	app.SetLanguage("ru")
	return en, app.Locale()
}
