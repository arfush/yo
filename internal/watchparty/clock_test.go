package watchparty

import (
	"testing"
	"time"
)

func TestClockKeepsTheTightestDelay(t *testing.T) {
	var clock Clock
	if _, ok := clock.Offset(); ok {
		t.Fatal("an empty clock must not report an offset")
	}

	hostMS := time.Unix(1_700_000_000, 0).UnixMilli()

	// The first anchor arrives after 180ms of network delay, the second after
	// 60ms. The smaller delay is the better estimate of the clock offset.
	clock.Add(hostMS, hostMS+180)
	clock.Add(hostMS+4000, hostMS+4060)

	offset, ok := clock.Offset()
	if !ok {
		t.Fatal("the clock has samples but reports no offset")
	}
	if want := -60 * time.Millisecond; offset != want {
		t.Fatalf("offset = %v, want %v", offset, want)
	}
}

func TestClockIgnoresGarbageAnchors(t *testing.T) {
	var clock Clock
	hostMS := time.Unix(1_700_000_000, 0).UnixMilli()

	clock.Add(0, hostMS)
	clock.Add(-1, hostMS)
	clock.Add(hostMS, 0)
	clock.Add(hostMS, hostMS-48*60*60*1000)
	if _, ok := clock.Offset(); ok {
		t.Fatal("garbage anchors must not produce an offset")
	}
}

func TestClockResetDropsSamples(t *testing.T) {
	var clock Clock
	hostMS := time.Unix(1_700_000_000, 0).UnixMilli()
	clock.Add(hostMS, hostMS+50)
	clock.Reset()
	if _, ok := clock.Offset(); ok {
		t.Fatal("Reset must drop every sample")
	}
}
