//go:build windows

package token

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// tokenEntropy binds the DPAPI blob to yoanime. DPAPI already ties it to the
// current Windows user; the extra entropy prevents another application running
// as the same user from decrypting the file just by calling CryptUnprotectData.
var tokenEntropy = []byte("yoanime/libsocial-token/v1")

// protect encrypts the token payload with user-scoped Windows DPAPI. The
// ciphertext can only be opened by the same Windows user on the same machine.
func protect(data []byte) ([]byte, error) {
	return cryptProtect(data, true)
}

// unprotect reverses protect. A failure means the file belongs to another
// Windows user, was copied from another machine, or was tampered with.
func unprotect(data []byte) ([]byte, error) {
	return cryptProtect(data, false)
}

func cryptProtect(data []byte, encrypt bool) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrEmptyPayload
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	entropy := windows.DataBlob{Size: uint32(len(tokenEntropy)), Data: &tokenEntropy[0]}
	var out windows.DataBlob
	var description *uint16
	var err error
	if encrypt {
		err = windows.CryptProtectData(
			&in,
			nil,
			&entropy,
			0,
			nil,
			windows.CRYPTPROTECT_UI_FORBIDDEN,
			&out,
		)
	} else {
		err = windows.CryptUnprotectData(
			&in,
			&description,
			&entropy,
			0,
			nil,
			windows.CRYPTPROTECT_UI_FORBIDDEN,
			&out,
		)
	}
	if description != nil {
		_, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(description)))
	}
	if err != nil {
		return nil, fmt.Errorf("DPAPI: %w", err)
	}
	if out.Data == nil || out.Size == 0 {
		return nil, ErrEmptyDPAPIResult
	}
	defer func() {
		_, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	}()
	result := make([]byte, out.Size)
	copy(result, unsafe.Slice(out.Data, out.Size))
	return result, nil
}
