package playback

import (
	"reflect"
	"testing"
)

func TestInputLockArgs(t *testing.T) {
	if got := inputLockArgs(false); got != nil {
		t.Fatalf("inputLockArgs(false) = %v, want no extra arguments", got)
	}
	want := []string{
		"--input-default-bindings=no",
		"--input-vo-keyboard=no",
		"--osc=no",
	}
	if got := inputLockArgs(true); !reflect.DeepEqual(got, want) {
		t.Fatalf("inputLockArgs(true) = %v, want %v", got, want)
	}
}
