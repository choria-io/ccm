// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package templates

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/CloudyKit/jet/v6"
	"github.com/expr-lang/expr"
	"github.com/tidwall/gjson"
)

// Env represents the template execution environment containing facts and data
type Env struct {
	Facts      map[string]any    `json:"facts" yaml:"facts"`
	Data       map[string]any    `json:"data" yaml:"data"`
	Environ    map[string]string `json:"environ" yaml:"environ"`
	WorkingDir string            `json:"-" yaml:"-"`

	envJSON json.RawMessage
	mu      sync.Mutex
}

func (e *Env) JetVariables() jet.VarMap {
	return jet.VarMap{
		"facts":   reflect.ValueOf(e.Facts),
		"Facts":   reflect.ValueOf(e.Facts),
		"data":    reflect.ValueOf(e.Data),
		"Data":    reflect.ValueOf(e.Data),
		"environ": reflect.ValueOf(e.Environ),
		"Environ": reflect.ValueOf(e.Environ),
	}
}

func (e *Env) JetFunctions() map[string]jet.Func {
	return map[string]jet.Func{
		"lookup": e.jetLookup(),
	}
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

	if filepath.IsAbs(file) {
		return "", fmt.Errorf("readFile can only read files in the working directory")
	}

	if e.WorkingDir != "" {
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

func (e *Env) jet(params ...any) (any, error) {
	lpat := "[["
	rpat := "]]"
	body := ""
	var ok bool

	switch len(params) {
	case 1:
		body, ok = params[0].(string)
		if !ok {
			return "", fmt.Errorf("jet requires a string argument for template body")
		}

	case 3:
		body, ok = params[0].(string)
		if !ok {
			return "", fmt.Errorf("jet requires a string argument for template body")
		}

		lpat, ok = params[1].(string)
		if !ok {
			return "", fmt.Errorf("jet requires a string argument for left delimiter")
		}

		rpat, ok = params[2].(string)
		if !ok {
			return "", fmt.Errorf("jet requires a string argument for right delimiter")
		}
	default:
		return "", fmt.Errorf("jet requires 1 or 3 arguments")
	}

	if strings.HasSuffix(body, ".jet") {
		f, err := e.readFile(body)
		if err != nil {
			return nil, err
		}
		body = f.(string)
	}

	set := jet.NewSet(jet.NewInMemLoader(), jet.WithDelims(lpat, rpat))
	tpl, err := set.Parse("input", body)
	if err != nil {
		return nil, err
	}

	for k, v := range e.JetFunctions() {
		set.AddGlobalFunc(k, v)
	}

	variables := jet.VarMap{
		"facts":   reflect.ValueOf(e.Facts),
		"Facts":   reflect.ValueOf(e.Facts),
		"data":    reflect.ValueOf(e.Data),
		"Data":    reflect.ValueOf(e.Data),
		"environ": reflect.ValueOf(e.Environ),
		"Environ": reflect.ValueOf(e.Environ),
	}

	buff := bytes.NewBuffer([]byte{})
	err = tpl.Execute(buff, variables, e)
	if err != nil {
		return nil, err
	}

	return buff.String(), nil
}

func (e *Env) jetLookup() jet.Func {
	return func(a jet.Arguments) reflect.Value {
		// 1–2 arguments, same as your original intent
		a.RequireNumOfArguments("lookup", 1, 2)

		// First arg: key (string)
		var key string
		if err := a.ParseInto(&key); err != nil {
			a.Panicf("lookup: first argument must be a string: %v", err)
		}

		// Optional second arg: default value (any)
		var defaultValue any
		if a.NumOfArguments() == 2 {
			defaultValue = a.Get(1).Interface()
		} else {
			defaultValue = nil
		}

		val, err := e.lookup(key, defaultValue)
		if err != nil {
			a.Panicf("lookup: failed: %v", err)
		}

		return reflect.ValueOf(val)
	}
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
		return ExprParse(matches[0][1], env)
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
		result.WriteString(out[lastIndex:fullStart])
		// Now the match
		result.WriteString(fmt.Sprint(value))

		lastIndex = fullEnd
	}

	// Append any remainder after last match
	result.WriteString(out[lastIndex:])

	return result.String(), slices.Contains(matched, true), nil
}

func ExprParse(query string, env *Env, opts ...expr.Option) (any, error) {
	o := []expr.Option{
		expr.Env(env),
		expr.Function("lookup", env.lookup),
		expr.Function("readFile", env.readFile),
		expr.Function("file", env.readFile),
		expr.Function("template", env.template),
		expr.Function("jet", env.jet),
	}
	o = append(o, opts...)

	program, err := expr.Compile(query, o...)
	if err != nil {
		return "", fmt.Errorf("expr compile error for '%s': %w", query, err)
	}

	return expr.Run(program, env)
}
