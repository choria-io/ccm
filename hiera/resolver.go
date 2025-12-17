// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package hiera

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
	"github.com/expr-lang/expr"
	"github.com/goccy/go-yaml"
	"github.com/tidwall/gjson"
)

// Hierarchy describes how data sections should be resolved.
type Hierarchy struct {
	// Order defines the lookup sequence for data sections.
	Order []string `yaml:"order"`
	// Merge selects the merge strategy ("first" or "deep").
	Merge string `yaml:"merge"`
}

type Options struct {
	DataKey string
}

var DefaultOptions = Options{
	DataKey: "data",
}

var (
	// maxInt represents the largest int value for the current architecture and is used to safely normalize numeric types.
	maxInt = int(^uint(0) >> 1)
	// minInt represents the smallest int value for the current architecture and is used to safely normalize numeric types.
	minInt = -maxInt - 1

	DefaultHierarchy = map[string]any{
		"order": []any{"default"},
	}
)

// Resolve consumes a parsed data document and a map of facts to produce a final data map.
// The data map is expected to contain a hierarchy section, a base data section, and any number of overlays.
// Placeholders in the hierarchy order (e.g. env:%{env}) are replaced with values from the provided facts map.
func Resolve(root map[string]any, facts map[string]any, opts Options, log model.Logger) (map[string]any, error) {
	if opts.DataKey == "" {
		opts.DataKey = "data"
	}

	_, ok := root["hierarchy"]
	if !ok {
		root["hierarchy"] = DefaultHierarchy
	}

	normalizedRoot, ok := normalizeNumericValues(root).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("root document must be a map")
	}

	root = normalizedRoot
	var overrides map[string]any
	if _, ok := root["overrides"]; ok {
		overrides = root["overrides"].(map[string]any)
	}

	hierarchy, err := parseHierarchy(root)
	if err != nil {
		return nil, err
	}

	env := map[string]any{
		"facts": facts,
	}

	base := map[string]any{}
	data, hasData := root[opts.DataKey].(map[string]any)
	if hasData {
		base, err = expandMapExprValues(iu.CloneMap(data), env)
		if err != nil {
			return nil, err
		}
	}

	mergeMode := strings.ToLower(hierarchy.Merge)
	if mergeMode == "" {
		mergeMode = "first"
	}

	for _, entry := range hierarchy.Order {
		resolvedKey, matched, err := applyFactsString(entry, env)
		if err != nil {
			return nil, err
		}

		if !matched {
			continue
		}

		if log != nil {
			log.Debug("Evaluating override", "override", resolvedKey)
		}

		candidateKey := resolvedKey
		if candidateKey == opts.DataKey && hasData {
			continue
		}
		candidate, ok := overrides[candidateKey].(map[string]any)
		if !ok {
			continue
		}

		candidate, err = expandMapExprValues(iu.CloneMap(candidate), env)
		if err != nil {
			return nil, err
		}

		switch mergeMode {
		case "deep":
			base = iu.DeepMergeMap(base, candidate)
		case "first":
			base = iu.ShallowMerge(base, candidate)
			return base, nil
		default:
			return nil, fmt.Errorf("unsupported merge mode: %s", mergeMode)
		}
	}

	return base, nil
}

// ResolveYaml consumes raw YAML bytes and a map of facts to produce a final data map.
// The function decodes the YAML document and delegates processing to Resolve to perform merges and fact substitution.
func ResolveYaml(data []byte, facts map[string]any, opts Options, log model.Logger) (map[string]any, error) {
	root := map[string]any{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return Resolve(root, facts, opts, log)
}

// ResolveJson consumes raw JSON bytes and a map of facts to produce a final data map.
// The function decodes the JSON document and delegates processing to Resolve to perform merges and fact substitution.
func ResolveJson(data []byte, facts map[string]any, opts Options, log model.Logger) (map[string]any, error) {
	root := map[string]any{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return Resolve(root, facts, opts, log)
}

// parseHierarchy extracts the hierarchy definition from the raw YAML map.
func parseHierarchy(root map[string]any) (Hierarchy, error) {
	raw, ok := root["hierarchy"].(map[string]any)
	if !ok {
		root["hierarchy"] = map[string]any{}
		return Hierarchy{}, fmt.Errorf("hierarchy section is required")
	}

	orderSlice, ok := raw["order"].([]any)
	if !ok {
		return Hierarchy{}, fmt.Errorf("hierarchy.order must be a list")
	}

	order := make([]string, 0, len(orderSlice))
	for _, item := range orderSlice {
		text, ok := item.(string)
		if !ok {
			return Hierarchy{}, fmt.Errorf("hierarchy.order must contain only strings")
		}
		order = append(order, text)
	}

	mergeMode, _ := raw["merge"].(string)

	return Hierarchy{Order: order, Merge: mergeMode}, nil
}

func genExprEnv(facts map[string]any) (map[string]any, error) {
	env := iu.CloneMap(facts)

	// do not try to json marshal these functions
	delete(env, "lookup")

	j, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}

	env["lookup"] = func(key string, args ...any) (any, error) {
		var dflt any
		if len(args) >= 1 {
			dflt = args[0]
		} else {
			dflt = ""
		}

		res := gjson.GetBytes(j, key)
		if !res.Exists() {
			return dflt, nil
		}

		if res.Type == gjson.Number {
			if strings.Contains(res.Raw, ".") {
				return res.Float(), nil
			} else {
				return res.Int(), nil
			}
		}

		return res.Value(), nil
	}

	return env, nil
}

func applyFactsTyped(template string, facts map[string]any) (any, error) {
	re := regexp.MustCompile(`{{\s*(.*?)\s*}}`)
	trimmed := strings.TrimSpace(template)

	matches := re.FindAllStringSubmatch(template, -1)
	switch {
	case matches == nil:
		return template, nil
	case len(matches) == 1 && strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}"):
		return exprParse(matches[0][1], facts)
	default:
		res, _, err := applyFactsString(template, facts)
		return res, err
	}
}

// applyFactsString parses {{ expression}} placeholders using expr and replace them with the resulting values
func applyFactsString(template string, facts map[string]any) (string, bool, error) {
	// Matches: {{ something }}
	// Capture group 1 = inner text
	re := regexp.MustCompile(`{{\s*(.*?)\s*}}`)

	out := template

	matches := re.FindAllStringSubmatchIndex(template, -1)
	if matches == nil {
		// nothing to replace so we report that we matched because this string should be used for those who care about matching
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

		value, err := exprParse(innerExpr, facts)
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

func exprParse(query string, facts map[string]any) (any, error) {
	env, err := genExprEnv(facts)
	if err != nil {
		return "", err
	}

	program, err := expr.Compile(query, expr.Env(env))
	if err != nil {
		return "", fmt.Errorf("expr compile error for '%s': %w", query, err)
	}

	return expr.Run(program, env)
}

func expandMapExprValues(value map[string]any, facts map[string]any) (map[string]any, error) {
	for k, v := range value {
		nv, err := expandExprValuesRecursively(v, facts)
		if err != nil {
			return nil, err
		}
		value[k] = nv
	}

	return value, nil
}

// expandExprValuesRecursively walks a data structure and replaces {{ expression }} placeholders in all string values.
// Maps and slices are recursively processed, while other types are returned unchanged.
func expandExprValuesRecursively(value any, facts map[string]any) (any, error) {
	switch typed := value.(type) {
	case string:
		// Apply expr template expansion to string values
		return applyFactsTyped(typed, facts)
	case map[string]any:
		// Recursively process all map values
		result := make(map[string]any, len(typed))
		for key, val := range typed {
			expanded, err := expandExprValuesRecursively(val, facts)
			if err != nil {
				return nil, err
			}
			result[key] = expanded
		}
		return result, nil
	case []any:
		// Recursively process all slice elements
		result := make([]any, len(typed))
		for i, val := range typed {
			expanded, err := expandExprValuesRecursively(val, facts)
			if err != nil {
				return nil, err
			}
			result[i] = expanded
		}
		return result, nil
	default:
		// Return non-string, non-container types unchanged (int, bool, float, etc.)
		return typed, nil
	}
}

// normalizeNumericValues walks a decoded YAML structure and converts numeric values into int when they safely fit the
// platform size. This keeps numeric handling consistent with the expectations of the resolver and its tests.
func normalizeNumericValues(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, val := range typed {
			result[key] = normalizeNumericValues(val)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i, val := range typed {
			result[i] = normalizeNumericValues(val)
		}
		return result
	case int64:
		if typed >= int64(minInt) && typed <= int64(maxInt) {
			return int(typed)
		}
		return typed
	case uint64:
		if typed <= uint64(maxInt) {
			return int(typed)
		}
		return typed
	case float64:
		if typed == math.Trunc(typed) && typed >= float64(minInt) && typed <= float64(maxInt) {
			return int(typed)
		}
		return typed
	case int, int8, int16, int32:
		return typed
	case uint:
		if typed <= uint(maxInt) {
			return int(typed)
		}
		return typed
	case uint8:
		return int(typed)
	case uint16:
		return int(typed)
	case uint32:
		if typed <= uint32(maxInt) {
			return int(typed)
		}
		return typed
	default:
		return typed
	}
}
