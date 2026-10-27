//go:build !windows

package token

// yoanime ships only for Windows; these stubs keep the package buildable and
// its pure parsing tests runnable on other platforms. They deliberately do not
// pretend to encrypt anything. The Windows build uses DPAPI.
func protect(data []byte) ([]byte, error) {
	return append([]byte(nil), data...), nil
}

func unprotect(data []byte) ([]byte, error) {
	return append([]byte(nil), data...), nil
}
