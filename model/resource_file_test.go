// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"github.com/choria-io/ccm/templates"
	"github.com/goccy/go-yaml"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("FileResourceProperties", func() {
	Describe("Validate", func() {
		DescribeTable("validation tests",
			func(name, ensure, owner, group, mode, errorText string) {
				prop := &FileResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   name,
						Ensure: ensure,
					},
					Owner: owner,
					Group: group,
					Mode:  mode,
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
			Entry("valid file properties", "/tmp/test.txt", "present", "root", "root", "0644", ""),
			Entry("valid file with absent ensure", "/tmp/test.txt", "absent", "root", "root", "0644", ""),
			Entry("valid absolute path", "/etc/nginx/nginx.conf", "present", "nginx", "nginx", "0640", ""),
			Entry("valid path with spaces in name", "/tmp/my file.txt", "present", "root", "root", "0644", ""),

			// Name validation
			Entry("empty name", "", "present", "root", "root", "0644", "name"),
			Entry("path with ..", "/tmp/../etc/passwd", "present", "root", "root", "0644", "absolute"),
			Entry("path with .", "/tmp/./file.txt", "present", "root", "root", "0644", "absolute"),

			// Ensure validation
			Entry("empty ensure", "/tmp/test.txt", "", "root", "root", "0644", "ensure"),
			Entry("invalid ensure value", "/tmp/test.txt", "latest", "root", "root", "0644", "ensure must be one of"),
			Entry("invalid ensure running", "/tmp/test.txt", "running", "root", "root", "0644", "ensure must be one of"),

			// Owner validation
			Entry("empty owner", "/tmp/test.txt", "present", "", "root", "0644", "owner cannot be empty"),

			// Group validation
			Entry("empty group", "/tmp/test.txt", "present", "root", "", "0644", "group cannot be empty"),

			// Mode validation
			Entry("empty mode", "/tmp/test.txt", "present", "root", "root", "", "mode cannot be empty"),
		)

		DescribeTable("legitimate file paths",
			func(name, owner, group, mode string) {
				prop := &FileResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   name,
						Ensure: EnsurePresent,
					},
					Owner: owner,
					Group: group,
					Mode:  mode,
				}

				err := prop.Validate()
				Expect(err).ToNot(HaveOccurred())
			},

			Entry("simple file", "/tmp/test.txt", "root", "root", "0644"),
			Entry("config file", "/etc/nginx/nginx.conf", "nginx", "nginx", "0640"),
			Entry("hidden file", "/home/user/.bashrc", "user", "user", "0644"),
			Entry("log file", "/var/log/app.log", "app", "app", "0644"),
			Entry("socket path", "/var/run/app.sock", "root", "root", "0755"),
			Entry("deep path", "/opt/app/config/settings/main.conf", "app", "app", "0600"),
		)
	})

	Describe("ResolveTemplates", func() {
		It("Should resolve templates in all fields", func() {
			prop := &FileResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/{{ Facts.filename }}",
					Ensure: EnsurePresent,
				},
				Owner:    "{{ Facts.owner }}",
				Group:    "{{ Facts.group }}",
				Mode:     "{{ Facts.mode }}",
				Contents: "Hello {{ Facts.name }}!",
			}

			env := &templates.Env{
				Facts: map[string]any{
					"filename": "test.txt",
					"owner":    "root",
					"group":    "wheel",
					"mode":     "0644",
					"name":     "World",
				},
			}

			err := prop.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Name).To(Equal("/tmp/test.txt"))
			Expect(prop.Owner).To(Equal("root"))
			Expect(prop.Group).To(Equal("wheel"))
			Expect(prop.Mode).To(Equal("0644"))
			Expect(prop.Contents).To(Equal("Hello World!"))
		})

		It("Should handle non-template strings", func() {
			prop := &FileResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/test.txt",
					Ensure: EnsurePresent,
				},
				Owner:    "root",
				Group:    "root",
				Mode:     "0644",
				Contents: "Static content",
			}

			env := &templates.Env{
				Facts: map[string]any{},
			}

			err := prop.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Name).To(Equal("/tmp/test.txt"))
			Expect(prop.Owner).To(Equal("root"))
			Expect(prop.Group).To(Equal("root"))
			Expect(prop.Mode).To(Equal("0644"))
			Expect(prop.Contents).To(Equal("Static content"))
		})

		It("Should return error for invalid template in owner", func() {
			prop := &FileResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/test.txt",
					Ensure: EnsurePresent,
				},
				Owner:    "{{ invalid syntax }}",
				Group:    "root",
				Mode:     "0644",
				Contents: "content",
			}

			env := &templates.Env{Facts: map[string]any{}}

			err := prop.ResolveTemplates(env)
			Expect(err).To(HaveOccurred())
		})

		It("Should return error for invalid template in group", func() {
			prop := &FileResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/test.txt",
					Ensure: EnsurePresent,
				},
				Owner:    "root",
				Group:    "{{ invalid syntax }}",
				Mode:     "0644",
				Contents: "content",
			}

			env := &templates.Env{Facts: map[string]any{}}

			err := prop.ResolveTemplates(env)
			Expect(err).To(HaveOccurred())
		})

		It("Should return error for invalid template in mode", func() {
			prop := &FileResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/test.txt",
					Ensure: EnsurePresent,
				},
				Owner:    "root",
				Group:    "root",
				Mode:     "{{ invalid syntax }}",
				Contents: "content",
			}

			env := &templates.Env{Facts: map[string]any{}}

			err := prop.ResolveTemplates(env)
			Expect(err).To(HaveOccurred())
		})

		It("Should return error for invalid template in contents", func() {
			prop := &FileResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/test.txt",
					Ensure: EnsurePresent,
				},
				Owner:    "root",
				Group:    "root",
				Mode:     "0644",
				Contents: "{{ invalid syntax }}",
			}

			env := &templates.Env{Facts: map[string]any{}}

			err := prop.ResolveTemplates(env)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ToYamlManifest", func() {
		It("Should marshal to YAML correctly", func() {
			prop := &FileResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/tmp/test.txt",
					Ensure: EnsurePresent,
				},
				Owner:    "root",
				Group:    "root",
				Mode:     "0644",
				Contents: "file content",
			}

			raw, err := prop.ToYamlManifest()
			Expect(err).ToNot(HaveOccurred())
			Expect(raw).ToNot(BeNil())

			// Verify the YAML contains expected fields
			yamlStr := string(raw)
			Expect(yamlStr).To(ContainSubstring("name: /tmp/test.txt"))
			Expect(yamlStr).To(ContainSubstring("ensure: present"))
			Expect(yamlStr).To(ContainSubstring("owner: root"))
			Expect(yamlStr).To(ContainSubstring("group: root"))
			Expect(yamlStr).To(ContainSubstring("mode: \"0644\""))
			Expect(yamlStr).To(ContainSubstring("content: file content"))
		})
	})

	Describe("NewFileResourcePropertiesFromYaml", func() {
		It("Should unmarshal from YAML correctly", func() {
			yamlData := `
name: /tmp/test.txt
ensure: present
owner: root
group: wheel
mode: "0644"
content: "hello world"
`
			prop, err := NewFileResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(prop).ToNot(BeNil())
			Expect(prop.Name).To(Equal("/tmp/test.txt"))
			Expect(prop.Ensure).To(Equal(EnsurePresent))
			Expect(prop.Owner).To(Equal("root"))
			Expect(prop.Group).To(Equal("wheel"))
			Expect(prop.Mode).To(Equal("0644"))
			Expect(prop.Contents).To(Equal("hello world"))
			Expect(prop.Type).To(Equal(FileTypeName))
		})

		It("Should return error for invalid YAML", func() {
			invalidYaml := `
name: [invalid
`
			_, err := NewFileResourcePropertiesFromYaml(yaml.RawMessage(invalidYaml))
			Expect(err).To(HaveOccurred())
		})

		It("Should handle multiline content", func() {
			yamlData := `
name: /tmp/test.txt
ensure: present
owner: root
group: root
mode: "0644"
content: |
  line 1
  line 2
  line 3
`
			prop, err := NewFileResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Contents).To(ContainSubstring("line 1"))
			Expect(prop.Contents).To(ContainSubstring("line 2"))
			Expect(prop.Contents).To(ContainSubstring("line 3"))
		})

		It("Should set Type to file", func() {
			yamlData := `
name: /tmp/test.txt
ensure: present
owner: root
group: root
mode: "0644"
`
			prop, err := NewFileResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Type).To(Equal(FileTypeName))
		})
	})

	Describe("FileState", func() {
		It("Should have correct structure", func() {
			state := &FileState{
				CommonResourceState: NewCommonResourceState(ResourceStatusFileProtocol, FileTypeName, "/tmp/test.txt", EnsurePresent),
				Metadata: &FileMetadata{
					Name:     "/tmp/test.txt",
					Checksum: "abc123",
					Owner:    "root",
					Group:    "root",
					Mode:     "0644",
					Provider: "posix",
					Extended: map[string]any{"key": "value"},
				},
			}

			Expect(state.Protocol).To(Equal(ResourceStatusFileProtocol))
			Expect(state.ResourceType).To(Equal(FileTypeName))
			Expect(state.Name).To(Equal("/tmp/test.txt"))
			Expect(state.Ensure).To(Equal(EnsurePresent))
			Expect(state.Metadata.Checksum).To(Equal("abc123"))
			Expect(state.Metadata.Owner).To(Equal("root"))
			Expect(state.Metadata.Group).To(Equal("root"))
			Expect(state.Metadata.Mode).To(Equal("0644"))
			Expect(state.Metadata.Provider).To(Equal("posix"))
			Expect(state.Metadata.Extended).To(HaveKeyWithValue("key", "value"))
		})
	})

	Describe("Constants", func() {
		It("Should have correct values", func() {
			Expect(ResourceStatusFileProtocol).To(Equal("io.choria.ccm.v1.resource.file.state"))
			Expect(FileTypeName).To(Equal("file"))
		})
	})
})
