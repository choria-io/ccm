// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package templates

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/tidwall/gjson"
)

// Env represents the template execution environment containing facts and data
type Env struct {
	Facts   map[string]any    `json:"facts" yaml:"facts"`
	Data    map[string]any    `json:"data" yaml:"data"`
	Environ map[string]string `json:"environ" yaml:"environ"`

	envJSON json.RawMessage
	mu      sync.Mutex
}

func (e *Env) lookup(params ...any) (any, error) {
	var key string
	var defaultValue any
	var ok bool

	if len(params) == 0 && len(params) > 2 {
		return nil, fmt.Errorf("lookup requires 1 or 2 arguments")
	}

	key, ok = params[0].(string)
	if !ok {
		return nil, fmt.Errorf("lookup requires a string argument")
	}

	if len(params) == 2 {
		defaultValue = params[1]
	} else {
		defaultValue = ""
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.envJSON == nil {
		j, err := json.Marshal(e)
		if err != nil {
			return "", err
		}
		e.envJSON = j
	}

	res := gjson.GetBytes(e.envJSON, key)
	if !res.Exists() {
		return defaultValue, nil
	}

	if res.Type == gjson.Number {
		if strings.Contains(res.Raw, ".") {
			return res.Float(), nil
		}

		return res.Int(), nil
	}

	return res.Value(), nil
}

// ResolveTemplateString resolves {{ expression }} placeholders in a template string and returns the result as a string
func ResolveTemplateString(template string, env *Env) (string, error) {
	if template == "" {
		return "", nil
	}

	re := regexp.MustCompile(`{{\s*(.*?)\s*}}`)

	matches := re.FindAllStringSubmatch(template, -1)
	switch {
	case matches == nil:
		return template, nil
	default:
		res, _, err := applyFactsString(template, env)
		return res, err
	}
}

// ResolveTemplateTyped resolves {{ expression }} placeholders and preserves the type of single expressions
func ResolveTemplateTyped(template string, env *Env) (any, error) {
	if template == "" {
		return "", nil
	}

	re := regexp.MustCompile(`{{\s*(.*?)\s*}}`)
	trimmed := strings.TrimSpace(template)

	matches := re.FindAllStringSubmatch(template, -1)
	switch {
	case matches == nil:
		return template, nil
	case len(matches) == 1 && strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}"):
		return exprParse(matches[0][1], env)
	default:
		res, _, err := applyFactsString(template, env)
		return res, err
	}
}

// applyFactsString parses {{ expression}} placeholders using expr and replace them with the resulting values
func applyFactsString(template string, env *Env) (string, bool, error) {
	// Matches: {{ something }}
	// Capture group 1 = inner text
	re := regexp.MustCompile(`{{\s*(.*?)\s*}}`)

	out := template

	matches := re.FindAllStringSubmatchIndex(template, -1)
	if matches == nil {
		// nothing to replace, so we report that we matched because this string should be used for those who care about matching
		return template, template != "", nil
	}

	// We will build the output incrementally
	var result strings.Builder
	lastIndex := 0
	var matched []bool

	for _, loc := range matches {
		fullStart, fullEnd := loc[0], loc[1]
		innerStart, innerEnd := loc[2], loc[3]

		innerExpr := template[innerStart:innerEnd]

		value, err := exprParse(innerExpr, env)
		if err != nil {
			return "", false, err
		}

		switch value.(type) {
		case string:
			if value == "" {
				matched = append(matched, false)
			} else {
				matched = append(matched, true)
			}
		case nil:
			matched = append(matched, false)
		default:
			matched = append(matched, true)
		}

		// Write everything before this match
		result.WriteString(out[lastIndex:fullStart])
		// Now the match
		result.WriteString(fmt.Sprint(value))

		lastIndex = fullEnd
	}

	// Append any remainder after last match
	result.WriteString(out[lastIndex:])

	return result.String(), slices.Contains(matched, true), nil
}

func exprParse(query string, env *Env) (any, error) {
	program, err := expr.Compile(query, expr.Env(env), expr.Function("lookup", env.lookup))
	if err != nil {
		return "", fmt.Errorf("expr compile error for '%s': %w", query, err)
	}

	return expr.Run(program, env)
}
