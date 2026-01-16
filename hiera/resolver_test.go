// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package hiera

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model/modelmocks"
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

var _ = Describe("ResolveKeyValue", func() {
	var (
		ctrl    *gomock.Controller
		mockMgr *modelmocks.MockManager
		mockJS  *modelmocks.MockJetStream
		mockKV  *modelmocks.MockKeyValue
		mockLog *modelmocks.MockLogger
		ctx     context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockMgr = modelmocks.NewMockManager(ctrl)
		mockJS = modelmocks.NewMockJetStream(ctrl)
		mockKV = modelmocks.NewMockKeyValue(ctrl)
		mockLog = modelmocks.NewMockLogger(ctrl)
		ctx = context.Background()

		mockLog.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("returns an error when bucket is empty", func() {
		_, err := ResolveKeyValue(ctx, mockMgr, "", "key", nil, DefaultOptions, mockLog)
		Expect(err).To(MatchError("bucket name is required for kv hiera data source"))
	})

	It("returns an error when key is empty", func() {
		_, err := ResolveKeyValue(ctx, mockMgr, "bucket", "", nil, DefaultOptions, mockLog)
		Expect(err).To(MatchError("key is required for kv hiera data source"))
	})

	It("returns an error when JetStream fails", func() {
		jsErr := errors.New("jetstream unavailable")
		mockMgr.EXPECT().JetStream().Return(nil, jsErr)

		_, err := ResolveKeyValue(ctx, mockMgr, "bucket", "key", nil, DefaultOptions, mockLog)
		Expect(err).To(MatchError(jsErr))
	})

	It("returns an error when bucket does not exist", func() {
		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().KeyValue(ctx, "missing-bucket").Return(nil, jetstream.ErrBucketNotFound)

		_, err := ResolveKeyValue(ctx, mockMgr, "missing-bucket", "key", nil, DefaultOptions, mockLog)
		Expect(err).To(MatchError(jetstream.ErrBucketNotFound))
	})

	It("returns an error when key does not exist", func() {
		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().KeyValue(ctx, "bucket").Return(mockKV, nil)
		mockKV.EXPECT().Get(ctx, "missing-key").Return(nil, jetstream.ErrKeyNotFound)

		_, err := ResolveKeyValue(ctx, mockMgr, "bucket", "missing-key", nil, DefaultOptions, mockLog)
		Expect(err).To(MatchError(jetstream.ErrKeyNotFound))
	})

	It("returns an error when the entry is not a put operation", func() {
		mockEntry := modelmocks.NewMockKeyValueEntry(ctrl)
		mockEntry.EXPECT().Operation().Return(jetstream.KeyValueDelete)

		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().KeyValue(ctx, "bucket").Return(mockKV, nil)
		mockKV.EXPECT().Get(ctx, "deleted-key").Return(mockEntry, nil)

		_, err := ResolveKeyValue(ctx, mockMgr, "bucket", "deleted-key", nil, DefaultOptions, mockLog)
		Expect(err).To(MatchError("kv bucket#deleted-key is not a put operation"))
	})

	It("returns an error when the entry value is empty", func() {
		mockEntry := modelmocks.NewMockKeyValueEntry(ctrl)
		mockEntry.EXPECT().Operation().Return(jetstream.KeyValuePut)
		mockEntry.EXPECT().Value().Return([]byte{})

		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().KeyValue(ctx, "bucket").Return(mockKV, nil)
		mockKV.EXPECT().Get(ctx, "empty-key").Return(mockEntry, nil)

		_, err := ResolveKeyValue(ctx, mockMgr, "bucket", "empty-key", nil, DefaultOptions, mockLog)
		Expect(err).To(MatchError("kv bucket#empty-key is empty"))
	})

	It("returns an error when JSON parsing fails", func() {
		mockEntry := modelmocks.NewMockKeyValueEntry(ctrl)
		mockEntry.EXPECT().Operation().Return(jetstream.KeyValuePut)
		mockEntry.EXPECT().Value().Return([]byte(`{invalid json`))

		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().KeyValue(ctx, "bucket").Return(mockKV, nil)
		mockKV.EXPECT().Get(ctx, "bad-json").Return(mockEntry, nil)

		_, err := ResolveKeyValue(ctx, mockMgr, "bucket", "bad-json", nil, DefaultOptions, mockLog)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to parse JSON from kv bucket#bad-json"))
	})

	It("resolves JSON data from KV successfully", func() {
		jsonData := []byte(`{
			"hierarchy": {
				"order": ["default"],
				"merge": "first"
			},
			"data": {
				"log_level": "INFO",
				"port": 8080
			}
		}`)

		mockEntry := modelmocks.NewMockKeyValueEntry(ctrl)
		mockEntry.EXPECT().Operation().Return(jetstream.KeyValuePut)
		mockEntry.EXPECT().Value().Return(jsonData)

		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().KeyValue(ctx, "config").Return(mockKV, nil)
		mockKV.EXPECT().Get(ctx, "app.settings").Return(mockEntry, nil)

		result, err := ResolveKeyValue(ctx, mockMgr, "config", "app.settings", map[string]any{}, DefaultOptions, mockLog)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"log_level": "INFO",
			"port":      8080,
		}))
	})

	It("resolves YAML data from KV successfully", func() {
		yamlData := []byte(`
hierarchy:
  order:
    - default
  merge: first
data:
  log_level: DEBUG
  timeout: 30
`)

		mockEntry := modelmocks.NewMockKeyValueEntry(ctrl)
		mockEntry.EXPECT().Operation().Return(jetstream.KeyValuePut)
		mockEntry.EXPECT().Value().Return(yamlData)

		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().KeyValue(ctx, "config").Return(mockKV, nil)
		mockKV.EXPECT().Get(ctx, "app.yaml").Return(mockEntry, nil)

		result, err := ResolveKeyValue(ctx, mockMgr, "config", "app.yaml", map[string]any{}, DefaultOptions, mockLog)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"log_level": "DEBUG",
			"timeout":   30,
		}))
	})

	It("resolves data with facts and hierarchy overrides", func() {
		jsonData := []byte(`{
			"hierarchy": {
				"order": ["env:{{ lookup('facts.env') }}", "default"],
				"merge": "deep"
			},
			"data": {
				"log_level": "INFO",
				"retries": 3
			},
			"overrides": {
				"env:prod": {
					"log_level": "WARN",
					"retries": 5
				}
			}
		}`)

		mockEntry := modelmocks.NewMockKeyValueEntry(ctrl)
		mockEntry.EXPECT().Operation().Return(jetstream.KeyValuePut)
		mockEntry.EXPECT().Value().Return(jsonData)

		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().KeyValue(ctx, "config").Return(mockKV, nil)
		mockKV.EXPECT().Get(ctx, "app.config").Return(mockEntry, nil)

		facts := map[string]any{"env": "prod"}
		result, err := ResolveKeyValue(ctx, mockMgr, "config", "app.config", facts, DefaultOptions, mockLog)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"log_level": "WARN",
			"retries":   5,
		}))
	})

	It("handles KeyValue access errors", func() {
		accessErr := errors.New("connection timeout")
		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().KeyValue(ctx, "bucket").Return(nil, accessErr)

		_, err := ResolveKeyValue(ctx, mockMgr, "bucket", "key", nil, DefaultOptions, mockLog)
		Expect(err).To(MatchError(accessErr))
	})

	It("handles Get errors", func() {
		getErr := errors.New("read timeout")
		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().KeyValue(ctx, "bucket").Return(mockKV, nil)
		mockKV.EXPECT().Get(ctx, "key").Return(nil, getErr)

		_, err := ResolveKeyValue(ctx, mockMgr, "bucket", "key", nil, DefaultOptions, mockLog)
		Expect(err).To(MatchError(getErr))
	})
})

var _ = Describe("ResolveUrl", func() {
	var (
		ctrl    *gomock.Controller
		mockMgr *modelmocks.MockManager
		mockJS  *modelmocks.MockJetStream
		mockKV  *modelmocks.MockKeyValue
		mockLog *modelmocks.MockLogger
		ctx     context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockMgr = modelmocks.NewMockManager(ctrl)
		mockJS = modelmocks.NewMockJetStream(ctrl)
		mockKV = modelmocks.NewMockKeyValue(ctrl)
		mockLog = modelmocks.NewMockLogger(ctrl)
		ctx = context.Background()

		mockLog.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("returns an error when source is empty", func() {
		_, err := ResolveUrl(ctx, "", mockMgr, nil, DefaultOptions, mockLog)
		Expect(err).To(MatchError("source is required"))
	})

	It("returns an error for unsupported schemes", func() {
		_, err := ResolveUrl(ctx, "http://example.com/data", mockMgr, nil, DefaultOptions, mockLog)
		Expect(err).To(MatchError("unsupported hiera data source: http://example.com/data"))
	})

	It("resolves kv:// URLs", func() {
		jsonData := []byte(`{
			"hierarchy": {
				"order": ["default"],
				"merge": "first"
			},
			"data": {
				"setting": "value"
			}
		}`)

		mockEntry := modelmocks.NewMockKeyValueEntry(ctrl)
		mockEntry.EXPECT().Operation().Return(jetstream.KeyValuePut)
		mockEntry.EXPECT().Value().Return(jsonData)

		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().KeyValue(ctx, "mybucket").Return(mockKV, nil)
		mockKV.EXPECT().Get(ctx, "mykey").Return(mockEntry, nil)

		result, err := ResolveUrl(ctx, "kv://mybucket/mykey", mockMgr, map[string]any{}, DefaultOptions, mockLog)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"setting": "value",
		}))
	})

	It("resolves obj:// URLs", func() {
		yamlData := []byte(`
hierarchy:
  order:
    - default
  merge: first
data:
  setting: from_object_store
`)

		mockObjStore := modelmocks.NewMockObjectStore(ctrl)

		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().ObjectStore(ctx, "databucket").Return(mockObjStore, nil)
		mockObjStore.EXPECT().GetBytes(ctx, "data.yaml").Return(yamlData, nil)

		result, err := ResolveUrl(ctx, "obj://databucket/data.yaml", mockMgr, map[string]any{}, DefaultOptions, mockLog)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"setting": "from_object_store",
		}))
	})
})

var _ = Describe("ResolveFile", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		ctx     context.Context
		tempDir string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		ctx = context.Background()

		mockLog.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

		var err error
		tempDir, err = os.MkdirTemp("", "hiera-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
		os.RemoveAll(tempDir)
	})

	It("resolves YAML files", func() {
		yamlContent := `
hierarchy:
  order:
    - default
  merge: first
data:
  setting: from_yaml
  count: 42
`
		filePath := tempDir + "/agent.yaml"
		err := os.WriteFile(filePath, []byte(yamlContent), 0644)
		Expect(err).NotTo(HaveOccurred())

		result, err := ResolveFile(ctx, filePath, map[string]any{}, DefaultOptions, mockLog)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"setting": "from_yaml",
			"count":   42,
		}))
	})

	It("resolves JSON files", func() {
		jsonContent := `{
			"hierarchy": {
				"order": ["default"],
				"merge": "first"
			},
			"data": {
				"setting": "from_json",
				"enabled": true
			}
		}`
		filePath := tempDir + "/config.json"
		err := os.WriteFile(filePath, []byte(jsonContent), 0644)
		Expect(err).NotTo(HaveOccurred())

		result, err := ResolveFile(ctx, filePath, map[string]any{}, DefaultOptions, mockLog)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"setting": "from_json",
			"enabled": true,
		}))
	})

	It("resolves files with facts and overrides", func() {
		yamlContent := `
hierarchy:
  order:
    - env:{{ lookup('facts.env') }}
    - default
  merge: deep
data:
  log_level: INFO
overrides:
  env:production:
    log_level: WARN
`
		filePath := tempDir + "/agent.yaml"
		err := os.WriteFile(filePath, []byte(yamlContent), 0644)
		Expect(err).NotTo(HaveOccurred())

		facts := map[string]any{"env": "production"}
		result, err := ResolveFile(ctx, filePath, facts, DefaultOptions, mockLog)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"log_level": "WARN",
		}))
	})
})
