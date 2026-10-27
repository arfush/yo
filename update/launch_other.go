//go:build !windows

package update

import "fmt"

// Launch is not implemented outside Windows. yoanime's updater replaces a
// Windows executable and the build targets Windows.
func (p *Plan) Launch() error {
	return fmt.Errorf("обновление приложения поддерживается только в Windows")
}
