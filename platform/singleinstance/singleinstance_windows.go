//go:build windows

package singleinstance

import (
	"errors"
	"sync"

	"golang.org/x/sys/windows"
)

// The Local\ prefix scopes the mutex to the current user session: two users
// logged into the same machine may each run their own copy.
var mutexName = `Local\yo.SingleInstance`

func acquire() (func(), bool) {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		// Never block startup because the guard itself failed.
		return func() {}, true
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if handle != 0 {
			_ = windows.CloseHandle(handle)
		}
		return nil, false
	}
	if err != nil {
		return func() {}, true
	}

	var once sync.Once
	return func() {
		once.Do(func() { _ = windows.CloseHandle(handle) })
	}, true
}
