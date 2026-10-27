package update

import "errors"

// Sentinel errors returned by the release checker. The UI compares against
// them to decide whether an update card should be shown or an error reported.
var (
	// ErrNoRelease means GitHub is reachable but has no usable release for
	// this architecture (nothing has been published yet, or the latest
	// release has no matching asset).
	ErrNoRelease = errors.New("no yo release published")
	// ErrNotConfigured means Repository is empty or malformed.
	ErrNotConfigured = errors.New("update repository is not configured")
	// ErrBadAsset is returned when a release asset fails the size or SHA-256
	// verification.
	ErrBadAsset = errors.New("update asset failed verification")
)
