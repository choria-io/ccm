// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package apt

// ported from Puppet::Util::Package::Version::Debian fc39fe4c705a2511c511800240fcbe7196e08140

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	regexEpoch           = `(?:([0-9]+):)?`
	regexUpstreamVersion = `([\.\+~0-9a-zA-Z-]+?)`
	regexDebianRevision  = `(?:-([\.\+~0-9a-zA-Z]*))?`
)

var (
	versionFullRx = regexp.MustCompile(`\A` + regexEpoch + regexUpstreamVersion + regexDebianRevision + `\z`)
	digitsRx      = regexp.MustCompile(`^([0-9]+)`)
	nonLettersRx  = regexp.MustCompile(`^([.+-]+)`)
	tildesRx      = regexp.MustCompile(`^(~+)`)
	lettersRx     = regexp.MustCompile(`^([A-Za-z]+)`)
)

// Version represents a parsed Debian package version
type Version struct {
	Epoch           int
	UpstreamVersion string
	DebianRevision  string
}

// ParseVersion parses a Debian version string into its components
func ParseVersion(ver string) (*Version, error) {
	if ver == "" {
		return nil, fmt.Errorf("unable to parse empty string as a debian version identifier")
	}

	matches := versionFullRx.FindStringSubmatch(ver)
	if matches == nil {
		return nil, fmt.Errorf("unable to parse %q as a debian version identifier", ver)
	}

	epoch := 0
	if matches[1] != "" {
		var err error
		epoch, err = strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("unable to parse epoch %q: %w", matches[1], err)
		}
	}

	return &Version{
		Epoch:           epoch,
		UpstreamVersion: matches[2],
		DebianRevision:  matches[3],
	}, nil
}

// String returns the version as a string
func (v *Version) String() string {
	s := v.UpstreamVersion
	if v.Epoch != 0 {
		s = fmt.Sprintf("%d:%s", v.Epoch, s)
	}
	if v.DebianRevision != "" {
		s = fmt.Sprintf("%s-%s", s, v.DebianRevision)
	}
	return s
}

// Compare compares two versions and returns:
// -1 if v < other
//
//	0 if v == other
//	1 if v > other
func (v *Version) Compare(other *Version) int {
	if other == nil {
		return 1
	}

	// Compare epochs first
	cmp := compareInt(v.Epoch, other.Epoch)
	if cmp != 0 {
		return cmp
	}

	// Compare upstream versions
	cmp = compareDebianVersions(v.UpstreamVersion, other.UpstreamVersion)
	if cmp != 0 {
		return cmp
	}

	// Compare debian revisions
	return compareDebianVersions(v.DebianRevision, other.DebianRevision)
}

// Equal returns true if two versions are equal
func (v *Version) Equal(other *Version) bool {
	if other == nil {
		return false
	}
	return v.Epoch == other.Epoch &&
		v.UpstreamVersion == other.UpstreamVersion &&
		v.DebianRevision == other.DebianRevision
}

// LessThan returns true if v < other
func (v *Version) LessThan(other *Version) bool {
	return v.Compare(other) < 0
}

// GreaterThan returns true if v > other
func (v *Version) GreaterThan(other *Version) bool {
	return v.Compare(other) > 0
}

// LessThanOrEqual returns true if v <= other
func (v *Version) LessThanOrEqual(other *Version) bool {
	return v.Compare(other) <= 0
}

// GreaterThanOrEqual returns true if v >= other
func (v *Version) GreaterThanOrEqual(other *Version) bool {
	return v.Compare(other) >= 0
}

// CompareVersionStrings compares two version strings directly
// Returns -1 if a < b, 0 if a == b, 1 if a > b
// Returns an error if either version string is invalid
func CompareVersionStrings(a, b string) (int, error) {
	va, err := ParseVersion(a)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", a, err)
	}

	vb, err := ParseVersion(b)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", b, err)
	}

	return va.Compare(vb), nil
}

func compareInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// compareDebianVersions implements the Debian version comparison algorithm.
//
// First the initial part of each string consisting entirely of non-digit characters is determined.
// These two parts (one of which may be empty) are compared lexically. If a difference is found it is
// returned. The lexical comparison is a comparison of ASCII values modified so that all the letters
// sort earlier than all the non-letters and so that a tilde sorts before anything, even the end of a
// part. For example, the following parts are in sorted order from earliest to latest: ~~, ~~a, ~, the
// empty part, a.
//
// Then the initial part of the remainder of each string which consists entirely of digit characters
// is determined. The numerical values of these two parts are compared, and any difference found is
// returned as the result of the comparison. For these purposes an empty string (which can only occur
// at the end of one or both version strings being compared) counts as zero.
//
// These two steps (comparing and removing initial non-digit strings and initial digit strings) are
// repeated until a difference is found or both strings are exhausted.
func compareDebianVersions(mine, yours string) int {
	mineIdx := 0
	yoursIdx := 0
	cmp := 0

	for mineIdx < len(mine) && yoursIdx < len(yours) && cmp == 0 {
		// Handle tildes
		myTildes := matchTildes(mine[mineIdx:])
		yoursTildes := matchTildes(yours[yoursIdx:])

		// More tildes means earlier (lower) version
		cmp = -1 * compareInt(len(myTildes), len(yoursTildes))
		mineIdx += len(myTildes)
		yoursIdx += len(yoursTildes)

		if cmp != 0 {
			continue
		}

		// Handle letters
		myLetters := matchLetters(mine[mineIdx:])
		yoursLetters := matchLetters(yours[yoursIdx:])

		cmp = strings.Compare(myLetters, yoursLetters)
		mineIdx += len(myLetters)
		yoursIdx += len(yoursLetters)

		if cmp != 0 {
			continue
		}

		// Handle non-letters (except tilde)
		myNonLetters := matchNonLetters(mine[mineIdx:])
		yoursNonLetters := matchNonLetters(yours[yoursIdx:])

		cmp = strings.Compare(myNonLetters, yoursNonLetters)
		mineIdx += len(myNonLetters)
		yoursIdx += len(yoursNonLetters)

		if cmp != 0 {
			continue
		}

		// Handle digits
		myDigits := matchDigits(mine[mineIdx:])
		yoursDigits := matchDigits(yours[yoursIdx:])

		myNum := 0
		if myDigits != "" {
			myNum, _ = strconv.Atoi(myDigits)
		}
		yoursNum := 0
		if yoursDigits != "" {
			yoursNum, _ = strconv.Atoi(yoursDigits)
		}

		cmp = compareInt(myNum, yoursNum)
		mineIdx += len(myDigits)
		yoursIdx += len(yoursDigits)
	}

	if cmp == 0 {
		// Check for trailing tildes
		if mineIdx < len(mine) && matchTildes(mine[mineIdx:]) != "" {
			cmp = -1
		} else if yoursIdx < len(yours) && matchTildes(yours[yoursIdx:]) != "" {
			cmp = 1
		} else {
			cmp = compareInt(len(mine), len(yours))
		}
	}

	return cmp
}

func matchDigits(s string) string {
	match := digitsRx.FindString(s)
	return match
}

func matchNonLetters(s string) string {
	match := nonLettersRx.FindString(s)
	return match
}

func matchTildes(s string) string {
	match := tildesRx.FindString(s)
	return match
}

func matchLetters(s string) string {
	match := lettersRx.FindString(s)
	return match
}
