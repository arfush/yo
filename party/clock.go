package party

import (
	"sync"
	"time"
)

const (
	// clockSamples caps how many anchor arrivals the offset estimate keeps.
	clockSamples = 96
	// clockSampleTTL expires an arrival no matter how few samples there are.
	// A long pause publishes only every ten seconds, so a count-only window
	// would hold estimates from a quarter of an hour ago while the host may
	// have changed its clock or restarted.
	clockSampleTTL = 5 * time.Minute
)

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
	mu      sync.Mutex
	samples []clockSample
}

type clockSample struct {
	delta int64
	// at is the local receipt time in unix milliseconds. Wall clock is enough
	// here because a sample that is five minutes older than its neighbours is
	// dropped whether or not the clock jumped.
	at int64
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
	c.prune(localMS)
	c.samples = append(c.samples, clockSample{delta: delta, at: localMS})
	if len(c.samples) > clockSamples {
		c.samples = append(c.samples[:0], c.samples[len(c.samples)-clockSamples:]...)
	}
}

// prune drops samples older than the TTL relative to the newest arrival. The
// caller must hold the mutex.
func (c *Clock) prune(localMS int64) {
	cutoff := localMS - clockSampleTTL.Milliseconds()
	kept := c.samples[:0]
	for _, sample := range c.samples {
		if sample.at >= cutoff {
			kept = append(kept, sample)
		}
	}
	c.samples = kept
}

// Offset returns host clock minus local clock. The boolean is false until the
// first usable sample arrives.
func (c *Clock) Offset() (time.Duration, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.samples) == 0 {
		return 0, false
	}
	min := c.samples[0].delta
	for _, sample := range c.samples[1:] {
		if sample.delta < min {
			min = sample.delta
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

// Reset drops every sample, used when the anchor source changes: after a
// reconnect, or when a host that had gone quiet starts answering again from a
// machine with another clock.
func (c *Clock) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.samples = nil
}
