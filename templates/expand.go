// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package templates

// ExpandMapValues resolves {{ expression }} placeholders in all values of a map
func ExpandMapValues(value map[string]any, env *Env) (map[string]any, error) {
	for k, v := range value {
		nv, err := ExpandValuesRecursively(v, env)
		if err != nil {
			return nil, err
		}
		value[k] = nv
	}

	return value, nil
}

// ExpandValuesRecursively walks a data structure and replaces {{ expression }} placeholders in all string values.
// Maps and slices are recursively processed, while other types are returned unchanged.
func ExpandValuesRecursively(value any, env *Env) (any, error) {
	switch typed := value.(type) {
	case string:
		return ResolveTemplateTyped(typed, env)
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, val := range typed {
			expanded, err := ExpandValuesRecursively(val, env)
			if err != nil {
				return nil, err
			}
			result[key] = expanded
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for i, val := range typed {
			expanded, err := ExpandValuesRecursively(val, env)
			if err != nil {
				return nil, err
			}
			result[i] = expanded
		}
		return result, nil
	default:
		return typed, nil
	}
}
