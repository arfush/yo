// Package singleinstance makes sure that only one copy of Yonime runs for a
// user at a time, so a second launch cannot open a second window or compete for
// the same database and browser profile.
//
// The lock lives in the operating system and is released when the process
// exits, so a crash never leaves a stale guard behind.
package singleinstance

// Acquire takes the per-user application lock. It returns ok=false when
// another instance already owns it. release must be called before the process
// exits and is safe to call more than once.
func Acquire() (release func(), ok bool) {
	return acquire()
}
