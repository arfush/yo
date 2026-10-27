// Package install keeps yoanime's Windows uninstall entry in sync with the
// executable that is actually running. The application updates itself in
// place, so Inno Setup's DisplayVersion would otherwise stay at the version
// that was installed while Windows Apps showed an outdated one.
package install

import "strings"

// normalizeVersion matches the installer's AppVersion, which has no leading
// "v" even though the UI displays one.
func normalizeVersion(raw string) string {
	return strings.TrimPrefix(strings.TrimSpace(raw), "v")
}
