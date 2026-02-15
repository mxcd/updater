package configuration

import (
	"strconv"
	"strings"
)

// ParseSemver extracts major, minor, and patch version components from a version string.
// It handles common prefixes (v/V) and pre-release suffixes (e.g. "1.2.3-beta1").
func ParseSemver(version string) (major, minor, patch int) {
	versionStr := strings.TrimPrefix(version, "v")
	versionStr = strings.TrimPrefix(versionStr, "V")

	// Split on pre-release separators to get the base version
	baseParts := strings.FieldsFunc(versionStr, func(r rune) bool {
		return r == '-' || r == '_' || r == '+'
	})

	if len(baseParts) == 0 {
		return 0, 0, 0
	}

	parts := strings.Split(baseParts[0], ".")

	if len(parts) >= 1 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		patch, _ = strconv.Atoi(parts[2])
	}

	return major, minor, patch
}
