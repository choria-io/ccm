// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/choria-io/fisk"
)

// RegistrationTTL represents a time-to-live value for registration entries.
// It can be a specific duration (e.g., "10m") or "never" to indicate no expiry.
// The zero value represents an unset TTL; use a nil pointer to omit from serialization.
type RegistrationTTL struct {
	duration time.Duration
	never    bool
}

// NeverExpire returns a RegistrationTTL that represents no expiry
func NeverExpire() *RegistrationTTL {
	return &RegistrationTTL{never: true}
}

// NewRegistrationTTL returns a RegistrationTTL with the given duration
func NewRegistrationTTL(d time.Duration) *RegistrationTTL {
	return &RegistrationTTL{duration: d}
}

// IsNever returns true when the TTL is set to never expire
func (t *RegistrationTTL) IsNever() bool {
	if t == nil {
		return false
	}
	return t.never
}

// Duration returns the duration value. Returns 0 if never or unset.
func (t *RegistrationTTL) Duration() time.Duration {
	if t == nil || t.never {
		return 0
	}
	return t.duration
}

// String returns "never" for never-expire, the duration string for a set duration,
// or an empty string for a zero/unset value
func (t *RegistrationTTL) String() string {
	if t == nil {
		return ""
	}
	if t.never {
		return "never"
	}
	if t.duration > 0 {
		return t.duration.String()
	}
	return ""
}

// MarshalJSON implements json.Marshaler
func (t RegistrationTTL) MarshalJSON() ([]byte, error) {
	if t.never {
		return json.Marshal("never")
	}
	if t.duration > 0 {
		return json.Marshal(t.duration.String())
	}
	return json.Marshal(nil)
}

// UnmarshalJSON implements json.Unmarshaler. Accepts:
//   - string "never" for never-expire
//   - string duration like "10m", "1h" parsed via fisk.ParseDuration
//   - number treated as nanoseconds (backward compatibility with time.Duration JSON)
//   - null for unset
func (t *RegistrationTTL) UnmarshalJSON(data []byte) error {
	var raw json.RawMessage
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
	}

	str := strings.TrimSpace(string(raw))

	// null
	if str == "null" {
		*t = RegistrationTTL{}
		return nil
	}

	// number (backward compat: time.Duration marshals as nanoseconds)
	if len(str) > 0 && (str[0] >= '0' && str[0] <= '9' || str[0] == '-') {
		var ns float64
		err = json.Unmarshal(raw, &ns)
		if err != nil {
			return fmt.Errorf("invalid TTL number: %w", err)
		}
		*t = RegistrationTTL{duration: time.Duration(ns)}
		return nil
	}

	// string
	var s string
	err = json.Unmarshal(raw, &s)
	if err != nil {
		return fmt.Errorf("invalid TTL value: %w", err)
	}

	return t.parseString(s)
}

// MarshalYAML implements the goccy/go-yaml BytesMarshaler interface
func (t RegistrationTTL) MarshalYAML() ([]byte, error) {
	if t.never {
		return []byte("never"), nil
	}
	if t.duration > 0 {
		return []byte(t.duration.String()), nil
	}
	return []byte("null"), nil
}

// UnmarshalYAML implements the goccy/go-yaml BytesUnmarshaler interface
func (t *RegistrationTTL) UnmarshalYAML(data []byte) error {
	s := strings.TrimSpace(string(data))

	if s == "" || s == "null" || s == "~" {
		*t = RegistrationTTL{}
		return nil
	}

	return t.parseString(s)
}

// ParseRegistrationTTL parses a string into a RegistrationTTL.
// Accepts "never" for no expiry or any duration string parseable by fisk.ParseDuration (e.g., "10m", "1h").
func ParseRegistrationTTL(s string) (*RegistrationTTL, error) {
	t := &RegistrationTTL{}
	err := t.parseString(s)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (t *RegistrationTTL) parseString(s string) error {
	if s == "never" {
		*t = RegistrationTTL{never: true}
		return nil
	}

	d, err := fisk.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid TTL duration %q: %w", s, err)
	}

	*t = RegistrationTTL{duration: d}
	return nil
}
