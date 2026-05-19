// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"fmt"
	"reflect"

	"github.com/goccy/go-yaml"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/templates"
)

// defaultSchemaPlaceholder is substituted for any template-bearing string field
// that lacks a `schema_placeholder` struct tag. It is chosen to satisfy the
// generic schema patterns used for names and identifiers across the manifest.
const defaultSchemaPlaceholder = "ccmplaceholder"

// substituteTemplatesForValidation produces a deep copy of prop with any
// remaining template expressions replaced by placeholder values, ready to be
// schema-validated. The copy is independent of prop, so callers may continue to
// use the original after this returns.
//
// Non-deferred template fields are expected to have been resolved before this
// runs. Anything still containing `${...}` or `{{...}}` is either a deferred
// field (e.g. exec environment, file content) or a reference that will be
// resolved at execution time. For those, the `schema_placeholder:"..."` struct
// tag declares a value that satisfies the schema's pattern constraints; absent
// a tag we fall back to defaultSchemaPlaceholder.
func substituteTemplatesForValidation(prop model.ResourceProperties) (model.ResourceProperties, error) {
	propValue := reflect.ValueOf(prop)
	if propValue.Kind() != reflect.Ptr || propValue.IsNil() {
		return nil, fmt.Errorf("expected non-nil pointer to resource properties, got %T", prop)
	}

	raw, err := prop.ToYamlManifest()
	if err != nil {
		return nil, fmt.Errorf("could not marshal resource for validation: %w", err)
	}

	dup := reflect.New(propValue.Type().Elem()).Interface()
	err = yaml.Unmarshal(raw, dup)
	if err != nil {
		return nil, fmt.Errorf("could not round-trip resource for validation: %w", err)
	}

	dupProp, ok := dup.(model.ResourceProperties)
	if !ok {
		return nil, fmt.Errorf("internal error: copy of %T does not satisfy ResourceProperties", prop)
	}

	// Type is json/yaml-skipped so it does not round-trip. Restore it so that
	// downstream validators that consult CommonProperties().Type continue to
	// work; the schema does not inspect it.
	if cp := dupProp.CommonProperties(); cp != nil {
		cp.Type = prop.CommonProperties().Type
	}

	err = substituteStruct(reflect.ValueOf(dupProp).Elem())
	if err != nil {
		return nil, err
	}

	return dupProp, nil
}

func substituteStruct(v reflect.Value) error {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		if !fv.CanSet() {
			continue
		}

		if field.Tag.Get("json") == "-" {
			continue
		}

		// Free-form maps (e.g. scaffold Data) cannot be placeholder-substituted
		// generically; they are also not subject to schema pattern constraints.
		if field.Tag.Get("template") == "resolve_keys" {
			continue
		}

		// Fields marked template:"-" are intentionally never resolved. If a
		// user has placed template syntax there it is a manifest error; the
		// schema should reject the literal expression rather than have us
		// silently substitute a placeholder.
		if field.Tag.Get("template") == "-" {
			continue
		}

		if field.Anonymous && fv.Kind() == reflect.Struct {
			err := substituteStruct(fv)
			if err != nil {
				return err
			}
			continue
		}

		placeholder := field.Tag.Get("schema_placeholder")
		if placeholder == "" {
			placeholder = defaultSchemaPlaceholder
		}

		err := substituteValue(fv, placeholder)
		if err != nil {
			return err
		}
	}

	return nil
}

func substituteValue(fv reflect.Value, placeholder string) error {
	switch fv.Kind() {
	case reflect.String:
		s := fv.String()
		if !templates.HasTemplateExpression(s) {
			return nil
		}
		fv.SetString(placeholder)
		return nil

	case reflect.Slice:
		if fv.IsNil() || fv.Len() == 0 {
			return nil
		}
		if fv.Type().Elem().Kind() == reflect.Uint8 {
			return nil
		}
		for i := 0; i < fv.Len(); i++ {
			err := substituteValue(fv.Index(i), placeholder)
			if err != nil {
				return err
			}
		}
		return nil

	case reflect.Map:
		if fv.IsNil() || fv.Type().Key().Kind() != reflect.String {
			return nil
		}
		valType := fv.Type().Elem()
		if valType.Kind() == reflect.String {
			for _, key := range fv.MapKeys() {
				val := fv.MapIndex(key).String()
				if !templates.HasTemplateExpression(val) {
					continue
				}
				fv.SetMapIndex(key, reflect.ValueOf(placeholder))
			}
		}
		return nil

	case reflect.Struct:
		return substituteStruct(fv)

	case reflect.Ptr:
		if fv.IsNil() {
			return nil
		}
		return substituteValue(fv.Elem(), placeholder)

	case reflect.Interface:
		if fv.IsNil() {
			return nil
		}
		concrete := fv.Elem()
		if concrete.Kind() != reflect.String {
			return nil
		}
		s := concrete.String()
		if !templates.HasTemplateExpression(s) {
			return nil
		}
		fv.Set(reflect.ValueOf(placeholder))
		return nil
	}

	return nil
}
