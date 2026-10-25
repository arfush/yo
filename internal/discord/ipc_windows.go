//go:build windows

package discord

import (
	"fmt"
	"time"

	"gopkg.in/natefinch/npipe.v2"
)

// ipcSlots is how many pipe names Discord may listen on. The client normally
// takes the first free slot, but a second running client (for example a test
// build) can occupy it.
const ipcSlots = 10

// dialIPC connects to the running Discord client. A short timeout matters: a
// missing Discord must fall through all slots quickly and be retried later
// instead of stalling the worker.
func dialIPC(appID string) (*ipcClient, error) {
	var lastErr error
	for slot := 0; slot < ipcSlots; slot++ {
		address := fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, slot)
		conn, err := npipe.DialTimeout(address, 300*time.Millisecond)
		if err == nil {
			return newClient(conn, appID)
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no pipe found")
	}
	return nil, fmt.Errorf("discord ipc: %w", lastErr)
}
