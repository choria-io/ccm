// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"encoding/json"
	"io"
	"reflect"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/choria-io/ccm/internal/fs"
	"github.com/choria-io/ccm/model"
)

// resourceTypesByName maps the schema definition prefix to a zero-value pointer
// of the corresponding Go properties struct. New resource types must be added
// here so the schema_placeholder invariant covers them.
var resourceTypesByName = map[string]model.ResourceProperties{
	"file":     &model.FileResourceProperties{},
	"exec":     &model.ExecResourceProperties{},
	"package":  &model.PackageResourceProperties{},
	"service":  &model.ServiceResourceProperties{},
	"archive":  &model.ArchiveResourceProperties{},
	"scaffold": &model.ScaffoldResourceProperties{},
}

var _ = Describe("schema_placeholder invariants", func() {
	var schemaPatterns map[string]map[string]string

	BeforeEach(func() {
		raw, err := fs.FS.Open("schemas/manifest.json")
		Expect(err).NotTo(HaveOccurred())
		defer raw.Close()

		data, err := io.ReadAll(raw)
		Expect(err).NotTo(HaveOccurred())

		var schema map[string]any
		err = json.Unmarshal(data, &schema)
		Expect(err).NotTo(HaveOccurred())

		schemaPatterns = collectResourcePatterns(schema)
	})

	It("every template:deferred field with a schema pattern carries a schema_placeholder tag", func() {
		for typeName, prop := range resourceTypesByName {
			patterns, ok := schemaPatterns[typeName]
			if !ok {
				continue
			}

			propType := reflect.TypeOf(prop).Elem()
			eachStructField(propType, func(field reflect.StructField) {
				yamlName := yamlFieldName(field)
				if yamlName == "" {
					return
				}

				pattern, hasPattern := patterns[yamlName]
				if !hasPattern {
					return
				}

				if field.Tag.Get("template") != "deferred" {
					return
				}

				placeholder := field.Tag.Get("schema_placeholder")
				Expect(placeholder).NotTo(BeEmpty(),
					"resource %q field %q is template:\"deferred\" and has schema pattern %q but no schema_placeholder tag",
					typeName, yamlName, pattern)

				re, err := regexp.Compile(pattern)
				Expect(err).NotTo(HaveOccurred(), "pattern %q for %s.%s did not compile", pattern, typeName, yamlName)

				Expect(re.MatchString(placeholder)).To(BeTrue(),
					"schema_placeholder %q for %s.%s does not match schema pattern %q",
					placeholder, typeName, yamlName, pattern)
			})
		}
	})
})

// collectResourcePatterns walks the manifest schema and returns a map keyed
// by resource type name (file, exec, ...) whose values map yaml property names
// to the schema pattern they enforce. Patterns on array items are attributed to
// the array's parent property so the corresponding []string field on the Go
// struct can be located via its yaml tag.
func collectResourcePatterns(schema map[string]any) map[string]map[string]string {
	defs, _ := schema["$defs"].(map[string]any)
	out := map[string]map[string]string{}

	for typeName := range resourceTypesByName {
		out[typeName] = map[string]string{}

		for _, suffix := range []string{"ResourcePropertiesWithName", "ResourceProperties"} {
			def, _ := defs[typeName+suffix].(map[string]any)
			if def == nil {
				continue
			}
			props, _ := def["properties"].(map[string]any)
			for propName, raw := range props {
				prop, _ := raw.(map[string]any)
				if prop == nil {
					continue
				}
				if pattern, ok := prop["pattern"].(string); ok {
					out[typeName][propName] = pattern
					continue
				}
				if items, ok := prop["items"].(map[string]any); ok {
					if pattern, ok := items["pattern"].(string); ok {
						out[typeName][propName] = pattern
					}
				}
			}
		}
	}

	return out
}

// eachStructField recursively walks a struct type, invoking fn for every leaf
// (non-embedded) field. Embedded structs are descended into so that fields on
// CommonResourceProperties are visited alongside the resource's own fields.
func eachStructField(t reflect.Type, fn func(reflect.StructField)) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			eachStructField(field.Type, fn)
			continue
		}
		fn(field)
	}
}

// yamlFieldName returns the field's yaml name, stripping any options. Returns
// "" for fields explicitly skipped by `yaml:"-"`.
func yamlFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("yaml")
	if tag == "" {
		return ""
	}
	for i, ch := range tag {
		if ch == ',' {
			tag = tag[:i]
			break
		}
	}
	if tag == "-" {
		return ""
	}
	return tag
}
