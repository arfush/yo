//go:build !windows

package install

// SyncDisplayVersion is a no-op outside Windows, where there is no uninstall
// registry entry to update.
func SyncDisplayVersion(string) {}
