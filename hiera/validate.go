// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package hiera

import (
	"errors"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/validator"
)

// ValidationRule represents a parsed annotation from a YAML comment.
type ValidationRule struct {
	Key        string // data key path, e.g. "user" or "web.listen_port"
	Required   bool   // @require or @required was present
	Validation string // expr from @validate, empty if not set
}

// ParseAnnotations scans a yaml.CommentMap for @require and @validate
// directives under the given dataKey prefix. Returns rules keyed by
// data path with the prefix stripped. Logs warnings for unrecognized
// @ directives.
func ParseAnnotations(cm yaml.CommentMap, dataKey string, log model.Logger) []ValidationRule {
	prefix := "$." + dataKey + "."
	ruleMap := map[string]*ValidationRule{}

	for path, comments := range cm {
		if !strings.HasPrefix(path, prefix) {
			continue
		}

		// Skip array element paths
		if strings.Contains(path, "[") {
			continue
		}

		key := strings.TrimPrefix(path, prefix)

		for _, comment := range comments {
			for _, text := range comment.Texts {
				line := strings.TrimSpace(text)
				if line == "" || !strings.HasPrefix(line, "@") {
					continue
				}

				rule, ok := ruleMap[key]
				if !ok {
					rule = &ValidationRule{Key: key}
					ruleMap[key] = rule
				}

				switch {
				case line == "@require" || line == "@required":
					rule.Required = true

				case strings.HasPrefix(line, "@validate "):
					expr := strings.TrimSpace(strings.TrimPrefix(line, "@validate "))
					if expr != "" {
						rule.Validation = expr
					}

				default:
					if log != nil {
						log.Warn("Unrecognized annotation in hiera data", "key", key, "annotation", line)
					}
				}
			}
		}
	}

	rules := make([]ValidationRule, 0, len(ruleMap))
	for _, rule := range ruleMap {
		if rule.Required || rule.Validation != "" {
			rules = append(rules, *rule)
		}
	}

	return rules
}

// ValidateData checks resolved data against parsed rules. Returns a
// multi-error (errors.Join) with all failures, each identifying the
// key and reason.
func ValidateData(data map[string]any, rules []ValidationRule) error {
	var errs []error

	for _, rule := range rules {
		value, found := lookupNestedKey(data, rule.Key)

		if rule.Required {
			if isRequiredZero(value) {
				errs = append(errs, fmt.Errorf("key %q is required but has no value", rule.Key))
				if !found || value == nil {
					continue
				}
			}
		}

		if rule.Validation == "" {
			continue
		}

		if value == nil {
			continue
		}

		switch value.(type) {
		case map[string]any:
			continue
		case []any:
			continue
		}

		ok, err := validator.Validate(value, rule.Validation)
		if err != nil {
			errs = append(errs, fmt.Errorf("key %q validation %q failed: %w", rule.Key, rule.Validation, err))
		} else if !ok {
			errs = append(errs, fmt.Errorf("key %q failed validation %q", rule.Key, rule.Validation))
		}
	}

	return errors.Join(errs...)
}

// lookupNestedKey traverses a nested map structure using a dot-separated key path.
func lookupNestedKey(data map[string]any, key string) (any, bool) {
	parts := strings.Split(key, ".")
	var current any = data

	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}

		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}

	return current, true
}

// isRequiredZero returns true for nil and empty string only.
// false and 0 are considered valid values.
func isRequiredZero(v any) bool {
	if v == nil {
		return true
	}

	s, ok := v.(string)
	if ok && s == "" {
		return true
	}

	return false
}
