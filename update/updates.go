// Package updates checks GitHub Releases for new yo builds.
//
// A release is published by the GitHub Actions workflow on a commit whose
// message starts with "release: <version>". The workflow uploads one Windows
// executable per architecture and a matching .sha256 file. The application
// reads the newest release accepted by the selected channel through the public
// GitHub API, picks the asset for the running architecture, and verifies both
// its size and SHA-256 before the updater is allowed to replace the executable.
//
// Stable builds use one or two version components (v1.2). Patch releases add a
// third component (v1.2.3); they are installed only when the user opts into beta
// testing.
package update

import "github.com/arfush/yo/log"

// Repository is the GitHub repository that publishes yo releases. It can
// be overridden at build time:
//
//	go build -ldflags "-X github.com/arfush/yo/update.Repository=owner/repo"
var Repository = "arfush/yo"

// apiBase is a variable so tests can point the client at a local server.
var apiBase = "https://api.github.com"

const (
	userAgent       = "yo"
	maxAPIBytes     = 4 << 20
	maxChecksumSize = 64 << 10
	maxReleaseRaw   = 1 << 30
)

var logger = log.For(log.SourceUpdate)
