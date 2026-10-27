//go:build !windows

package singleinstance

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/arfush/yoanime/platform/appdata"
)

var lockName = "yoanime.lock"

func acquire() (func(), bool) {
	dir, err := appdata.Dir()
	if err != nil {
		// Never block startup because the guard itself failed.
		return func() {}, true
	}
	file, err := os.OpenFile(filepath.Join(dir, lockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return func() {}, true
	}
	// The kernel drops the lock when the process dies, so the lock file itself
	// may stay on disk between runs.
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, false
		}
		return func() {}, true
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
			_ = file.Close()
		})
	}, true
}
