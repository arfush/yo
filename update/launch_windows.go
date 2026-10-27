//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

const createNoWindow = 0x08000000

// Launch starts the hidden helper. It returns as soon as cmd.exe has been
// created; the helper keeps retrying until the running application exits and
// the executable can be replaced.
func (p *Plan) Launch() error {
	if p == nil {
		return fmt.Errorf("update plan is nil")
	}
	if err := os.WriteFile(p.Script, []byte(helperScript()), 0o644); err != nil {
		return fmt.Errorf("не удалось создать помощник обновления: %w", err)
	}

	// "call" keeps the script path as a separate argument, so spaces and
	// non-Latin characters in %APPDATA% do not need manual quoting.
	command := exec.Command(
		"cmd.exe",
		"/d",
		"/s",
		"/c",
		"call",
		p.Script,
		p.Target,
		p.Source,
		p.Log,
	)
	command.Dir = p.WorkDir
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | createNoWindow,
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("не удалось запустить помощник обновления: %w", err)
	}
	_ = command.Process.Release()
	return nil
}
