package update

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strings"
)

// FileSHA256 returns the lowercase hexadecimal SHA-256 of path.
func FileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// IsUpdate reports whether release should replace the running build.
//
// A strictly newer version always qualifies. The same version qualifies only
// when the published SHA-256 differs from the running executable, which lets a
// fixed build be re-published under the same version. Older versions never
// qualify, so a release cannot downgrade an installation.
func IsUpdate(release *Release, currentVersion, currentSHA string) bool {
	if release == nil {
		return false
	}
	switch CompareVersions(release.Version, currentVersion) {
	case 1:
		return true
	case -1:
		return false
	}
	if currentSHA == "" {
		return false
	}
	return !strings.EqualFold(release.SHA256, currentSHA)
}
