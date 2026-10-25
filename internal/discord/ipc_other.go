//go:build !windows

package discord

import "errors"

// dialIPC is unavailable outside Windows: the project only ships a Windows
// build, and the stub keeps the package compiling everywhere else.
func dialIPC(string) (*ipcClient, error) {
	return nil, errors.New("discord ipc is only available on Windows")
}
