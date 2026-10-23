//go:build windows

package browser

import (
	"os/exec"
	"strconv"
	"syscall"
)

// killProcessTree terminates the Playwright CLI and the downloader child it
// spawns. Killing only node.exe can leave the out-of-process download running.
func killProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := kill.Run(); err != nil {
		_ = cmd.Process.Kill()
	}
}
