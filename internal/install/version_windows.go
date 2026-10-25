//go:build windows

package install

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const uninstallRoot = `Software\Microsoft\Windows\CurrentVersion\Uninstall`

// SyncDisplayVersion updates Add/Remove Programs after a self-update. It only
// touches an entry whose InstallLocation is the directory of the running
// executable, so a portable copy or a different user's install is never
// modified by accident.
func SyncDisplayVersion(raw string) {
	version := normalizeVersion(raw)
	if version == "" {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		return
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return
	}
	installDir := filepath.Dir(executable)

	root, err := registry.OpenKey(registry.CURRENT_USER, uninstallRoot, registry.READ)
	if err != nil {
		return
	}
	names, err := root.ReadSubKeyNames(-1)
	root.Close()
	if err != nil {
		return
	}
	for _, name := range names {
		path := uninstallRoot + `\` + name
		key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.READ)
		if err != nil {
			continue
		}
		location, _, _ := key.GetStringValue("InstallLocation")
		icon, _, _ := key.GetStringValue("DisplayIcon")
		current, _, _ := key.GetStringValue("DisplayVersion")
		key.Close()
		if !ownsExecutable(location, icon, installDir) || current == version {
			continue
		}
		write, err := registry.OpenKey(registry.CURRENT_USER, path, registry.SET_VALUE)
		if err != nil {
			continue
		}
		_ = write.SetStringValue("DisplayVersion", version)
		write.Close()
	}
}

// ownsExecutable accepts the Inno Setup location or its icon path, which both
// point at the installed executable. The ",0" icon index is stripped.
func ownsExecutable(location, icon, installDir string) bool {
	if sameDir(location, installDir) {
		return true
	}
	icon = strings.TrimSpace(icon)
	if comma := strings.LastIndex(icon, ","); comma >= 0 {
		icon = icon[:comma]
	}
	if strings.TrimSpace(icon) == "" {
		return false
	}
	return sameDir(filepath.Dir(icon), installDir)
}

func sameDir(value, installDir string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return false
	}
	return strings.EqualFold(filepath.Clean(absolute), filepath.Clean(installDir))
}
