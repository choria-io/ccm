// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// ExecutableInPath finds the command name in path
func ExecutableInPath(file string) (string, bool, error) {
	f, err := exec.LookPath(file)

	return f, err == nil, err
}

// FileExists determines if a file exists regardless of type
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDirectory determines if a path is a directory
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
	result := CloneMap(target)
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
					combined := append(CloneSlice(existingTyped), incomingSlice...)
					result[key] = combined
					continue
				}
			}
		}
		result[key] = CloneValue(value)
	}
	return result
}

// CloneMap creates a shallow copy of the provided map with cloned values.
func CloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = CloneValue(value)
	}
	return result
}

// CloneMapStrings creates a shallow copy of the provided map with cloned values.
func CloneMapStrings(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = CloneValue(value).(string)
	}
	return result
}

// CloneSlice returns a shallow copy of a slice with cloned elements.
func CloneSlice(source []any) []any {
	result := make([]any, len(source))
	for i, value := range source {
		result[i] = CloneValue(value)
	}
	return result
}

// CloneValue duplicates maps and slices to avoid mutating caller state.
func CloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return CloneMap(typed)
	case []any:
		return CloneSlice(typed)
	default:
		return typed
	}
}

// ShallowMerge merges source keys into target without recursion.
func ShallowMerge(target, source map[string]any) map[string]any {
	result := CloneMap(target)
	for key, value := range source {
		result[key] = CloneValue(value)
	}
	return result
}

// IsJsonObject checks if bytes are json maps
func IsJsonObject(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))

	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(string(trimmed), "[")
}

// UntarGz extracts a tar.gz file into a target directory
func UntarGz(s io.Reader, td string) ([]string, error) {
	uncompressed, err := gzip.NewReader(s)
	if err != nil {
		return nil, fmt.Errorf("unzip failed: %s", err)
	}

	var files []string

	tarReader := tar.NewReader(uncompressed)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir {
			return nil, fmt.Errorf("only regular files and directories are supported")
		}

		if strings.Contains(header.Name, "..") {
			return nil, fmt.Errorf("invalid tar file detected")
		}

		path := filepath.Join(td, header.Name)
		if !strings.HasPrefix(path, td) {
			return nil, fmt.Errorf("invalid tar file detected")
		}

		nfo := header.FileInfo()
		if nfo.IsDir() {
			err = os.MkdirAll(path, nfo.Mode())
			if err != nil {
				return nil, err
			}
			continue
		}

		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, nfo.Mode())
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(file, tarReader)
		file.Close()
		if err != nil {
			return nil, err
		}

		files = append(files, path)
	}

	return files, nil
}

// MapStringsToMapStringAny converts a map[string]string to a map[string]any
func MapStringsToMapStringAny(m map[string]string) map[string]any {
	res := make(map[string]any, len(m))
	for k, v := range m {
		res[k] = v
	}

	return res
}

// IsValidResourceRef checks if a resource reference is valid
func IsValidResourceRef(refs ...string) bool {
	for _, ref := range refs {
		if len(strings.SplitN(ref, "#", 2)) != 2 {
			return false
		}
	}

	return true
}

// IsTerminal determines if stdout is a terminal
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
