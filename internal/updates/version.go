package updates

import (
	"strconv"
	"strings"
)

// parseVersion parses versions such as v1, v1.3, v1.3.0 or v1.3.0.1.
func parseVersion(value string) ([]int64, bool) {
	text := strings.TrimSpace(value)
	if text == "" {
		return nil, false
	}
	if text[0] == 'v' || text[0] == 'V' {
		text = text[1:]
	}
	if text == "" {
		return nil, false
	}
	parts := strings.Split(text, ".")
	if len(parts) == 0 || len(parts) > 4 {
		return nil, false
	}
	numbers := make([]int64, len(parts))
	for index, part := range parts {
		if part == "" || len(part) > 9 {
			return nil, false
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return nil, false
			}
		}
		number, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, false
		}
		numbers[index] = number
	}
	return numbers, true
}

// CompareVersions compares two Yonime versions. It returns -1 when left is
// older, 0 when the versions are equal and 1 when left is newer. Missing
// components count as zero, so v1.3 and v1.3.0 are equal. Invalid versions
// compare as equal, which makes callers treat them as "not newer".
func CompareVersions(left, right string) int {
	a, ok := parseVersion(left)
	if !ok {
		return 0
	}
	b, ok := parseVersion(right)
	if !ok {
		return 0
	}
	length := max(len(a), len(b))
	for index := 0; index < length; index++ {
		var av, bv int64
		if index < len(a) {
			av = a[index]
		}
		if index < len(b) {
			bv = b[index]
		}
		switch {
		case av < bv:
			return -1
		case av > bv:
			return 1
		}
	}
	return 0
}

// IsNewer reports whether candidate is strictly newer than current.
func IsNewer(candidate, current string) bool {
	return CompareVersions(candidate, current) > 0
}

// NormalizeVersion returns a canonical v-prefixed version, or "" when the
// value is not a valid version.
func NormalizeVersion(value string) string {
	parts, ok := parseVersion(value)
	if !ok {
		return ""
	}
	text := make([]string, len(parts))
	for index, part := range parts {
		text[index] = strconv.FormatInt(part, 10)
	}
	return "v" + strings.Join(text, ".")
}
