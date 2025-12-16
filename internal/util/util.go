// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// ExecutableInPath finds command name in path
func ExecutableInPath(file string) (string, bool, error) {
	f, err := exec.LookPath(file)

	return f, err == nil, err
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func IsDirectory(path string) bool {
	stat, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if stat == nil {
		return false
	}

	return stat.IsDir()
}

// VersionCmp compares two version strings.
// It returns -1 if versionA < versionB, 0 if equal, 1 if versionA > versionB.
// If ignoreTrailingZeroes is true, it normalizes trailing ".0" segments
// in the part before the first "-" (e.g. "1.0.0-rc1" -> "1-rc1").
//
// Logic ported from Puppet source
func VersionCmp(versionA, versionB string, ignoreTrailingZeroes bool) int {
	vre := regexp.MustCompile(`[-.]|\d+|[^-.\d]+`)

	if ignoreTrailingZeroes {
		versionA = normalize(versionA)
		versionB = normalize(versionB)
	}

	ax := vre.FindAllString(versionA, -1)
	bx := vre.FindAllString(versionB, -1)

	for len(ax) > 0 && len(bx) > 0 {
		a := ax[0]
		b := bx[0]
		ax = ax[1:]
		bx = bx[1:]

		if a == b {
			continue
		}
		if a == "-" {
			return -1
		}
		if b == "-" {
			return 1
		}
		if a == "." {
			return -1
		}
		if b == "." {
			return 1
		}

		aIsDigits := isDigits(a)
		bIsDigits := isDigits(b)

		if aIsDigits && bIsDigits {
			// If either starts with 0, compare as strings (lexically)
			if strings.HasPrefix(a, "0") || strings.HasPrefix(b, "0") {
				return cmpStringsCaseInsensitive(a, b)
			}

			ai, _ := strconv.Atoi(a)
			bi, _ := strconv.Atoi(b)
			if ai < bi {
				return -1
			}
			if ai > bi {
				return 1
			}
			return 0
		}

		return cmpStringsCaseInsensitive(a, b)
	}

	// Fallback to whole-string comparison (matching Ruby's version_a <=> version_b)
	return strings.Compare(versionA, versionB)
}

// normalize removes trailing ".0" (and dots) from the part of the version
// before the first "-".
//
// Ruby equivalent:
//
//	version = version.split('-')
//	version.first.sub!(/([.0]+)$/, '')
//	version.join('-')
func normalize(version string) string {
	parts := strings.Split(version, "-")
	if len(parts) == 0 {
		return version
	}

	re := regexp.MustCompile(`([.0]+)$`)
	parts[0] = re.ReplaceAllString(parts[0], "")
	return strings.Join(parts, "-")
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func cmpStringsCaseInsensitive(a, b string) int {
	au := strings.ToUpper(a)
	bu := strings.ToUpper(b)
	if au < bu {
		return -1
	}
	if au > bu {
		return 1
	}
	return 0
}

// DeepMergeMap merges source maps into target recursively. Map values are merged, slices are concatenated, and other values override.
func DeepMergeMap(target map[string]any, source map[string]any) map[string]any {
	result := cloneMap(target)
	for key, value := range source {
		if existing, ok := result[key]; ok {
			switch existingTyped := existing.(type) {
			case map[string]any:
				if incomingMap, ok := value.(map[string]any); ok {
					result[key] = DeepMergeMap(existingTyped, incomingMap)
					continue
				}
			case []any:
				if incomingSlice, ok := value.([]any); ok {
					combined := append(cloneSlice(existingTyped), incomingSlice...)
					result[key] = combined
					continue
				}
			}
		}
		result[key] = cloneValue(value)
	}
	return result
}

// cloneMap creates a shallow copy of the provided map with cloned values.
func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = cloneValue(value)
	}
	return result
}

// cloneSlice returns a shallow copy of a slice with cloned elements.
func cloneSlice(source []any) []any {
	result := make([]any, len(source))
	for i, value := range source {
		result[i] = cloneValue(value)
	}
	return result
}

// cloneValue duplicates maps and slices to avoid mutating caller state.
func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		return cloneSlice(typed)
	default:
		return typed
	}
}
