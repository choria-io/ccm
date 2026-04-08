// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package templates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
)

// templateMatch represents a found template expression with its position in the source string
type templateMatch struct {
	fullStart, fullEnd   int // bounds of entire {{ expr }} or ${ expr } including delimiters
	innerStart, innerEnd int // bounds of expression text (whitespace-trimmed)
}

// delimKind distinguishes the two template syntaxes
type delimKind int

const (
	delimDoubleBrace delimKind = iota // {{ expr }}
	delimDollarBrace                  // ${ expr }
)

// findTemplateExpressions scans a string for {{ expr }} and ${ expr } template expressions
// using a state-machine lexer that correctly handles closing delimiters inside quoted strings.
// For {{ }} expressions, the closer is }}. For ${ } expressions, brace depth is tracked so
// that nested braces (e.g., in expr-lang lambdas) are handled correctly.
func findTemplateExpressions(s string) []templateMatch {
	var matches []templateMatch
	n := len(s)
	i := 0

	for i < n {
		// scan for opener: {{ or ${
		var kind delimKind
		var fullStart int
		var delimLen int

		if i < n-1 && s[i] == '{' && s[i+1] == '{' {
			kind = delimDoubleBrace
			fullStart = i
			delimLen = 2
		} else if i < n-1 && s[i] == '$' && s[i+1] == '{' {
			// check for ${{ which is literal $ followed by {{ expression
			if i+2 < n && s[i+2] == '{' {
				// treat as literal $, the {{ will be picked up on next iteration
				i++
				continue
			}
			kind = delimDollarBrace
			fullStart = i
			delimLen = 2
		} else {
			i++
			continue
		}

		i += delimLen // skip past opener

		// scan for closer, tracking quote state
		var inDouble, inSingle, inBacktick, escaped bool
		braceDepth := 1 // for ${ } mode
		closed := false
		innerRawStart := i

	exprScan:
		for i < n {
			ch := s[i]

			if escaped {
				escaped = false
				i++
				continue
			}

			if inBacktick {
				// raw strings: no escape processing, only exit on backtick
				if ch == '`' {
					inBacktick = false
				}
				i++
				continue
			}

			if inDouble {
				if ch == '\\' {
					escaped = true
				} else if ch == '"' {
					inDouble = false
				}
				i++
				continue
			}

			if inSingle {
				if ch == '\\' {
					escaped = true
				} else if ch == '\'' {
					inSingle = false
				}
				i++
				continue
			}

			// not inside any quote
			if ch == '"' {
				inDouble = true
			} else if ch == '\'' {
				inSingle = true
			} else if ch == '`' {
				inBacktick = true
			} else if kind == delimDoubleBrace && ch == '}' && i+1 < n && s[i+1] == '}' {
				m := buildMatch(s, fullStart, i+2, innerRawStart, i)
				matches = append(matches, m)
				i += 2
				closed = true

				break exprScan
			} else if kind == delimDollarBrace {
				if ch == '{' {
					braceDepth++
				} else if ch == '}' {
					braceDepth--
					if braceDepth == 0 {
						m := buildMatch(s, fullStart, i+1, innerRawStart, i)
						matches = append(matches, m)
						i++
						closed = true

						break exprScan
					}
				}
			}

			i++
		}

		if !closed {
			// unterminated expression, skip past the opener and continue scanning
			i = fullStart + delimLen
		}
	}

	return matches
}

// buildMatch creates a templateMatch with whitespace-trimmed inner bounds
func buildMatch(s string, fullStart, fullEnd, innerRawStart, innerRawEnd int) templateMatch {
	inner := s[innerRawStart:innerRawEnd]
	innerTrimmed := strings.TrimSpace(inner)
	innerStart := innerRawStart + strings.Index(inner, innerTrimmed)
	innerEnd := innerStart + len(innerTrimmed)

	if innerTrimmed == "" {
		innerStart = innerRawStart
		innerEnd = innerRawStart
	}

	return templateMatch{
		fullStart:  fullStart,
		fullEnd:    fullEnd,
		innerStart: innerStart,
		innerEnd:   innerEnd,
	}
}

// hasTemplateExpression returns true if the string contains any template expressions
func hasTemplateExpression(s string) bool {
	if !strings.Contains(s, "{{") && !strings.Contains(s, "${") {
		return false
	}

	return len(findTemplateExpressions(s)) > 0
}

// Env represents the template execution environment containing facts and data
type Env struct {
	Facts      map[string]any    `json:"facts" yaml:"facts"`
	Data       map[string]any    `json:"data" yaml:"data"`
	Environ    map[string]string `json:"environ" yaml:"environ"`
	WorkingDir string            `json:"-" yaml:"-"`

	RegistrationsFunc func(cluster, protocol, service, ip string) (any, error) `json:"-" yaml:"-"`

	// DefaultOnMissing when true causes lookup() to return "" instead of an error
	// when a key is missing and no default is provided
	DefaultOnMissing bool `json:"-" yaml:"-"`

	// RestrictFunctions when true only registers lookup() in expressions,
	// excluding file I/O and template functions
	RestrictFunctions bool `json:"-" yaml:"-"`

	envJSON json.RawMessage
	mu      sync.Mutex
}

func (e *Env) readFile(params ...any) (any, error) {
	var file string
	var ok bool

	if len(params) == 0 {
		return "", fmt.Errorf("readFile requires a string argument")
	}

	file, ok = params[0].(string)
	if !ok {
		return "", fmt.Errorf("readFile requires a string argument")
	}

	if !filepath.IsAbs(file) && e.WorkingDir != "" {
		file = filepath.Join(e.WorkingDir, filepath.Clean(file))
	}

	fb, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file %q: working dir: %q: %w", file, e.WorkingDir, err)
	}

	return string(fb), nil
}

func (e *Env) template(params ...any) (any, error) {
	var contents string
	var ok bool

	if len(params) == 0 {
		return "", fmt.Errorf("template requires a string argument")
	}

	contents, ok = params[0].(string)
	if !ok {
		return "", fmt.Errorf("template requires a string argument")
	}

	suff := filepath.Ext(contents)
	switch suff {
	case ".templ":
		f, err := e.readFile(contents)
		if err != nil {
			return nil, err
		}
		contents = f.(string)
	case ".jet":
		return e.jet(contents)
	}

	return ResolveTemplateTyped(contents, e)
}

func (e *Env) registrations(params ...any) (any, error) {
	if len(params) != 4 {
		return nil, fmt.Errorf("registrations requires 4 string arguments: cluster, protocol, service, ip")
	}

	args := make([]string, 4)
	for i, p := range params {
		s, ok := p.(string)
		if !ok {
			return nil, fmt.Errorf("registrations argument %d must be a string", i+1)
		}
		args[i] = s
	}

	if e.RegistrationsFunc == nil {
		return nil, fmt.Errorf("registrations function not available")
	}

	return e.RegistrationsFunc(args[0], args[1], args[2], args[3])
}

func (e *Env) lookup(params ...any) (any, error) {
	var key string
	var defaultValue any
	var ok bool

	if len(params) == 0 || len(params) > 2 {
		return nil, fmt.Errorf("lookup requires 1 or 2 arguments")
	}

	key, ok = params[0].(string)
	if !ok {
		return nil, fmt.Errorf("lookup requires a string argument")
	}

	if len(params) == 2 {
		defaultValue = params[1]
	} else {
		defaultValue = nil
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
		if defaultValue == nil {
			if e.DefaultOnMissing {
				return "", nil
			}
			return "", fmt.Errorf("missing key '%s' in environment", key)
		}
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

// ResolveTemplateStringMatch resolves {{ expression }} placeholders in a template string and returns the result,
// a boolean indicating if any placeholders matched and produced non-empty values, and any error
func ResolveTemplateStringMatch(template string, env *Env) (string, bool, error) {
	return applyFactsString(template, env)
}

// ResolveTemplateString resolves {{ expression }} placeholders in a template string and returns the result as a string
func ResolveTemplateString(template string, env *Env) (string, error) {
	if template == "" {
		return "", nil
	}

	if !hasTemplateExpression(template) {
		return template, nil
	}

	res, _, err := applyFactsString(template, env)
	return res, err
}

// ResolveTemplateTyped resolves {{ expression }} placeholders and preserves the type of single expressions
func ResolveTemplateTyped(template string, env *Env) (any, error) {
	if template == "" {
		return "", nil
	}

	matches := findTemplateExpressions(template)
	if len(matches) == 0 {
		return template, nil
	}

	if len(matches) == 1 {
		trimmed := strings.TrimSpace(template)
		fullMatch := template[matches[0].fullStart:matches[0].fullEnd]

		if trimmed == fullMatch {
			inner := template[matches[0].innerStart:matches[0].innerEnd]
			return ExprParse(inner, env)
		}
	}

	res, _, err := applyFactsString(template, env)
	return res, err
}

// applyFactsString parses {{ expression}} placeholders using expr and replace them with the resulting values
func applyFactsString(template string, env *Env) (string, bool, error) {
	matches := findTemplateExpressions(template)
	if len(matches) == 0 {
		// nothing to replace, so we report that we matched because this string should be used for those who care about matching
		return template, template != "", nil
	}

	// We will build the output incrementally
	var result strings.Builder
	lastIndex := 0
	var matched []bool

	for _, m := range matches {
		innerExpr := template[m.innerStart:m.innerEnd]

		value, err := ExprParse(innerExpr, env)
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
		result.WriteString(template[lastIndex:m.fullStart])
		// Now the match
		result.WriteString(fmt.Sprint(value))

		lastIndex = m.fullEnd
	}

	// Append any remainder after last match
	result.WriteString(template[lastIndex:])

	return result.String(), slices.Contains(matched, true), nil
}
