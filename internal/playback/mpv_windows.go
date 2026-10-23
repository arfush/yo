//go:build windows

package playback

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func configureMPVCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
}

func mpvPipePath(name string) string {
	return `\\.\pipe\` + name
}
