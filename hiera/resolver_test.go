// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package hiera

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHiera(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hiera Resolver Suite")
}

var _ = Describe("ResolveYaml", func() {
	It("merges data deeply following hierarchy order", func() {
		yamlData := []byte(`
hierarchy:
  order:
    - other:{{ lookup('facts.other', 'other') }}
    - env:{{ lookup('facts.env') }}
    - role:{{ lookup('facts.role') }}
    - host:{{ lookup('facts.hostname') }}
    - global
  merge: deep
data:
  log_level: INFO
  packages:
    - ca-certificates
  web:
    listen_port: 80
    tls: false
  other: test

overrides:
  env:prod:
    log_level: WARN

  role:web:
    packages:
    - nginx
    web:
      tls: true

  host:web01:
    log_level: TRACE

  other:stuff:
    other: extra
`)

		facts := map[string]any{
			"env":      "prod",
			"role":     "web",
			"hostname": "web01",
			"other":    "stuff",
		}

		result, err := ResolveYaml(yamlData, facts, DefaultOptions, nil)
		Expect(err).NotTo(HaveOccurred())

		Expect(result).To(Equal(map[string]any{
			"log_level": "TRACE",
			"packages":  []any{"ca-certificates", "nginx"},
			"other":     "extra",
			"web": map[string]any{
				"listen_port": 80,
				"tls":         true,
			},
		}))
	})

	It("returns the first matching overlay when using first merge mode", func() {
		yamlData := []byte(`
hierarchy:
  order:
    - env:{{ lookup('facts.env') }}
    - role:{ lookup('facts.role') }}
  merge: first

data:
  log_level: INFO

overrides:
    env:stage:
      log_level: DEBUG

    role:web:
      log_level: WARN
`)

		facts := map[string]any{
			"env":  "stage",
			"role": "web",
		}

		result, err := ResolveYaml(yamlData, facts, DefaultOptions, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"log_level": "DEBUG",
		}))
	})
})

var _ = Describe("Resolve", func() {
	It("Should support changing the data key", func() {
		data := map[string]any{
			"hierarchy": map[string]any{
				"order": []any{"data", "role:{{ lookup('role') | lower() }}"},
				"merge": "first",
			},
			"config": map[string]any{
				"value": 1,
			},
			"overrides": map[string]any{
				"role:web": map[string]any{
					"value": "{{ lookup('value') | int() }}",
				},
			},
		}

		facts := map[string]any{"role": "WEB", "value": 1}

		result, err := Resolve(data, facts, Options{DataKey: "config"}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"value": 1,
		}))
	})

	It("Should set a default hierarchy", func() {
		data := map[string]any{
			"config": map[string]any{
				"value": 1,
			},
			"overrides": map[string]any{
				"default": map[string]any{
					"value": "{{ lookup('facts.value') | int() }}",
				},
			},
		}

		facts := map[string]any{"role": "WEB", "value": 1}

		result, err := Resolve(data, facts, Options{DataKey: "config"}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"value": 1,
		}))
	})

	It("Should expand expr placeholders in override values", func() {
		data := map[string]any{
			"hierarchy": map[string]any{
				"order": []any{"data", "role:{{ lookup('facts.role') | lower() }}"},
				"merge": "first",
			},
			"data": map[string]any{
				"value": 1,
				"list":  []any{1},
				"other": "{{ lookup('facts.other') }}",
			},
			"overrides": map[string]any{
				"role:web": map[string]any{
					"list":  "{{ lookup('facts.list') }}",
					"value": "{{ lookup('facts.value') | int() }}",
				},
			},
		}

		facts := map[string]any{"role": "WEB", "value": 1, "list": []any{1}, "other": map[string]any{"key": "other"}}

		result, err := Resolve(data, facts, DefaultOptions, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"value": 1,
			"list":  []any{float64(1)}, // gjson converts to json first and there numbers become floats
			"other": map[string]any{"key": "other"},
		}))
	})

	It("processes an already parsed map without mutating input", func() {
		data := map[string]any{
			"hierarchy": map[string]any{
				"order": []any{"data", "role:{{ lookup('facts.role') | lower() }}"},
				"merge": "deep",
			},
			"data": map[string]any{
				"value": 1,
			},
			"overrides": map[string]any{
				"role:web": map[string]any{
					"list":  []any{float64(2)},
					"value": 2,
				},
			},
		}

		facts := map[string]any{"role": "WEB"}

		result, err := Resolve(data, facts, DefaultOptions, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"value": 2,
			"list":  []any{2},
		}))

		Expect(data).To(Equal(map[string]any{
			"hierarchy": map[string]any{
				"order": []any{"data", "role:{{ lookup('facts.role') | lower() }}"},
				"merge": "deep",
			},
			"data": map[string]any{
				"value": 1,
			},
			"overrides": map[string]any{
				"role:web": map[string]any{
					"list":  []any{float64(2)},
					"value": 2,
				},
			},
		}))
	})
})

var _ = Describe("parseHierarchy", func() {
	It("extracts order and merge data", func() {
		// Ensures hierarchy parsing returns expected values when the structure is correct.
		root := map[string]any{
			"hierarchy": map[string]any{
				"order": []any{"global", "env:%{env}"},
				"merge": "deep",
			},
		}

		hierarchy, err := parseHierarchy(root)
		Expect(err).NotTo(HaveOccurred())
		Expect(hierarchy.Order).To(Equal([]string{"global", "env:%{env}"}))
		Expect(hierarchy.Merge).To(Equal("deep"))
	})

	It("returns an error when the hierarchy is malformed", func() {
		// Validates that bad hierarchy data is rejected early.
		root := map[string]any{
			"hierarchy": map[string]any{
				"order": []any{"global", 2},
			},
		}

		_, err := parseHierarchy(root)
		Expect(err).To(MatchError("hierarchy.order must contain only strings"))
	})
})

var _ = Describe("applyFactsString", func() {
	It("replaces placeholders with fact values", func() {
		// Verifies templated segments are substituted when facts are available.
		result, matched, err := applyFactsString("role:{{ lookup('role') }}", map[string]any{"role": "web"})
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeTrue())
		Expect(result).To(Equal("role:web"))
	})

	It("drops placeholders when facts are missing", func() {
		// Confirms missing fact keys result in empty substitutions.
		result, matched, err := applyFactsString("env:{{ lookup('unknown') }}", map[string]any{})
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeFalse())
		Expect(result).To(Equal("env:"))
	})

	It("Should support gjson lookups", func() {
		result, matched, err := applyFactsString("{{ lookup('node.fqdn') }}", map[string]any{"node": map[string]any{"fqdn": "example.com"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeTrue())
		Expect(result).To(Equal("example.com"))

		result, matched, err = applyFactsString("{{ lookup('node.foo') }}", map[string]any{"node": map[string]any{"fqdn": "example.com"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeFalse())
		Expect(result).To(Equal(""))
	})
})

var _ = Describe("expandExprValuesRecursively", func() {
	It("expands expr placeholders in string values", func() {
		// Verifies that string values with {{ ... }} placeholders are properly expanded.
		facts := map[string]any{"env": "production", "port": 8080}
		result, err := expandExprValuesRecursively("Environment: {{ lookup('env') }}", facts)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("Environment: production"))
	})

	It("returns non-string primitives unchanged", func() {
		// Ensures that integers, booleans, and floats pass through without modification.
		facts := map[string]any{}

		intResult, err := expandExprValuesRecursively(42, facts)
		Expect(err).NotTo(HaveOccurred())
		Expect(intResult).To(Equal(42))

		boolResult, err := expandExprValuesRecursively(true, facts)
		Expect(err).NotTo(HaveOccurred())
		Expect(boolResult).To(Equal(true))

		floatResult, err := expandExprValuesRecursively(3.14, facts)
		Expect(err).NotTo(HaveOccurred())
		Expect(floatResult).To(Equal(3.14))
	})

	It("recursively expands strings in maps", func() {
		// Validates that nested map values are processed recursively.
		facts := map[string]any{"hostname": "web01", "env": "prod"}
		input := map[string]any{
			"host": "{{ lookup('hostname') }}",
			"config": map[string]any{
				"environment": "{{ lookup('env') }}",
				"port":        8080,
			},
		}

		result, err := expandExprValuesRecursively(input, facts)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"host": "web01",
			"config": map[string]any{
				"environment": "prod",
				"port":        8080,
			},
		}))
	})

	It("recursively expands strings in slices", func() {
		// Confirms that slice elements are expanded recursively.
		facts := map[string]any{"prefix": "/var/log"}
		input := []any{
			"{{ lookup('prefix') }}/app.log",
			"{{ lookup('prefix') }}/error.log",
			42,
		}

		result, err := expandExprValuesRecursively(input, facts)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]any{
			"/var/log/app.log",
			"/var/log/error.log",
			42,
		}))
	})

	It("handles deeply nested structures", func() {
		// Tests processing of complex nested maps and slices.
		facts := map[string]any{"region": "us-east", "tier": "frontend"}
		input := map[string]any{
			"metadata": map[string]any{
				"region": "{{ lookup('region') }}",
				"tags": []any{
					"{{ lookup('tier') }}",
					"production",
				},
			},
			"instances": []any{
				map[string]any{
					"name": "server-{{ lookup('tier') }}-01",
					"port": 8080,
				},
			},
		}

		result, err := expandExprValuesRecursively(input, facts)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"metadata": map[string]any{
				"region": "us-east",
				"tags": []any{
					"frontend",
					"production",
				},
			},
			"instances": []any{
				map[string]any{
					"name": "server-frontend-01",
					"port": 8080,
				},
			},
		}))
	})

	It("returns an error when expr evaluation fails", func() {
		// Ensures that invalid expressions propagate errors correctly.
		facts := map[string]any{}
		input := map[string]any{
			"invalid": "{{ undefined_function() }}",
		}

		_, err := expandExprValuesRecursively(input, facts)
		Expect(err).To(HaveOccurred())
	})

	It("supports expr operations in placeholders", func() {
		// Verifies that expr operations like filters work within placeholders.
		facts := map[string]any{"role": "WEB"}
		input := map[string]any{
			"role": "{{ lookup('role') | lower() }}",
		}

		result, err := expandExprValuesRecursively(input, facts)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"role": "web",
		}))
	})

	It("handles empty maps and slices", func() {
		// Confirms that empty containers are processed without error.
		facts := map[string]any{}

		emptyMap, err := expandExprValuesRecursively(map[string]any{}, facts)
		Expect(err).NotTo(HaveOccurred())
		Expect(emptyMap).To(Equal(map[string]any{}))

		emptySlice, err := expandExprValuesRecursively([]any{}, facts)
		Expect(err).NotTo(HaveOccurred())
		Expect(emptySlice).To(Equal([]any{}))
	})
})

var _ = Describe("normalizeNumericValues", func() {
	It("coerces compatible numeric types into int", func() {
		// Ensures whole numbers within int bounds are normalized to int for consistent merging.
		normalized := normalizeNumericValues(map[string]any{
			"float":    10.0,
			"int64":    int64(20),
			"uint64":   uint64(30),
			"inBounds": []any{float64(5)},
		})

		Expect(normalized).To(Equal(map[string]any{
			"float":    10,
			"int64":    20,
			"uint64":   30,
			"inBounds": []any{5},
		}))
	})
})
