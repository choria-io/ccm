// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package templates

import (
	"fmt"
	"reflect"
)

// ResolveStructTemplates walks target (must be a pointer to a struct) via reflection
// and resolves all string-typed fields using ResolveTemplateString. It recurses into
// all composite types (slices, maps, structs, pointers) automatically.
//
// Fields are controlled by the `template` struct tag:
//   - template:"-"             always skipped
//   - template:"deferred"      skipped when deferred=false, resolved when deferred=true
//   - template:"resolve_keys"  for map fields, also resolve map keys (rebuilds the map)
//   - no tag                   resolved when deferred=false, skipped when deferred=true
//
// Fields with json:"-" are always skipped (internal computed fields).
// Unexported fields are silently skipped.
func ResolveStructTemplates(target any, env *Env, deferred bool) error {
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("ResolveStructTemplates requires a pointer to a struct, got %T", target)
	}

	return resolveStruct(v.Elem(), env, deferred, "")
}

func resolveStruct(v reflect.Value, env *Env, deferred bool, prefix string) error {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		if !fv.CanSet() {
			continue
		}

		// Skip fields tagged json:"-" (internal computed fields)
		if jsonTag := field.Tag.Get("json"); jsonTag == "-" {
			continue
		}

		templateTag := field.Tag.Get("template")

		// Always skip template:"-"
		if templateTag == "-" {
			continue
		}

		// Phase filtering
		isDeferred := templateTag == "deferred"
		if deferred && !isDeferred {
			continue
		}
		if !deferred && isDeferred {
			continue
		}

		fieldName := field.Name
		if prefix != "" {
			fieldName = prefix + "." + fieldName
		}

		// Handle embedded (anonymous) structs - recurse directly
		if field.Anonymous && fv.Kind() == reflect.Struct {
			if err := resolveStruct(fv, env, deferred, fieldName); err != nil {
				return err
			}
			continue
		}

		resolveKeys := templateTag == "resolve_keys"

		if err := resolveValue(fv, env, deferred, resolveKeys, fieldName); err != nil {
			return err
		}
	}

	return nil
}

// resolveValue resolves templates in any value recursively based on its kind.
func resolveValue(fv reflect.Value, env *Env, deferred bool, resolveKeys bool, fieldName string) error {
	switch fv.Kind() {
	case reflect.String:
		return resolveStringValue(fv, env, fieldName)

	case reflect.Slice:
		return resolveSlice(fv, env, deferred, fieldName)

	case reflect.Map:
		if fv.IsNil() {
			return nil
		}
		return resolveMap(fv, env, deferred, resolveKeys, fieldName)

	case reflect.Struct:
		return resolveStruct(fv, env, deferred, fieldName)

	case reflect.Ptr:
		if fv.IsNil() {
			return nil
		}
		return resolveValue(fv.Elem(), env, deferred, resolveKeys, fieldName)

	case reflect.Interface:
		if fv.IsNil() {
			return nil
		}
		return resolveInterfaceValue(fv, env, deferred, resolveKeys, fieldName)
	}

	return nil
}

func resolveStringValue(fv reflect.Value, env *Env, fieldName string) error {
	s := fv.String()
	if s == "" {
		return nil
	}

	resolved, err := ResolveTemplateString(s, env)
	if err != nil {
		return fmt.Errorf("resolving field %s: %w", fieldName, err)
	}

	fv.SetString(resolved)

	return nil
}

func resolveSlice(fv reflect.Value, env *Env, deferred bool, fieldName string) error {
	if fv.IsNil() || fv.Len() == 0 {
		return nil
	}

	// []byte / yaml.RawMessage - skip
	if fv.Type().Elem().Kind() == reflect.Uint8 {
		return nil
	}

	for i := 0; i < fv.Len(); i++ {
		elem := fv.Index(i)
		elemName := fmt.Sprintf("%s[%d]", fieldName, i)

		if err := resolveValue(elem, env, deferred, false, elemName); err != nil {
			return err
		}
	}

	return nil
}

func resolveMap(fv reflect.Value, env *Env, deferred bool, resolveKeys bool, fieldName string) error {
	if fv.Type().Key().Kind() != reflect.String {
		return nil
	}

	valType := fv.Type().Elem()

	// For map[string]string without key resolution we can resolve values in place
	if valType.Kind() == reflect.String && !resolveKeys {
		for _, key := range fv.MapKeys() {
			val := fv.MapIndex(key).String()
			if val == "" {
				continue
			}

			resolved, err := ResolveTemplateString(val, env)
			if err != nil {
				return fmt.Errorf("resolving field %s[%s]: %w", fieldName, key.String(), err)
			}

			fv.SetMapIndex(key, reflect.ValueOf(resolved))
		}

		return nil
	}

	// For all other maps (resolve_keys, interface values, struct values, etc.)
	// we must rebuild to handle key resolution and non-addressable map values
	newMap := reflect.MakeMap(fv.Type())

	for _, key := range fv.MapKeys() {
		k := key.String()

		resolvedKey := k
		if resolveKeys {
			var err error
			resolvedKey, err = ResolveTemplateString(k, env)
			if err != nil {
				return fmt.Errorf("resolving field %s key %s: %w", fieldName, k, err)
			}
		}

		val := fv.MapIndex(key)
		elemName := fmt.Sprintf("%s[%s]", fieldName, resolvedKey)

		resolved, err := resolveMapValue(val, env, deferred, elemName)
		if err != nil {
			return err
		}

		newMap.SetMapIndex(reflect.ValueOf(resolvedKey), resolved)
	}

	fv.Set(newMap)

	return nil
}

// resolveMapValue resolves a map value, returning the resolved reflect.Value.
// Map values are not addressable so we can't modify them in place for non-string types,
// hence we return new values.
func resolveMapValue(v reflect.Value, env *Env, deferred bool, fieldName string) (reflect.Value, error) {
	// Unwrap interface
	actual := v
	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return v, nil
		}
		actual = v.Elem()
	}

	switch actual.Kind() {
	case reflect.String:
		s := actual.String()
		if s == "" {
			return v, nil
		}

		resolved, err := ResolveTemplateString(s, env)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("resolving field %s: %w", fieldName, err)
		}

		result := reflect.ValueOf(resolved)
		// If original was interface-wrapped, wrap the result back
		if v.Kind() == reflect.Interface {
			return result, nil
		}
		return result, nil

	case reflect.Map:
		if actual.Type().Key().Kind() == reflect.String {
			result := reflect.MakeMap(actual.Type())
			for _, key := range actual.MapKeys() {
				resolved, err := resolveMapValue(actual.MapIndex(key), env, deferred, fmt.Sprintf("%s[%s]", fieldName, key.String()))
				if err != nil {
					return reflect.Value{}, err
				}
				result.SetMapIndex(key, resolved)
			}
			return result, nil
		}
		return v, nil

	case reflect.Slice:
		if actual.IsNil() || actual.Len() == 0 {
			return v, nil
		}
		// []byte - skip
		if actual.Type().Elem().Kind() == reflect.Uint8 {
			return v, nil
		}
		result := reflect.MakeSlice(actual.Type(), actual.Len(), actual.Len())
		for i := 0; i < actual.Len(); i++ {
			resolved, err := resolveMapValue(actual.Index(i), env, deferred, fmt.Sprintf("%s[%d]", fieldName, i))
			if err != nil {
				return reflect.Value{}, err
			}
			result.Index(i).Set(resolved)
		}
		return result, nil

	default:
		return v, nil
	}
}

// resolveInterfaceValue handles interface-typed fields by inspecting the concrete value.
func resolveInterfaceValue(fv reflect.Value, env *Env, deferred bool, resolveKeys bool, fieldName string) error {
	concrete := fv.Elem()

	switch concrete.Kind() {
	case reflect.String:
		s := concrete.String()
		if s == "" {
			return nil
		}

		resolved, err := ResolveTemplateString(s, env)
		if err != nil {
			return fmt.Errorf("resolving field %s: %w", fieldName, err)
		}

		fv.Set(reflect.ValueOf(resolved))

	case reflect.Map:
		if concrete.Type().Key().Kind() == reflect.String {
			resolved, err := resolveMapValue(fv, env, deferred, fieldName)
			if err != nil {
				return err
			}
			fv.Set(resolved)
		}

	case reflect.Slice:
		if concrete.Len() > 0 && concrete.Type().Elem().Kind() != reflect.Uint8 {
			resolved, err := resolveMapValue(fv, env, deferred, fieldName)
			if err != nil {
				return err
			}
			fv.Set(resolved)
		}
	}

	return nil
}
