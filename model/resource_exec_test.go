// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"

	"github.com/goccy/go-yaml"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/choria-io/ccm/templates"
)

var _ = Describe("ExecResourceProperties", func() {
	Describe("Validate", func() {
		DescribeTable("validation tests",
			func(name, ensure, timeout, errorText string) {
				prop := &ExecResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   name,
						Ensure: ensure,
					},
					Timeout: timeout,
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
			Entry("valid simple command", "/bin/echo hello", "present", "", ""),
			Entry("valid command with args", "/usr/bin/ls -la /tmp", "present", "", ""),
			Entry("valid command with timeout", "/bin/sleep 10", "present", "30s", ""),
			Entry("valid command with minute timeout", "/bin/sleep 10", "present", "5m", ""),
			Entry("valid command with hour timeout", "/bin/sleep 10", "present", "1h", ""),
			Entry("valid ensure absent", "/bin/echo hello", "absent", "", ""),

			// Name validation
			Entry("empty name", "", "present", "", "name"),
			Entry("invalid shell quote", "/bin/echo 'unterminated", "present", "", "Unterminated"),

			// Ensure validation
			Entry("empty ensure", "/bin/echo hello", "", "", "ensure"),

			// Timeout validation
			Entry("invalid timeout format", "/bin/echo hello", "present", "invalid", "invalid duration"),
		)

		It("Should skip validation when SkipValidate is true", func() {
			prop := &ExecResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:         "",
					Ensure:       "",
					SkipValidate: true,
				},
			}

			err := prop.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should parse timeout into ParsedTimeout", func() {
			prop := &ExecResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/bin/echo hello",
					Ensure: "present",
				},
				Timeout: "30s",
			}

			err := prop.Validate()
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.ParsedTimeout).To(Equal(30 * time.Second))
		})

		It("Should handle complex timeout values", func() {
			prop := &ExecResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/bin/echo hello",
					Ensure: "present",
				},
				Timeout: "1h30m",
			}

			err := prop.Validate()
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.ParsedTimeout).To(Equal(90 * time.Minute))
		})

		DescribeTable("legitimate exec commands",
			func(name string) {
				prop := &ExecResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   name,
						Ensure: EnsurePresent,
					},
				}

				err := prop.Validate()
				Expect(err).ToNot(HaveOccurred())
			},

			Entry("simple echo", "/bin/echo hello"),
			Entry("command with multiple args", "/usr/bin/find /var -name '*.log'"),
			Entry("command with pipes in quoted arg", "/bin/sh -c 'echo hello | cat'"),
			Entry("command with environment style", "PATH=/usr/bin /bin/echo test"),
			Entry("command with double quotes", "/bin/echo \"hello world\""),
			Entry("command with escaped quotes", "/bin/echo \"it's working\""),
		)

		DescribeTable("subscribe validation",
			func(subscribe []string, errorText string) {
				prop := &ExecResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   "/bin/echo hello",
						Ensure: EnsurePresent,
					},
					Subscribe: subscribe,
				}

				err := prop.Validate()

				if errorText != "" {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(errorText))
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},

			Entry("valid subscribe format", []string{"file#/etc/app.conf"}, ""),
			Entry("valid multiple subscribes", []string{"file#/etc/app.conf", "service#nginx"}, ""),
			Entry("empty subscribe array", []string{}, ""),
			Entry("invalid subscribe without hash", []string{"file:/etc/app.conf"}, "invalid subscribe format"),
			Entry("invalid subscribe with multiple hashes", []string{"file#name#extra"}, "invalid subscribe format"),
			Entry("invalid subscribe empty string", []string{""}, "invalid subscribe format"),
			Entry("mixed valid and invalid", []string{"file#/etc/app.conf", "invalid"}, "invalid subscribe format"),
		)

		DescribeTable("path validation",
			func(path string, errorText string) {
				prop := &ExecResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   "/bin/echo hello",
						Ensure: EnsurePresent,
					},
					Path: path,
				}

				err := prop.Validate()

				if errorText != "" {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(errorText))
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},

			Entry("empty path", "", ""),
			Entry("single absolute path", "/usr/bin", ""),
			Entry("multiple absolute paths", "/usr/local/bin:/usr/bin:/bin", ""),
			Entry("path with spaces in directory", "/usr/local/my bin:/usr/bin", ""),
			Entry("relative path", "usr/bin", "not an absolute path"),
			Entry("mixed absolute and relative", "/usr/bin:local/bin", "not an absolute path"),
			Entry("empty segment at start", ":/usr/bin", "empty directory in path"),
			Entry("empty segment at end", "/usr/bin:", "empty directory in path"),
			Entry("empty segment in middle", "/usr/bin::/bin", "empty directory in path"),
			Entry("just colon", ":", "empty directory in path"),
		)

		DescribeTable("environment validation",
			func(environment []string, errorText string) {
				prop := &ExecResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   "/bin/echo hello",
						Ensure: EnsurePresent,
					},
					Environment: environment,
				}

				err := prop.Validate()

				if errorText != "" {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(errorText))
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},

			Entry("empty environment", []string{}, ""),
			Entry("single valid variable", []string{"FOO=bar"}, ""),
			Entry("multiple valid variables", []string{"FOO=bar", "BAZ=qux"}, ""),
			Entry("variable with equals in value", []string{"FOO=bar=baz"}, ""),
			Entry("variable with spaces in value", []string{"FOO=hello world"}, ""),
			Entry("missing equals", []string{"FOO"}, "missing '='"),
			Entry("empty key", []string{"=value"}, "empty key"),
			Entry("empty value", []string{"FOO="}, "empty value"),
			Entry("mixed valid and invalid", []string{"FOO=bar", "INVALID"}, "missing '='"),
		)
	})

	Describe("ResolveTemplates", func() {
		It("Should resolve templates in Cwd", func() {
			prop := &ExecResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/bin/echo hello",
					Ensure: EnsurePresent,
				},
				Cwd: "/home/{{ Facts.username }}",
			}

			env := &templates.Env{
				Facts: map[string]any{
					"username": "testuser",
				},
			}

			err := prop.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Cwd).To(Equal("/home/testuser"))
		})

		It("Should resolve templates in Subscribe array", func() {
			prop := &ExecResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/bin/echo hello",
					Ensure: EnsurePresent,
				},
				Subscribe: []string{
					"file://{{ Facts.config_path }}",
					"service://{{ Facts.service_name }}",
				},
			}

			env := &templates.Env{
				Facts: map[string]any{
					"config_path":  "/etc/app.conf",
					"service_name": "nginx",
				},
			}

			err := prop.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Subscribe).To(HaveLen(2))
			Expect(prop.Subscribe[0]).To(Equal("file:///etc/app.conf"))
			Expect(prop.Subscribe[1]).To(Equal("service://nginx"))
		})

		It("Should resolve templates in common properties", func() {
			prop := &ExecResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/bin/{{ Facts.command }}",
					Ensure: "{{ Facts.ensure_state }}",
				},
			}

			env := &templates.Env{
				Facts: map[string]any{
					"command":      "echo hello",
					"ensure_state": "present",
				},
			}

			err := prop.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Name).To(Equal("/bin/echo hello"))
			Expect(prop.Ensure).To(Equal("present"))
		})

		It("Should handle non-template strings", func() {
			prop := &ExecResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/bin/echo hello",
					Ensure: EnsurePresent,
				},
				Cwd:       "/tmp",
				Subscribe: []string{"file:///etc/config"},
			}

			env := &templates.Env{
				Facts: map[string]any{},
			}

			err := prop.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Cwd).To(Equal("/tmp"))
			Expect(prop.Subscribe[0]).To(Equal("file:///etc/config"))
		})

		It("Should handle empty Subscribe array", func() {
			prop := &ExecResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/bin/echo hello",
					Ensure: EnsurePresent,
				},
				Subscribe: []string{},
			}

			env := &templates.Env{
				Facts: map[string]any{},
			}

			err := prop.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(prop.Subscribe).To(BeEmpty())
		})

		It("Should return error for invalid template in Cwd", func() {
			prop := &ExecResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/bin/echo hello",
					Ensure: EnsurePresent,
				},
				Cwd: "{{ invalid syntax }}",
			}

			env := &templates.Env{Facts: map[string]any{}}

			err := prop.ResolveTemplates(env)
			Expect(err).To(HaveOccurred())
		})

		It("Should return error for invalid template in Subscribe", func() {
			prop := &ExecResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/bin/echo hello",
					Ensure: EnsurePresent,
				},
				Subscribe: []string{"{{ invalid syntax }}"},
			}

			env := &templates.Env{Facts: map[string]any{}}

			err := prop.ResolveTemplates(env)
			Expect(err).To(HaveOccurred())
		})

		It("Should return error for invalid template in common Name", func() {
			prop := &ExecResourceProperties{
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

	Describe("NewExecResourcePropertiesFromYaml", func() {
		It("Should unmarshal from YAML correctly", func() {
			yamlData := `
name: /bin/echo hello
ensure: present
cwd: /tmp
timeout: 30s
creates: /tmp/marker
refreshonly: true
`
			props, err := NewExecResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ExecResourceProperties)
			Expect(prop.Name).To(Equal("/bin/echo hello"))
			Expect(prop.Ensure).To(Equal(EnsurePresent))
			Expect(prop.Cwd).To(Equal("/tmp"))
			Expect(prop.Timeout).To(Equal("30s"))
			Expect(prop.Creates).To(Equal("/tmp/marker"))
			Expect(prop.RefreshOnly).To(BeTrue())
			Expect(prop.Type).To(Equal(ExecTypeName))
		})

		It("Should set Type to exec", func() {
			yamlData := `
name: /bin/echo hello
ensure: present
`
			props, err := NewExecResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ExecResourceProperties)
			Expect(prop.Type).To(Equal(ExecTypeName))
		})

		It("Should default ensure to present when not specified", func() {
			yamlData := `
name: /bin/echo hello
`
			props, err := NewExecResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ExecResourceProperties)
			Expect(prop.Ensure).To(Equal(EnsurePresent))
		})

		It("Should preserve explicit ensure value", func() {
			yamlData := `
name: /bin/echo hello
ensure: absent
`
			props, err := NewExecResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ExecResourceProperties)
			Expect(prop.Ensure).To(Equal(EnsureAbsent))
		})

		It("Should return error for invalid YAML", func() {
			invalidYaml := `
name: [invalid
`
			_, err := NewExecResourcePropertiesFromYaml(yaml.RawMessage(invalidYaml))
			Expect(err).To(HaveOccurred())
		})

		It("Should parse environment variables", func() {
			yamlData := `
name: /bin/echo hello
ensure: present
environment:
  - FOO=bar
  - BAZ=qux
`
			props, err := NewExecResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ExecResourceProperties)
			Expect(prop.Environment).To(HaveLen(2))
			Expect(prop.Environment[0]).To(Equal("FOO=bar"))
			Expect(prop.Environment[1]).To(Equal("BAZ=qux"))
		})

		It("Should parse returns array", func() {
			yamlData := `
name: /bin/echo hello
ensure: present
returns:
  - 0
  - 1
  - 2
`
			props, err := NewExecResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ExecResourceProperties)
			Expect(prop.Returns).To(HaveLen(3))
			Expect(prop.Returns).To(ContainElements(0, 1, 2))
		})

		It("Should parse subscribe array", func() {
			yamlData := `
name: /bin/systemctl reload nginx
ensure: present
subscribe:
  - file:///etc/nginx/nginx.conf
  - file:///etc/nginx/conf.d/default.conf
`
			props, err := NewExecResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ExecResourceProperties)
			Expect(prop.Subscribe).To(HaveLen(2))
			Expect(prop.Subscribe[0]).To(Equal("file:///etc/nginx/nginx.conf"))
			Expect(prop.Subscribe[1]).To(Equal("file:///etc/nginx/conf.d/default.conf"))
		})

		It("Should parse path field", func() {
			yamlData := `
name: echo hello
ensure: present
path: /usr/local/bin:/usr/bin:/bin
`
			props, err := NewExecResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ExecResourceProperties)
			Expect(prop.Path).To(Equal("/usr/local/bin:/usr/bin:/bin"))
		})

		It("Should parse logoutput field", func() {
			yamlData := `
name: /bin/echo hello
ensure: present
logoutput: true
`
			props, err := NewExecResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ExecResourceProperties)
			Expect(prop.LogOutput).To(BeTrue())
		})

		It("Should default logoutput to false", func() {
			yamlData := `
name: /bin/echo hello
ensure: present
`
			props, err := NewExecResourcePropertiesFromYaml(yaml.RawMessage(yamlData))
			Expect(err).ToNot(HaveOccurred())
			Expect(props).To(HaveLen(1))
			prop := props[0].(*ExecResourceProperties)
			Expect(prop.LogOutput).To(BeFalse())
		})
	})

	Describe("ToYamlManifest", func() {
		It("Should marshal to YAML correctly", func() {
			prop := &ExecResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/bin/echo hello",
					Ensure: EnsurePresent,
				},
				Cwd:         "/tmp",
				Timeout:     "30s",
				Creates:     "/tmp/marker",
				RefreshOnly: true,
			}

			raw, err := prop.ToYamlManifest()
			Expect(err).ToNot(HaveOccurred())
			Expect(raw).ToNot(BeNil())

			yamlStr := string(raw)
			Expect(yamlStr).To(ContainSubstring("name: /bin/echo hello"))
			Expect(yamlStr).To(ContainSubstring("ensure: present"))
			Expect(yamlStr).To(ContainSubstring("cwd: /tmp"))
			Expect(yamlStr).To(ContainSubstring("timeout: 30s"))
			Expect(yamlStr).To(ContainSubstring("creates: /tmp/marker"))
			Expect(yamlStr).To(ContainSubstring("refreshonly: true"))
		})
	})

	Describe("ExecState", func() {
		It("Should have correct structure", func() {
			exitCode := 0
			state := &ExecState{
				CommonResourceState: NewCommonResourceState(ResourceStatusExecProtocol, ExecTypeName, "/bin/echo hello", EnsurePresent),
				ExitCode:            &exitCode,
				CreatesSatisfied:    true,
			}

			Expect(state.Protocol).To(Equal(ResourceStatusExecProtocol))
			Expect(state.ResourceType).To(Equal(ExecTypeName))
			Expect(state.Name).To(Equal("/bin/echo hello"))
			Expect(state.Ensure).To(Equal(EnsurePresent))
			Expect(*state.ExitCode).To(Equal(0))
			Expect(state.CreatesSatisfied).To(BeTrue())
		})

		It("Should return CommonState correctly", func() {
			state := &ExecState{
				CommonResourceState: NewCommonResourceState(ResourceStatusExecProtocol, ExecTypeName, "/bin/echo", EnsurePresent),
			}

			common := state.CommonState()
			Expect(common.Protocol).To(Equal(ResourceStatusExecProtocol))
			Expect(common.ResourceType).To(Equal(ExecTypeName))
		})
	})

	Describe("Constants", func() {
		It("Should have correct values", func() {
			Expect(ResourceStatusExecProtocol).To(Equal("io.choria.ccm.v1.resource.exec.state"))
			Expect(ExecTypeName).To(Equal("exec"))
		})
	})

	Describe("CommonProperties", func() {
		It("Should return the common properties", func() {
			prop := &ExecResourceProperties{
				CommonResourceProperties: CommonResourceProperties{
					Name:   "/bin/echo hello",
					Ensure: EnsurePresent,
				},
			}

			common := prop.CommonProperties()
			Expect(common).ToNot(BeNil())
			Expect(common.Name).To(Equal("/bin/echo hello"))
			Expect(common.Ensure).To(Equal(EnsurePresent))
		})
	})
})
