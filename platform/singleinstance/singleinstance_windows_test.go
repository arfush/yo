//go:build windows

package singleinstance

import (
	"fmt"
	"os"
	"testing"
)

func TestAcquireIsExclusive(t *testing.T) {
	original := mutexName
	mutexName = fmt.Sprintf(`Local\yo.SingleInstance.Test.%d`, os.Getpid())
	t.Cleanup(func() { mutexName = original })

	release, ok := Acquire()
	if !ok {
		t.Fatal("first Acquire failed")
	}
	secondRelease, secondOK := Acquire()
	if secondOK {
		secondRelease()
		release()
		t.Fatal("second Acquire succeeded while the first lock was held")
	}

	// Releasing twice must be harmless.
	release()
	release()

	thirdRelease, thirdOK := Acquire()
	if !thirdOK {
		t.Fatal("Acquire failed after the lock was released")
	}
	thirdRelease()
}
