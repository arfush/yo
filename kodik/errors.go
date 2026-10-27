package kodik

import "errors"

// Sentinels returned by the Kodik resolver. Messages are technical English;
// the UI maps them to Russian sentences.
var (
	// ErrEmptyLink means the embed URL was empty.
	ErrEmptyLink = errors.New("empty Kodik link")
	// ErrNoVideoLinks means the resolved page had no <video> sources.
	ErrNoVideoLinks = errors.New("Kodik returned no video links")
	// ErrEmptyLinks means the m3u8 fetch produced an empty list.
	ErrEmptyLinks = errors.New("Kodik returned an empty link list")
	// ErrUnknownFormat means the quality/format object was not understood.
	ErrUnknownFormat = errors.New("Kodik returned links in an unknown format")
	// ErrEmptySource means the link entry carried no URL.
	ErrEmptySource = errors.New("empty link")
	// ErrCannotDecodeLink means the encrypted page link could not be
	// decrypted.
	ErrCannotDecodeLink = errors.New("cannot decode the Kodik link")
	// ErrBrowserClosed means the shared browser exited mid-resolve.
	ErrBrowserClosed = errors.New("Kodik browser is closed")
	// ErrBrowserUnavailable means the browser is busy or unresponsive.
	ErrBrowserUnavailable = errors.New("Kodik browser is unavailable")
)
