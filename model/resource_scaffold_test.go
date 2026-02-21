// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"github.com/goccy/go-yaml"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/choria-io/ccm/templates"
)

var _ = Describe("ScaffoldResourceProperties", func() {
	Describe("Validate", func() {
		DescribeTable("validation tests",
			func(name, ensure, source string, engine ScaffoldResourceEngine, post []map[string]string, errorText string) {
				prop := &ScaffoldResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   name,
						Ensure: ensure,
					},
					Source: source,
					Engine: engine,
					Post:   post,
				}

				err := prop.Validate()

				if errorText != "" {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(errorText))
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},

			// Valid cases
			Entry("valid scaffold with go engine", "/opt/app/scaffold", "present", "https://example.com/scaffold.tar.gz", ScaffoldEngineGo, nil, ""),
			Entry("valid scaffold with jet engine", "/opt/app/scaffold", "present", "https://example.com/scaffold.tar.gz", ScaffoldEngineJet, nil, ""),
			Entry("valid scaffold with absent ensure", "/opt/app/scaffold", "absent", "https://example.com/scaffold.tar.gz", ScaffoldEngineGo, nil, ""),
			Entry("valid scaffold with post", "/opt/app/scaffold", "present", "https://example.com/scaffold.tar.gz", ScaffoldEngineGo, []map[string]string{{"key1": "value1"}}, ""),

			// Name validation
			Entry("empty name", "", "present", "https://example.com/scaffold.tar.gz", ScaffoldEngineGo, nil, "name"),
			Entry("relative name", "relative/path", "present", "https://example.com/scaffold.tar.gz", ScaffoldEngineGo, nil, "absolute path"),
			Entry("path with ..", "/opt/../etc/scaffold", "present", "https://example.com/scaffold.tar.gz", ScaffoldEngineGo, nil, "canonical"),
			Entry("path with .", "/opt/./scaffold", "present", "https://example.com/scaffold.tar.gz", ScaffoldEngineGo, nil, "canonical"),

			// Ensure validation
			Entry("empty ensure", "/opt/app/scaffold", "", "https://example.com/scaffold.tar.gz", ScaffoldEngineGo, nil, "ensure"),
			Entry("invalid ensure value", "/opt/app/scaffold", "latest", "https://example.com/scaffold.tar.gz", ScaffoldEngineGo, nil, "invalid ensure value"),
			Entry("invalid ensure running", "/opt/app/scaffold", "running", "https://example.com/scaffold.tar.gz", ScaffoldEngineGo, nil, "invalid ensure value"),

			// Source validation
			Entry("empty source", "/opt/app/scaffold", "present", "", ScaffoldEngineGo, nil, "source cannot be empty"),

			// Engine validation
			Entry("invalid engine", "/opt/app/scaffold", "present", "https://example.com/scaffold.tar.gz", ScaffoldResourceEngine("invalid"), nil, "engine must be one of"),

			// Post validation
			Entry("post with empty value", "/opt/app/scaffold", "present", "https://example.com/scaffold.tar.gz", ScaffoldEngineGo, []map[string]string{{"key1": ""}}, "post value for key"),
		)

		DescribeTable("legitimate scaffold paths",
			func(name, source string, engine ScaffoldResourceEngine) {
				prop := &ScaffoldResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   name,
						Ensure: EnsurePresent,
					},
					Source: source,
					Engine: engine,
				}

				err := prop.Validate()
				Expect(err).ToNot(HaveOccurred())
			},

			Entry("simple scaffold", "/opt/app", "https://example.com/scaffold.tar.gz", ScaffoldEngineGo),
			Entry("config scaffold", "/etc/myapp", "https://example.com/config.tar.gz", ScaffoldEngineJet),
			Entry("deep path", "/opt/company/app/config/templates", "https://cdn.example.com/templates.zip", ScaffoldEngineGo),
			Entry("var path", "/var/lib/myapp/scaffold", "file:///local/scaffold", ScaffoldEngineJet),
		)

		It("Should skip validation when SkipValidate is true", func() {
			prop := &ScaffoldResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:         "", // Invalid
					Ensure:       "invalid",
					SkipValidate: true,
				},
				Source: "", // Invalid
				Engine: ScaffoldResourceEngine("invalid"),
			}

			err := prop.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should validate post with multiple valid entries", func() {
			prop := &ScaffoldResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: EnsurePresent,
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: ScaffoldEngineGo,
				Post: []map[string]string{
					{"key1": "value1"},
					{"key2": "value2"},
					{"key3": "value3"},
				},
			}

			err := prop.Validate()
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("ResolveTemplates", func() {
		It("Should resolve templates in source field", func() {
			prop := &ScaffoldResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/opt/{{ Facts.appname }}/scaffold",
					Ensure: EnsurePresent,
				},
				Source: "https://example.com/{{ Facts.version }}/scaffold.tar.gz",
				Engine: ScaffoldEngineGo,
			}

			env := &templates.Env{
				Facts: map[string]any{
					"appname": "myapp",
					"version": "v1.0.0",
				},
			}

			err := prop.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Name).To(Equal("/opt/myapp/scaffold"))
			Expect(prop.Source).To(Equal("https://example.com/v1.0.0/scaffold.tar.gz"))
		})

		It("Should handle non-template strings", func() {
			prop := &ScaffoldResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: EnsurePresent,
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: ScaffoldEngineGo,
			}

			env := &templates.Env{
				Facts: map[string]any{},
			}

			err := prop.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Name).To(Equal("/opt/app/scaffold"))
			Expect(prop.Source).To(Equal("https://example.com/scaffold.tar.gz"))
		})

		It("Should return error for invalid template in source", func() {
			prop := &ScaffoldResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: EnsurePresent,
				},
				Source: "{{ invalid syntax }}",
				Engine: ScaffoldEngineGo,
			}

			env := &templates.Env{Facts: map[string]any{}}

			err := prop.ResolveTemplates(env)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ToYamlManifest", func() {
		It("Should marshal to YAML correctly", func() {
			prop := &ScaffoldResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: EnsurePresent,
				},
				Source:         "https://example.com/scaffold.tar.gz",
				Engine:         ScaffoldEngineGo,
				SkipEmpty:      true,
				LeftDelimiter:  "<%",
				RightDelimiter: "%>",
				Purge:          true,
				Post:           []map[string]string{{"reload": "systemctl reload app"}},
			}

			raw, err := prop.ToYamlManifest()
			Expect(err).ToNot(HaveOccurred())
			Expect(raw).ToNot(BeNil())

			yamlStr := string(raw)
			Expect(yamlStr).To(ContainSubstring("name: /opt/app/scaffold"))
			Expect(yamlStr).To(ContainSubstring("ensure: present"))
			Expect(yamlStr).To(ContainSubstring("source: https://example.com/scaffold.tar.gz"))
			Expect(yamlStr).To(ContainSubstring("engine: go"))
			Expect(yamlStr).To(ContainSubstring("skip_empty: true"))
			Expect(yamlStr).To(ContainSubstring("left_delimiter: <%"))
			Expect(yamlStr).To(ContainSubstring("right_delimiter:"))
			Expect(yamlStr).To(ContainSubstring("purge: true"))
			Expect(yamlStr).To(ContainSubstring("reload: systemctl reload app"))
		})

		It("Should omit empty optional fields", func() {
			prop := &ScaffoldResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: EnsurePresent,
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: ScaffoldEngineGo,
			}

			raw, err := prop.ToYamlManifest()
			Expect(err).ToNot(HaveOccurred())

			yamlStr := string(raw)
			Expect(yamlStr).ToNot(ContainSubstring("skip_empty:"))
			Expect(yamlStr).ToNot(ContainSubstring("left_delimiter:"))
			Expect(yamlStr).ToNot(ContainSubstring("right_delimiter:"))
			Expect(yamlStr).ToNot(ContainSubstring("purge:"))
			Expect(yamlStr).ToNot(ContainSubstring("post:"))
		})
	})

	Describe("NewScaffoldResourcePropertiesFromYaml", func() {
		It("Should unmarshal from YAML correctly", func() {
			yamlData := `
name: /opt/app/scaffold
ensure: present
source: https://example.com/scaffold.tar.gz
engine: go
skip_empty: true
left_delimiter: "<%"
right_delimiter: "%>"
purge: true
post:
  - reload: systemctl reload app
`
			props, err := NewScaffoldResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ScaffoldResourceProperties)
			Expect(prop.Name).To(Equal("/opt/app/scaffold"))
			Expect(prop.Ensure).To(Equal(EnsurePresent))
			Expect(prop.Source).To(Equal("https://example.com/scaffold.tar.gz"))
			Expect(prop.Engine).To(Equal(ScaffoldEngineGo))
			Expect(prop.SkipEmpty).To(BeTrue())
			Expect(prop.LeftDelimiter).To(Equal("<%"))
			Expect(prop.RightDelimiter).To(Equal("%>"))
			Expect(prop.Purge).To(BeTrue())
			Expect(prop.Post).To(HaveLen(1))
			Expect(prop.Post[0]).To(HaveKeyWithValue("reload", "systemctl reload app"))
			Expect(prop.Type).To(Equal(ScaffoldTypeName))
		})

		It("Should unmarshal with jet engine", func() {
			yamlData := `
name: /opt/app/scaffold
ensure: present
source: https://example.com/scaffold.tar.gz
engine: jet
`
			props, err := NewScaffoldResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ScaffoldResourceProperties)
			Expect(prop.Engine).To(Equal(ScaffoldEngineJet))
		})

		It("Should return error for invalid YAML", func() {
			invalidYaml := `
name: [invalid
`
			_, err := NewScaffoldResourcePropertiesFromYaml(yaml.RawMessage(invalidYaml))
			Expect(err).To(HaveOccurred())
		})

		It("Should set Type to scaffold", func() {
			yamlData := `
name: /opt/app/scaffold
ensure: present
source: https://example.com/scaffold.tar.gz
engine: go
`
			props, err := NewScaffoldResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ScaffoldResourceProperties)
			Expect(prop.Type).To(Equal(ScaffoldTypeName))
		})
	})

	Describe("ScaffoldState", func() {
		It("Should have correct structure", func() {
			state := &ScaffoldState{
				CommonResourceState: NewCommonResourceState(ResourceStatusScaffoldProtocol, ScaffoldTypeName, "/opt/app/scaffold", EnsurePresent),
				Metadata: &ScaffoldMetadata{
					Name:     "/opt/app/scaffold",
					Provider: "posix",
					Changed:  []string{"file1.txt", "file2.txt"},
					Purged:   []string{"old.txt"},
					Stable:   []string{"unchanged.txt"},
					Engine:   "go",
				},
			}

			Expect(state.Protocol).To(Equal(ResourceStatusScaffoldProtocol))
			Expect(state.ResourceType).To(Equal(ScaffoldTypeName))
			Expect(state.Name).To(Equal("/opt/app/scaffold"))
			Expect(state.Ensure).To(Equal(EnsurePresent))
			Expect(state.Metadata.Name).To(Equal("/opt/app/scaffold"))
			Expect(state.Metadata.Provider).To(Equal("posix"))
			Expect(state.Metadata.Changed).To(ConsistOf("file1.txt", "file2.txt"))
			Expect(state.Metadata.Purged).To(ConsistOf("old.txt"))
			Expect(state.Metadata.Stable).To(ConsistOf("unchanged.txt"))
			Expect(state.Metadata.Engine).To(Equal(ScaffoldEngineGo))
		})

		It("Should return CommonState correctly", func() {
			state := &ScaffoldState{
				CommonResourceState: NewCommonResourceState(ResourceStatusScaffoldProtocol, ScaffoldTypeName, "/opt/app/scaffold", EnsurePresent),
			}

			common := state.CommonState()
			Expect(common).ToNot(BeNil())
			Expect(common.Name).To(Equal("/opt/app/scaffold"))
		})
	})

	Describe("ScaffoldResourceProperties CommonProperties", func() {
		It("Should return CommonProperties correctly", func() {
			prop := &ScaffoldResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: EnsurePresent,
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: ScaffoldEngineGo,
			}

			common := prop.CommonProperties()
			Expect(common).ToNot(BeNil())
			Expect(common.Name).To(Equal("/opt/app/scaffold"))
			Expect(common.Ensure).To(Equal(EnsurePresent))
		})
	})

	Describe("Constants", func() {
		It("Should have correct values", func() {
			Expect(ResourceStatusScaffoldProtocol).To(Equal("io.choria.ccm.v1.resource.scaffold.state"))
			Expect(ScaffoldTypeName).To(Equal("scaffold"))
			Expect(ScaffoldEngineGo).To(Equal(ScaffoldResourceEngine("go")))
			Expect(ScaffoldEngineJet).To(Equal(ScaffoldResourceEngine("jet")))
		})
	})
})
