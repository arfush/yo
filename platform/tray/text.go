package tray

import "unicode/utf16"

// NOTIFYICONDATAW stores szInfoTitle as 64 UTF-16 units and szInfo as 256.
// Their limits include the terminating NUL.
const (
	MaxNotificationTitle = 63
	MaxNotificationBody  = 255
)

// TrimNotificationText limits a string to the UTF-16 units Windows copies into
// NOTIFYICONDATAW, keeping surrogate pairs intact and ending a shortened value
// with an ellipsis.
func TrimNotificationText(value string, maxUnits int) string {
	if maxUnits <= 0 {
		return ""
	}
	units := utf16.Encode([]rune(value))
	if len(units) <= maxUnits {
		return value
	}
	limit := maxUnits
	if limit > 1 {
		limit--
	}
	units = units[:limit]
	if len(units) > 0 && units[len(units)-1] >= 0xd800 && units[len(units)-1] <= 0xdbff {
		units = units[:len(units)-1]
	}
	return string(utf16.Decode(units)) + "…"
}
