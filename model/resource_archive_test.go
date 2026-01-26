// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"github.com/goccy/go-yaml"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/choria-io/ccm/templates"
)

var _ = Describe("ArchiveResourceProperties", func() {
	Describe("Validate", func() {
		DescribeTable("validation tests",
			func(name, ensure, url, owner, group, creates, extractParent, errorText string) {
				prop := &ArchiveResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   name,
						Ensure: ensure,
					},
					Url:           url,
					Owner:         owner,
					Group:         group,
					Creates:       creates,
					ExtractParent: extractParent,
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
			Entry("valid archive properties", "/tmp/archive.tar.gz", "present", "https://example.com/archive.tar.gz", "root", "root", "", "", ""),
			Entry("valid archive with absent ensure", "/tmp/archive.tar.gz", "absent", "https://example.com/archive.tar.gz", "root", "root", "", "", ""),
			Entry("valid archive with creates", "/tmp/archive.tar.gz", "present", "https://example.com/archive.tar.gz", "root", "root", "/opt/app/bin", "", ""),
			Entry("valid archive with extract_parent", "/tmp/archive.tar.gz", "present", "https://example.com/archive.tar.gz", "root", "root", "", "/opt", ""),
			Entry("valid archive with all options", "/tmp/archive.tar.gz", "present", "https://example.com/archive.tar.gz", "app", "app", "/opt/app/bin", "/opt", ""),

			// Name validation
			Entry("empty name", "", "present", "https://example.com/archive.tar.gz", "root", "root", "", "", "name"),
			Entry("path with ..", "/tmp/../etc/archive.tar.gz", "present", "https://example.com/archive.tar.gz", "root", "root", "", "", "absolute"),
			Entry("path with .", "/tmp/./archive.tar.gz", "present", "https://example.com/archive.tar.gz", "root", "root", "", "", "absolute"),

			// URL validation
			Entry("empty url", "/tmp/archive.tar.gz", "present", "", "root", "root", "", "", "url cannot be empty"),
			Entry("relative url", "/tmp/archive.tar.gz", "present", "/path/to/archive.tar.gz", "root", "root", "", "", "url must be absolute"),
			Entry("url without scheme", "/tmp/archive.tar.gz", "present", "example.com/archive.tar.gz", "root", "root", "", "", "url must be absolute"),
			Entry("url without filename", "/tmp/archive.tar.gz", "present", "https://example.com/", "root", "root", "", "", "url must have a filename"),
			Entry("url with .exe extension", "/tmp/archive.tar.gz", "present", "https://example.com/archive.exe", "root", "root", "", "", "url filename must end in .zip, .tar.gz, .tgz, or .tar"),
			Entry("url with .tar.xz extension", "/tmp/archive.tar.gz", "present", "https://example.com/archive.tar.xz", "root", "root", "", "", "url filename must end in .zip, .tar.gz, .tgz, or .tar"),

			// Name extension validation
			Entry("name with invalid extension", "/tmp/archive.bin", "present", "https://example.com/archive.tar.gz", "root", "root", "", "", "name must end in .zip, .tar.gz, .tgz, or .tar"),

			// Archive type mismatch validation
			Entry("tar.gz url with zip name", "/tmp/archive.zip", "present", "https://example.com/archive.tar.gz", "root", "root", "", "", "url and name must have the same archive type"),
			Entry("zip url with tar.gz name", "/tmp/archive.tar.gz", "present", "https://example.com/archive.zip", "root", "root", "", "", "url and name must have the same archive type"),
			Entry("tar url with tar.gz name", "/tmp/archive.tar.gz", "present", "https://example.com/archive.tar", "root", "root", "", "", "url and name must have the same archive type"),
			Entry("tar.gz url with tar name", "/tmp/archive.tar", "present", "https://example.com/archive.tar.gz", "root", "root", "", "", "url and name must have the same archive type"),

			// Ensure validation
			Entry("empty ensure", "/tmp/archive.tar.gz", "", "https://example.com/archive.tar.gz", "root", "root", "", "", "ensure"),
			Entry("invalid ensure value", "/tmp/archive.tar.gz", "latest", "https://example.com/archive.tar.gz", "root", "root", "", "", "invalid ensure value"),
			Entry("invalid ensure running", "/tmp/archive.tar.gz", "running", "https://example.com/archive.tar.gz", "root", "root", "", "", "invalid ensure value"),

			// Owner validation
			Entry("empty owner", "/tmp/archive.tar.gz", "present", "https://example.com/archive.tar.gz", "", "root", "", "", "owner cannot be empty"),

			// Group validation
			Entry("empty group", "/tmp/archive.tar.gz", "present", "https://example.com/archive.tar.gz", "root", "", "", "", "group cannot be empty"),

			// Creates validation
			Entry("creates path with ..", "/tmp/archive.tar.gz", "present", "https://example.com/archive.tar.gz", "root", "root", "/opt/../etc/file", "", "creates path must be absolute"),
			Entry("creates path with .", "/tmp/archive.tar.gz", "present", "https://example.com/archive.tar.gz", "root", "root", "/opt/./file", "", "creates path must be absolute"),

			// ExtractParent validation
			Entry("extract_parent path with ..", "/tmp/archive.tar.gz", "present", "https://example.com/archive.tar.gz", "root", "root", "", "/opt/../etc", "extract_parent path must be absolute"),
			Entry("extract_parent path with .", "/tmp/archive.tar.gz", "present", "https://example.com/archive.tar.gz", "root", "root", "", "/opt/./dir", "extract_parent path must be absolute"),
		)

		It("Should fail when cleanup is true but extract_parent is empty", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/archive.tar.gz",
					Ensure: EnsurePresent,
				},
				Url:     "https://example.com/archive.tar.gz",
				Owner:   "root",
				Group:   "root",
				Cleanup: true,
			}

			err := prop.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cleanup requires extract_parent"))
		})

		It("Should fail when cleanup is true but creates is empty", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/archive.tar.gz",
					Ensure: EnsurePresent,
				},
				Url:           "https://example.com/archive.tar.gz",
				Owner:         "root",
				Group:         "root",
				Cleanup:       true,
				ExtractParent: "/opt",
			}

			err := prop.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cleanup requires creates"))
		})

		It("Should pass when cleanup is true and extract_parent and creates are set", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/archive.tar.gz",
					Ensure: EnsurePresent,
				},
				Url:           "https://example.com/archive.tar.gz",
				Owner:         "root",
				Group:         "root",
				Cleanup:       true,
				ExtractParent: "/opt",
				Creates:       "/opt/app/bin",
			}

			err := prop.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		DescribeTable("legitimate archive paths",
			func(name, url, owner, group string) {
				prop := &ArchiveResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   name,
						Ensure: EnsurePresent,
					},
					Url:   url,
					Owner: owner,
					Group: group,
				}

				err := prop.Validate()
				Expect(err).ToNot(HaveOccurred())
			},

			Entry("simple archive", "/tmp/archive.tar.gz", "https://example.com/archive.tar.gz", "root", "root"),
			Entry("zip archive", "/opt/downloads/app.zip", "https://releases.example.com/v1.0/app.zip", "app", "app"),
			Entry("tgz archive", "/var/cache/downloads/package.tgz", "https://cdn.example.com/package.tgz", "nobody", "nogroup"),
			Entry("tar archive", "/tmp/archive.tar", "https://example.com/archive.tar", "root", "root"),
			Entry("url with query params", "/tmp/archive.tar.gz", "https://example.com/download/archive.tar.gz?token=abc123", "root", "root"),
			Entry("url with port", "/tmp/archive.tar.gz", "https://example.com:8443/archive.tar.gz", "root", "root"),
			Entry("http url", "/tmp/archive.zip", "http://internal.example.com/archive.zip", "root", "root"),
			Entry("uppercase extension", "/tmp/archive.tar.gz", "https://example.com/archive.TAR.GZ", "root", "root"),
			Entry("tgz url with tar.gz name", "/tmp/archive.tar.gz", "https://example.com/archive.tgz", "root", "root"),
			Entry("tar.gz url with tgz name", "/tmp/archive.tgz", "https://example.com/archive.tar.gz", "root", "root"),
		)

		It("Should skip validation when SkipValidate is true", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:         "", // Invalid
					Ensure:       "invalid",
					SkipValidate: true,
				},
				Owner: "", // Invalid
				Group: "", // Invalid
			}

			err := prop.Validate()
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("ResolveTemplates", func() {
		It("Should resolve templates in all fields", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/{{ Facts.filename }}",
					Ensure: EnsurePresent,
				},
				Url:      "https://example.com/{{ Facts.version }}/archive.tar.gz",
				Owner:    "{{ Facts.owner }}",
				Group:    "{{ Facts.group }}",
				Checksum: "{{ Facts.checksum }}",
				Creates:  "/opt/{{ Facts.appname }}/bin",
			}

			env := &templates.Env{
				Facts: map[string]any{
					"filename": "app.tar.gz",
					"version":  "v1.0.0",
					"owner":    "root",
					"group":    "wheel",
					"checksum": "abc123def456",
					"appname":  "myapp",
				},
			}

			err := prop.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Name).To(Equal("/tmp/app.tar.gz"))
			Expect(prop.Url).To(Equal("https://example.com/v1.0.0/archive.tar.gz"))
			Expect(prop.Owner).To(Equal("root"))
			Expect(prop.Group).To(Equal("wheel"))
			Expect(prop.Checksum).To(Equal("abc123def456"))
			Expect(prop.Creates).To(Equal("/opt/myapp/bin"))
		})

		It("Should handle non-template strings", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/archive.tar.gz",
					Ensure: EnsurePresent,
				},
				Url:   "https://example.com/archive.tar.gz",
				Owner: "root",
				Group: "root",
			}

			env := &templates.Env{
				Facts: map[string]any{},
			}

			err := prop.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Name).To(Equal("/tmp/archive.tar.gz"))
			Expect(prop.Url).To(Equal("https://example.com/archive.tar.gz"))
			Expect(prop.Owner).To(Equal("root"))
			Expect(prop.Group).To(Equal("root"))
		})

		It("Should not resolve empty checksum", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/archive.tar.gz",
					Ensure: EnsurePresent,
				},
				Url:      "https://example.com/archive.tar.gz",
				Owner:    "root",
				Group:    "root",
				Checksum: "",
			}

			env := &templates.Env{Facts: map[string]any{}}

			err := prop.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Checksum).To(Equal(""))
		})

		It("Should not resolve empty creates", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/archive.tar.gz",
					Ensure: EnsurePresent,
				},
				Url:     "https://example.com/archive.tar.gz",
				Owner:   "root",
				Group:   "root",
				Creates: "",
			}

			env := &templates.Env{Facts: map[string]any{}}

			err := prop.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Creates).To(Equal(""))
		})

		It("Should return error for invalid template in url", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/archive.tar.gz",
					Ensure: EnsurePresent,
				},
				Url:   "{{ invalid syntax }}",
				Owner: "root",
				Group: "root",
			}

			env := &templates.Env{Facts: map[string]any{}}

			err := prop.ResolveTemplates(env)
			Expect(err).To(HaveOccurred())
		})

		It("Should return error for invalid template in owner", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/archive.tar.gz",
					Ensure: EnsurePresent,
				},
				Url:   "https://example.com/archive.tar.gz",
				Owner: "{{ invalid syntax }}",
				Group: "root",
			}

			env := &templates.Env{Facts: map[string]any{}}

			err := prop.ResolveTemplates(env)
			Expect(err).To(HaveOccurred())
		})

		It("Should return error for invalid template in group", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/archive.tar.gz",
					Ensure: EnsurePresent,
				},
				Url:   "https://example.com/archive.tar.gz",
				Owner: "root",
				Group: "{{ invalid syntax }}",
			}

			env := &templates.Env{Facts: map[string]any{}}

			err := prop.ResolveTemplates(env)
			Expect(err).To(HaveOccurred())
		})

		It("Should return error for invalid template in checksum", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/archive.tar.gz",
					Ensure: EnsurePresent,
				},
				Url:      "https://example.com/archive.tar.gz",
				Owner:    "root",
				Group:    "root",
				Checksum: "{{ invalid syntax }}",
			}

			env := &templates.Env{Facts: map[string]any{}}

			err := prop.ResolveTemplates(env)
			Expect(err).To(HaveOccurred())
		})

		It("Should return error for invalid template in creates", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/archive.tar.gz",
					Ensure: EnsurePresent,
				},
				Url:     "https://example.com/archive.tar.gz",
				Owner:   "root",
				Group:   "root",
				Creates: "{{ invalid syntax }}",
			}

			env := &templates.Env{Facts: map[string]any{}}

			err := prop.ResolveTemplates(env)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ToYamlManifest", func() {
		It("Should marshal to YAML correctly", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/archive.tar.gz",
					Ensure: EnsurePresent,
				},
				Url:           "https://example.com/archive.tar.gz",
				Owner:         "root",
				Group:         "root",
				Checksum:      "abc123",
				ExtractParent: "/opt",
				Cleanup:       true,
				Creates:       "/opt/app/bin",
			}

			raw, err := prop.ToYamlManifest()
			Expect(err).ToNot(HaveOccurred())
			Expect(raw).ToNot(BeNil())

			// Verify the YAML contains expected fields
			yamlStr := string(raw)
			Expect(yamlStr).To(ContainSubstring("name: /tmp/archive.tar.gz"))
			Expect(yamlStr).To(ContainSubstring("ensure: present"))
			Expect(yamlStr).To(ContainSubstring("url: https://example.com/archive.tar.gz"))
			Expect(yamlStr).To(ContainSubstring("owner: root"))
			Expect(yamlStr).To(ContainSubstring("group: root"))
			Expect(yamlStr).To(ContainSubstring("checksum: abc123"))
			Expect(yamlStr).To(ContainSubstring("extract_parent: /opt"))
			Expect(yamlStr).To(ContainSubstring("cleanup: true"))
			Expect(yamlStr).To(ContainSubstring("creates: /opt/app/bin"))
		})

		It("Should omit empty optional fields", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/archive.tar.gz",
					Ensure: EnsurePresent,
				},
				Url:   "https://example.com/archive.tar.gz",
				Owner: "root",
				Group: "root",
			}

			raw, err := prop.ToYamlManifest()
			Expect(err).ToNot(HaveOccurred())

			yamlStr := string(raw)
			Expect(yamlStr).ToNot(ContainSubstring("checksum:"))
			Expect(yamlStr).ToNot(ContainSubstring("extract_parent:"))
			Expect(yamlStr).ToNot(ContainSubstring("creates:"))
			Expect(yamlStr).ToNot(ContainSubstring("headers:"))
		})
	})

	Describe("NewArchiveResourcePropertiesFromYaml", func() {
		It("Should unmarshal from YAML correctly", func() {
			yamlData := `
name: /tmp/archive.tar.gz
ensure: present
url: https://example.com/archive.tar.gz
owner: root
group: wheel
checksum: sha256abc123
extract_parent: /opt
cleanup: true
creates: /opt/app/bin
`
			props, err := NewArchiveResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ArchiveResourceProperties)
			Expect(prop.Name).To(Equal("/tmp/archive.tar.gz"))
			Expect(prop.Ensure).To(Equal(EnsurePresent))
			Expect(prop.Url).To(Equal("https://example.com/archive.tar.gz"))
			Expect(prop.Owner).To(Equal("root"))
			Expect(prop.Group).To(Equal("wheel"))
			Expect(prop.Checksum).To(Equal("sha256abc123"))
			Expect(prop.ExtractParent).To(Equal("/opt"))
			Expect(prop.Cleanup).To(BeTrue())
			Expect(prop.Creates).To(Equal("/opt/app/bin"))
			Expect(prop.Type).To(Equal(ArchiveTypeName))
		})

		It("Should unmarshal headers correctly", func() {
			yamlData := `
name: /tmp/archive.tar.gz
ensure: present
url: https://example.com/archive.tar.gz
owner: root
group: root
headers:
  Authorization: Bearer token123
  X-Custom-Header: value
`
			props, err := NewArchiveResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ArchiveResourceProperties)
			Expect(prop.Headers).To(HaveLen(2))
			Expect(prop.Headers["Authorization"]).To(Equal("Bearer token123"))
			Expect(prop.Headers["X-Custom-Header"]).To(Equal("value"))
		})

		It("Should return error for invalid YAML", func() {
			invalidYaml := `
name: [invalid
`
			_, err := NewArchiveResourcePropertiesFromYaml(yaml.RawMessage(invalidYaml))
			Expect(err).To(HaveOccurred())
		})

		It("Should set Type to archive", func() {
			yamlData := `
name: /tmp/archive.tar.gz
ensure: present
url: https://example.com/archive.tar.gz
owner: root
group: root
`
			props, err := NewArchiveResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ArchiveResourceProperties)
			Expect(prop.Type).To(Equal(ArchiveTypeName))
		})
	})

	Describe("ArchiveState", func() {
		It("Should have correct structure", func() {
			state := &ArchiveState{
				CommonResourceState: NewCommonResourceState(ResourceStatusArchiveProtocol, ArchiveTypeName, "/tmp/archive.tar.gz", EnsurePresent),
				Metadata: &ArchiveMetadata{
					Name:          "/tmp/archive.tar.gz",
					Checksum:      "abc123",
					ArchiveExists: true,
					CreatesExists: true,
					Owner:         "root",
					Group:         "root",
					Provider:      "posix",
					Size:          1024,
				},
			}

			Expect(state.Protocol).To(Equal(ResourceStatusArchiveProtocol))
			Expect(state.ResourceType).To(Equal(ArchiveTypeName))
			Expect(state.Name).To(Equal("/tmp/archive.tar.gz"))
			Expect(state.Ensure).To(Equal(EnsurePresent))
			Expect(state.Metadata.Checksum).To(Equal("abc123"))
			Expect(state.Metadata.ArchiveExists).To(BeTrue())
			Expect(state.Metadata.CreatesExists).To(BeTrue())
			Expect(state.Metadata.Owner).To(Equal("root"))
			Expect(state.Metadata.Group).To(Equal("root"))
			Expect(state.Metadata.Provider).To(Equal("posix"))
			Expect(state.Metadata.Size).To(Equal(int64(1024)))
		})

		It("Should return CommonState correctly", func() {
			state := &ArchiveState{
				CommonResourceState: NewCommonResourceState(ResourceStatusArchiveProtocol, ArchiveTypeName, "/tmp/archive.tar.gz", EnsurePresent),
			}

			common := state.CommonState()
			Expect(common).ToNot(BeNil())
			Expect(common.Name).To(Equal("/tmp/archive.tar.gz"))
		})
	})

	Describe("ArchiveResourceProperties CommonProperties", func() {
		It("Should return CommonProperties correctly", func() {
			prop := &ArchiveResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/archive.tar.gz",
					Ensure: EnsurePresent,
				},
				Url:   "https://example.com/archive.tar.gz",
				Owner: "root",
				Group: "root",
			}

			common := prop.CommonProperties()
			Expect(common).ToNot(BeNil())
			Expect(common.Name).To(Equal("/tmp/archive.tar.gz"))
			Expect(common.Ensure).To(Equal(EnsurePresent))
		})
	})

	Describe("Constants", func() {
		It("Should have correct values", func() {
			Expect(ResourceStatusArchiveProtocol).To(Equal("io.choria.ccm.v1.resource.archive.state"))
			Expect(ArchiveTypeName).To(Equal("archive"))
		})
	})
})
