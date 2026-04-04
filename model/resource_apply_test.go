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

var _ = Describe("ApplyResourceProperties", func() {
	Describe("Validate", func() {
		DescribeTable("validation tests",
			func(name, ensure string, noop bool, errorText string) {
				prop := &ApplyResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   name,
						Ensure: ensure,
					},
					Noop: noop,
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
			Entry("valid absolute path", "/etc/ccm/manifests/web.yaml", "present", false, ""),
			Entry("valid relative path", "manifests/web.yaml", "present", false, ""),
			Entry("valid with noop", "/etc/ccm/manifests/web.yaml", "present", true, ""),

			// Name validation
			Entry("empty name", "", "present", false, "name"),
			Entry("http URL", "http://example.com/manifest.yaml", "present", false, "file path, not a URL"),
			Entry("https URL", "https://example.com/manifest.yaml", "present", false, "file path, not a URL"),
			Entry("ftp URL", "ftp://example.com/manifest.yaml", "present", false, "file path, not a URL"),

			// Ensure validation
			Entry("empty ensure", "/etc/ccm/manifests/web.yaml", "", false, "ensure"),
			Entry("invalid ensure absent", "/etc/ccm/manifests/web.yaml", "absent", false, "invalid ensure value"),
			Entry("invalid ensure running", "/etc/ccm/manifests/web.yaml", "running", false, "invalid ensure value"),
		)

		It("Should skip validation when SkipValidate is true", func() {
			prop := &ApplyResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:         "",
					Ensure:       "invalid",
					SkipValidate: true,
				},
			}

			err := prop.Validate()
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("ResolveTemplates", func() {
		It("Should resolve templates in name", func() {
			prop := &ApplyResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/etc/ccm/{{ Facts.role }}/manifest.yaml",
					Ensure: EnsurePresent,
				},
			}

			env := &templates.Env{
				Facts: map[string]any{
					"role": "webserver",
				},
			}

			err := prop.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Name).To(Equal("/etc/ccm/webserver/manifest.yaml"))
		})

		It("Should handle non-template strings", func() {
			prop := &ApplyResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/etc/ccm/manifest.yaml",
					Ensure: EnsurePresent,
				},
			}

			env := &templates.Env{
				Facts: map[string]any{},
			}

			err := prop.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Name).To(Equal("/etc/ccm/manifest.yaml"))
		})

		It("Should return error for invalid template", func() {
			prop := &ApplyResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "{{ invalid syntax }}",
					Ensure: EnsurePresent,
				},
			}

			env := &templates.Env{Facts: map[string]any{}}

			err := prop.ResolveTemplates(env)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ToYamlManifest", func() {
		It("Should marshal to YAML correctly", func() {
			prop := &ApplyResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/etc/ccm/manifest.yaml",
					Ensure: EnsurePresent,
				},
				Noop:            true,
				HealthCheckOnly: true,
				AllowApply:      true,
				Data:            map[string]any{"key": "value"},
			}

			raw, err := prop.ToYamlManifest()
			Expect(err).ToNot(HaveOccurred())
			Expect(raw).ToNot(BeNil())

			yamlStr := string(raw)
			Expect(yamlStr).To(ContainSubstring("name: /etc/ccm/manifest.yaml"))
			Expect(yamlStr).To(ContainSubstring("ensure: present"))
			Expect(yamlStr).To(ContainSubstring("noop: true"))
			Expect(yamlStr).To(ContainSubstring("health_check_only: true"))
			Expect(yamlStr).To(ContainSubstring("allow_apply: true"))
			Expect(yamlStr).To(ContainSubstring("key: value"))
		})

		It("Should omit empty optional fields", func() {
			prop := &ApplyResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/etc/ccm/manifest.yaml",
					Ensure: EnsurePresent,
				},
			}

			raw, err := prop.ToYamlManifest()
			Expect(err).ToNot(HaveOccurred())

			yamlStr := string(raw)
			Expect(yamlStr).ToNot(ContainSubstring("noop:"))
			Expect(yamlStr).ToNot(ContainSubstring("health_check_only:"))
			Expect(yamlStr).ToNot(ContainSubstring("allow_apply:"))
			Expect(yamlStr).ToNot(ContainSubstring("data:"))
		})
	})

	Describe("NewApplyResourcePropertiesFromYaml", func() {
		It("Should unmarshal from YAML correctly", func() {
			yamlData := `
name: /etc/ccm/manifest.yaml
ensure: present
noop: true
health_check_only: true
allow_apply: true
data:
  key: value
`
			props, err := NewApplyResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ApplyResourceProperties)
			Expect(prop.Name).To(Equal("/etc/ccm/manifest.yaml"))
			Expect(prop.Ensure).To(Equal(EnsurePresent))
			Expect(prop.Noop).To(BeTrue())
			Expect(prop.HealthCheckOnly).To(BeTrue())
			Expect(prop.AllowApply).To(BeTrue())
			Expect(prop.Data).To(HaveKeyWithValue("key", "value"))
			Expect(prop.Type).To(Equal(ApplyTypeName))
		})

		It("Should default ensure to present", func() {
			yamlData := `
name: /etc/ccm/manifest.yaml
`
			props, err := NewApplyResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ApplyResourceProperties)
			Expect(prop.Ensure).To(Equal(EnsurePresent))
		})

		It("Should return error for invalid YAML", func() {
			invalidYaml := `
name: [invalid
`
			_, err := NewApplyResourcePropertiesFromYaml(yaml.RawMessage(invalidYaml))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("CommonProperties", func() {
		It("Should return CommonProperties correctly", func() {
			prop := &ApplyResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/etc/ccm/manifest.yaml",
					Ensure: EnsurePresent,
				},
			}

			common := prop.CommonProperties()
			Expect(common).ToNot(BeNil())
			Expect(common.Name).To(Equal("/etc/ccm/manifest.yaml"))
			Expect(common.Ensure).To(Equal(EnsurePresent))
		})
	})

	Describe("Constants", func() {
		It("Should have correct values", func() {
			Expect(ResourceStatusApplyProtocol).To(Equal("io.choria.ccm.v1.resource.apply.state"))
			Expect(ApplyTypeName).To(Equal("apply"))
		})
	})
})
