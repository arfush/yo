package update

import (
	"net/url"
	"strings"
)

// Release is one published update. URL is a GitHub release asset URL, SHA256 is
// the lowercase hexadecimal digest of the executable, and AssetName is the
// basename stored next to the downloaded file.
type Release struct {
	Version   string `json:"version"`
	Size      int64  `json:"size"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	AssetName string `json:"asset,omitempty"`
	Notes     string `json:"notes,omitempty"`
}

// Valid reports whether the release has all fields needed for a safe download.
func (r Release) Valid() bool {
	if NormalizeVersion(r.Version) != r.Version {
		return false
	}
	if r.Size <= 0 || r.Size > maxReleaseRaw {
		return false
	}
	if !validSHA256(r.SHA256) {
		return false
	}
	return validReleaseURL(r.URL)
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

// AssetName returns the release asset name for a version and GOARCH value.
func AssetName(version, arch string) string {
	return "yoanime_" + NormalizeVersion(version) + "_" + arch + ".exe"
}

func validReleaseURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	repository := strings.Trim(Repository, "/")
	if repository == "" || !strings.Contains(repository, "/") {
		return false
	}
	return parsed.Scheme == "https" &&
		strings.EqualFold(parsed.Hostname(), "github.com") &&
		parsed.User == nil &&
		parsed.Fragment == "" &&
		strings.HasPrefix(parsed.Path, "/"+repository+"/releases/download/") &&
		parsed.Path != "/"+repository+"/releases/download/"
}
