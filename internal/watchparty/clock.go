package watchparty

import (
	"sync"
	"time"
)

// clockSamples is how many anchor arrivals the offset estimate keeps. At one
// anchor every few seconds this covers several minutes, which is long enough to
// average out queueing spikes and short enough to follow clock drift.
const clockSamples = 96

// Clock estimates the offset between the host wall clock and the local one.
//
// Every host anchor carries the host timestamp at which it was sampled. The
// difference between the local arrival time and that timestamp is the offset
// plus the network delay, so the smallest difference in the window is the
// tightest estimate. This needs no extra traffic: the anchors are already
// there for playback.
//
// The session records samples on the socket reader goroutine while the playback
// loop reads the estimate, so the window is guarded by a mutex.
type Clock struct {
	mu     sync.Mutex
	deltas []int64
}

// Add records one anchor arrival. anchorMS is the host clock, localMS is the
// local wall clock at receipt.
func (c *Clock) Add(anchorMS, localMS int64) {
	if anchorMS <= 0 || localMS <= 0 {
		return
	}
	delta := localMS - anchorMS
	// A wildly negative delta means the host clock jumped backwards or the
	// anchor is stale; keeping it would poison the minimum filter.
	if delta < -24*60*60*1000 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deltas = append(c.deltas, delta)
	if len(c.deltas) > clockSamples {
		c.deltas = append(c.deltas[:0], c.deltas[len(c.deltas)-clockSamples:]...)
	}
}

// Offset returns host clock minus local clock. The boolean is false until the
// first usable sample arrives.
func (c *Clock) Offset() (time.Duration, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.deltas) == 0 {
		return 0, false
	}
	min := c.deltas[0]
	for _, delta := range c.deltas[1:] {
		if delta < min {
			min = delta
		}
	}
	// local = host - offset + delay, so delta = -offset + delay. The minimum
	// delay observed gives the tightest upper bound on the offset.
	return -time.Duration(min) * time.Millisecond, true
}

// HostNow converts a local wall clock instant to the host clock.
func (c *Clock) HostNow(local time.Time) time.Time {
	offset, ok := c.Offset()
	if !ok {
		return local
	}
	return local.Add(offset)
}

// Reset drops every sample, used after a reconnect when the host may have moved
// to another machine.
func (c *Clock) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deltas = nil
}
