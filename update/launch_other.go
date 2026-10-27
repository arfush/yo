//go:build !windows

package update

import "fmt"

// Launch is not implemented outside Windows. yo's updater replaces a
// Windows executable and the build targets Windows.
func (p *Plan) Launch() error {
	return fmt.Errorf("application updates are supported on Windows only")
}
