// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"testing"

	"github.com/goccy/go-yaml"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPackageResourceProperties(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Model")
}

var _ = Describe("PackageResourceProperties", func() {
	Describe("Validate", func() {
		DescribeTable("validation tests",
			func(propName, ensure, errorText string) {
				prop := &PackageResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   propName,
						Ensure: ensure,
					},
				}

				err := prop.Validate()

				if errorText != "" {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(errorText))
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},

			Entry("valid package name", "nginx", "present", ""),
			Entry("valid package name with dots", "python3.11", "present", ""),
			Entry("valid package name with hyphens", "python3-pip", "present", ""),
			Entry("valid package name with underscores", "lib_name", "present", ""),
			Entry("valid package name with plus", "g++", "present", ""),
			Entry("valid package name with colon (epoch)", "package:1.0", "present", ""),
			Entry("valid package name with tilde", "package~test", "present", ""),
			Entry("valid version string", "nginx", "1.2.3-4.el9", ""),
			Entry("valid ensure absent", "nginx", "absent", ""),
			Entry("valid ensure latest", "nginx", "latest", ""),
			Entry("empty package name", "", "present", "name"),
			Entry("empty ensure", "nginx", "", "ensure"),
			Entry("package name with semicolon (command separator)", "nginx; rm -rf /", "present", "dangerous characters"),
			Entry("package name with pipe (command pipe)", "nginx | cat", "present", "dangerous characters"),
			Entry("package name with ampersand (background)", "nginx & whoami", "present", "dangerous characters"),
			Entry("package name with dollar (variable expansion)", "nginx$PATH", "present", "dangerous characters"),
			Entry("package name with backtick (command substitution)", "nginx`whoami`", "present", "dangerous characters"),
			Entry("package name with single quote", "nginx'test", "present", "dangerous characters"),
			Entry("package name with double quote", "nginx\"test", "present", "dangerous characters"),
			Entry("package name with parentheses (subshell)", "nginx(whoami)", "present", "dangerous characters"),
			Entry("package name with brackets", "nginx[test]", "present", "dangerous characters"),
			Entry("package name with asterisk (wildcard)", "nginx*", "present", "dangerous characters"),
			Entry("package name with question mark (wildcard)", "nginx?", "present", "dangerous characters"),
			Entry("package name with redirect", "nginx > /tmp/file", "present", "dangerous characters"),
			Entry("package name with backslash", "nginx\\test", "present", "dangerous characters"),
			Entry("package name with newline", "nginx\nwhoami", "present", "dangerous characters"),
			Entry("package name with tab", "nginx\twhoami", "present", "dangerous characters"),
			Entry("package name with space", "nginx test", "present", "invalid characters"),
			Entry("package name with leading space", " nginx", "present", "invalid characters"),
			Entry("package name with trailing space", "nginx ", "present", "invalid characters"),
			Entry("package name with invalid characters", "nginx@test", "present", "invalid characters"),
			Entry("version with semicolon", "nginx", "1.2.3; rm -rf /", "dangerous characters"),
			Entry("version with command substitution", "nginx", "1.2.3$(whoami)", "dangerous characters"),
		)

		DescribeTable("legitimate packages",
			func(name, ensure string) {
				prop := &PackageResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   name,
						Ensure: ensure,
					},
				}

				err := prop.Validate()
				Expect(err).ToNot(HaveOccurred())
			},

			Entry("nginx present", "nginx", "present"),
			Entry("python3.11 present", "python3.11", "present"),
			Entry("python3-pip present", "python3-pip", "present"),
			Entry("libssl1.1 present", "libssl1.1", "present"),
			Entry("g++ present", "g++", "present"),
			Entry("gcc-c++ present", "gcc-c++", "present"),
			Entry("kernel-devel with version", "kernel-devel", "3.10.0-1160.el7"),
			Entry("nginx with version", "nginx", "1.20.1-1.el8"),
			Entry("nodejs with complex version", "nodejs", "16.14.2-1nodesource1"),
			Entry("package_name with version", "package_name", "1.0.0"),
			Entry("package.name with version", "package.name", "1.0.0"),
			Entry("package-name with version", "package-name", "1.0.0"),
			Entry("package+extra with version", "package+extra", "1.0.0"),
			Entry("package~test with version", "package~test", "1.0.0"),
			Entry("vim-enhanced latest", "vim-enhanced", "latest"),
			Entry("httpd absent", "httpd", "absent"),
		)
	})
})

var _ = Describe("CommonResourceProperties", func() {
	Describe("Validate", func() {
		Describe("Require", func() {
			It("Should accept valid require references", func() {
				prop := &CommonResourceProperties{
					Name:    "test",
					Ensure:  "present",
					Require: []string{"package#nginx", "file#/etc/config", "service#httpd"},
				}

				err := prop.Validate()
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should accept empty require list", func() {
				prop := &CommonResourceProperties{
					Name:    "test",
					Ensure:  "present",
					Require: []string{},
				}

				err := prop.Validate()
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should accept nil require list", func() {
				prop := &CommonResourceProperties{
					Name:    "test",
					Ensure:  "present",
					Require: nil,
				}

				err := prop.Validate()
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should reject require without hash separator", func() {
				prop := &CommonResourceProperties{
					Name:    "test",
					Ensure:  "present",
					Require: []string{"package-nginx"},
				}

				err := prop.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ErrInvalidRequires))
			})

			It("Should accept require with empty type (validation is lenient)", func() {
				// IsValidResourceRef only checks for presence of # separator
				prop := &CommonResourceProperties{
					Name:    "test",
					Ensure:  "present",
					Require: []string{"#nginx"},
				}

				err := prop.Validate()
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should accept require with empty name (validation is lenient)", func() {
				// IsValidResourceRef only checks for presence of # separator
				prop := &CommonResourceProperties{
					Name:    "test",
					Ensure:  "present",
					Require: []string{"package#"},
				}

				err := prop.Validate()
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should reject if any require is invalid", func() {
				prop := &CommonResourceProperties{
					Name:    "test",
					Ensure:  "present",
					Require: []string{"package#nginx", "invalid-format", "file#/etc/config"},
				}

				err := prop.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ErrInvalidRequires))
			})

			It("Should accept require with paths containing hash", func() {
				prop := &CommonResourceProperties{
					Name:    "test",
					Ensure:  "present",
					Require: []string{"file#/path/to/file#with#hash"},
				}

				err := prop.Validate()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
	Describe("NewPackageResourcePropertiesFromYaml", func() {
		Describe("traditional single resource format", func() {
			It("Should parse single resource format", func() {
				yamlData := `
name: nginx
ensure: present
`
				props, err := NewPackageResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
				Expect(err).ToNot(HaveOccurred())
				Expect(props).To(HaveLen(1))

				prop := props[0].(*PackageResourceProperties)
				Expect(prop.Name).To(Equal("nginx"))
				Expect(prop.Ensure).To(Equal(EnsurePresent))
			})

			It("Should parse single resource with require", func() {
				yamlData := `
name: nginx
ensure: present
require: ["file#/etc/nginx.conf"]
`
				props, err := NewPackageResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
				Expect(err).ToNot(HaveOccurred())
				Expect(props).To(HaveLen(1))

				prop := props[0].(*PackageResourceProperties)
				Expect(prop.Name).To(Equal("nginx"))
				Expect(prop.Require).To(Equal([]string{"file#/etc/nginx.conf"}))
			})
		})

		Describe("new multi-resource format", func() {
			It("Should parse multiple resources without defaults", func() {
				yamlData := `
- zsh:
    ensure: present
- bash:
    ensure: absent
`
				props, err := NewPackageResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
				Expect(err).ToNot(HaveOccurred())
				Expect(props).To(HaveLen(2))

				prop := props[0].(*PackageResourceProperties)
				Expect(prop.Name).To(Equal("zsh"))
				Expect(prop.Ensure).To(Equal(EnsurePresent))

				prop = props[1].(*PackageResourceProperties)
				Expect(prop.Name).To(Equal("bash"))
				Expect(prop.Ensure).To(Equal(EnsureAbsent))
			})

			It("Should apply defaults to all resources", func() {
				yamlData := `
- defaults:
    require: ["file#x"]
- zsh:
    ensure: present
- bash:
    ensure: absent
`
				props, err := NewPackageResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
				Expect(err).ToNot(HaveOccurred())
				Expect(props).To(HaveLen(2))

				prop := props[0].(*PackageResourceProperties)
				Expect(prop.Require).To(Equal([]string{"file#x"}))
				Expect(prop.Name).To(Equal("zsh"))
				Expect(prop.Ensure).To(Equal(EnsurePresent))

				prop = props[1].(*PackageResourceProperties)
				Expect(prop.Require).To(Equal([]string{"file#x"}))
				Expect(prop.Name).To(Equal("bash"))
				Expect(prop.Ensure).To(Equal(EnsureAbsent))
			})

			It("Should allow resource-specific properties to override defaults", func() {
				yamlData := `
- defaults:
    ensure: present
    require: ["file#default"]
- nginx:
    require: ["file#nginx-specific"]
- vim:
    ensure: latest
`
				props, err := NewPackageResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
				Expect(err).ToNot(HaveOccurred())
				Expect(props).To(HaveLen(2))

				// nginx overrides the require from defaults but inherits ensure
				prop := props[0].(*PackageResourceProperties)
				Expect(prop.Name).To(Equal("nginx"))
				Expect(prop.Ensure).To(Equal(EnsurePresent))
				Expect(prop.Require).To(Equal([]string{"file#nginx-specific"}))

				// vim overrides ensure but inherits require
				prop = props[1].(*PackageResourceProperties)
				Expect(prop.Name).To(Equal("vim"))
				Expect(prop.Ensure).To(Equal("latest"))
				Expect(prop.Require).To(Equal([]string{"file#default"}))
			})

			It("Should preserve resource order", func() {
				yamlData := `
- defaults:
    ensure: present
- alpha:
- beta:
- gamma:
- delta:
`
				props, err := NewPackageResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
				Expect(err).ToNot(HaveOccurred())
				Expect(props).To(HaveLen(4))

				Expect(props[0].(*PackageResourceProperties).Name).To(Equal("alpha"))
				Expect(props[1].(*PackageResourceProperties).Name).To(Equal("beta"))
				Expect(props[2].(*PackageResourceProperties).Name).To(Equal("gamma"))
				Expect(props[3].(*PackageResourceProperties).Name).To(Equal("delta"))
			})

			It("Should handle defaults at any position in the list", func() {
				yamlData := `
- zsh:
    ensure: present
- defaults:
    require: ["file#x"]
- bash:
    ensure: absent
`
				props, err := NewPackageResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
				Expect(err).ToNot(HaveOccurred())
				Expect(props).To(HaveLen(2))

				// Both should have the require from defaults
				prop := props[0].(*PackageResourceProperties)
				Expect(prop.Name).To(Equal("zsh"))
				Expect(prop.Require).To(Equal([]string{"file#x"}))

				prop = props[1].(*PackageResourceProperties)
				Expect(prop.Name).To(Equal("bash"))
				Expect(prop.Require).To(Equal([]string{"file#x"}))
			})

			It("Should handle empty resource properties with explicit empty block", func() {
				yamlData := `
- defaults:
    ensure: latest
- nginx: {}
- vim: {}
`
				props, err := NewPackageResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
				Expect(err).ToNot(HaveOccurred())
				Expect(props).To(HaveLen(2))

				prop := props[0].(*PackageResourceProperties)
				Expect(prop.Name).To(Equal("nginx"))
				Expect(prop.Ensure).To(Equal("latest"))

				prop = props[1].(*PackageResourceProperties)
				Expect(prop.Name).To(Equal("vim"))
				Expect(prop.Ensure).To(Equal("latest"))
			})

			It("Should handle null resource properties (defaults cleared)", func() {
				// When a resource has no properties (null), unmarshaling clears the struct
				yamlData := `
- defaults:
    ensure: latest
- nginx:
- vim:
`
				props, err := NewPackageResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
				Expect(err).ToNot(HaveOccurred())
				Expect(props).To(HaveLen(2))

				// With null values, defaults are not preserved
				prop := props[0].(*PackageResourceProperties)
				Expect(prop.Name).To(Equal("nginx"))
				Expect(prop.Ensure).To(BeEmpty())

				prop = props[1].(*PackageResourceProperties)
				Expect(prop.Name).To(Equal("vim"))
				Expect(prop.Ensure).To(BeEmpty())
			})

			It("Should set the Type field correctly", func() {
				yamlData := `
- nginx:
    ensure: present
`
				props, err := NewPackageResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
				Expect(err).ToNot(HaveOccurred())
				Expect(props).To(HaveLen(1))

				prop := props[0].(*PackageResourceProperties)
				Expect(prop.Type).To(Equal(PackageTypeName))
			})
		})

		Describe("error cases", func() {
			It("Should fail when multiple defaults blocks are present", func() {
				yamlData := `
- defaults:
    ensure: present
- defaults:
    require: ["file#x"]
- nginx:
`
				_, err := NewPackageResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("multiple defaults"))
			})
		})
	})
})
