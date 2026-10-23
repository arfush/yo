package version

import "strings"

// Version is the application version. It can be overridden at build time:
//
//	go build -ldflags "-X github.com/arfush/yonime/internal/version.Version=0.2"
var Version = "1.1"

// String returns the version formatted for display, with a single leading "v".
func String() string {
	return "v" + strings.TrimPrefix(strings.TrimSpace(Version), "v")
}
