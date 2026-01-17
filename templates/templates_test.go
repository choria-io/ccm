// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package templates

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTemplates(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Templates")
}

var _ = Describe("Templates", func() {
	var env *Env

	BeforeEach(func() {
		env = &Env{
			Facts: map[string]any{
				"os":       "linux",
				"hostname": "test-server",
				"memory":   16384,
				"cpu":      8,
			},
			Data: map[string]any{
				"app_name":    "myapp",
				"app_version": "1.2.3",
				"port":        8080,
				"enabled":     true,
				"container": map[string]any{
					"version": "v1",
				},
			},
		}
	})

	Describe("ResolveTemplateString", func() {
		It("Should return empty string for empty template", func() {
			result, err := ResolveTemplateString("", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(""))
		})

		It("Should return unchanged string without templates", func() {
			result, err := ResolveTemplateString("plain text", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("plain text"))
		})

		It("Should resolve single template expression", func() {
			result, err := ResolveTemplateString("{{ Data.app_name }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("myapp"))
		})

		It("Should resolve multiple template expressions", func() {
			result, err := ResolveTemplateString("App: {{ Data.app_name }} v{{ Data.app_version }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("App: myapp v1.2.3"))
		})

		It("Should resolve facts", func() {
			result, err := ResolveTemplateString("OS: {{ Facts.os }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("OS: linux"))
		})

		It("Should resolve numbers as strings", func() {
			result, err := ResolveTemplateString("Port: {{ Data.port }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("Port: 8080"))
		})

		It("Should resolve boolean as strings", func() {
			result, err := ResolveTemplateString("Enabled: {{ Data.enabled }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("Enabled: true"))
		})

		It("Should handle whitespace in templates", func() {
			result, err := ResolveTemplateString("{{  Data.app_name  }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("myapp"))
		})

		It("Should fail with invalid expression", func() {
			result, err := ResolveTemplateString("{{ invalid syntax }}", env)
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(""))
		})

		It("Should support arithmetic expressions", func() {
			result, err := ResolveTemplateString("{{ Data.port + 1 }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("8081"))
		})

		It("Should support string concatenation", func() {
			result, err := ResolveTemplateString("{{ Data.app_name + '-' + Data.app_version }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("myapp-1.2.3"))
		})

		It("Should support conditional expressions", func() {
			result, err := ResolveTemplateString("{{ Data.enabled ? 'yes' : 'no' }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("yes"))
		})
	})

	Describe("ResolveTemplateTyped", func() {
		It("Should return empty string for empty template", func() {
			result, err := ResolveTemplateTyped("", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(""))
		})

		It("Should return unchanged string without templates", func() {
			result, err := ResolveTemplateTyped("plain text", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("plain text"))
		})

		It("Should preserve integer type for single expression", func() {
			result, err := ResolveTemplateTyped("{{ Data.port }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeAssignableToTypeOf(int(0)))
			Expect(result).To(Equal(8080))
		})

		It("Should preserve boolean type for single expression", func() {
			result, err := ResolveTemplateTyped("{{ Data.enabled }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeAssignableToTypeOf(true))
			Expect(result).To(Equal(true))
		})

		It("Should return string for single string expression", func() {
			result, err := ResolveTemplateTyped("{{ Data.app_name }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeAssignableToTypeOf(""))
			Expect(result).To(Equal("myapp"))
		})

		It("Should return string for multiple expressions", func() {
			result, err := ResolveTemplateTyped("Port: {{ Data.port }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeAssignableToTypeOf(""))
			Expect(result).To(Equal("Port: 8080"))
		})

		It("Should handle expression with whitespace", func() {
			result, err := ResolveTemplateTyped("{{  Data.port  }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(8080))
		})

		It("Should preserve type for calculated integer", func() {
			result, err := ResolveTemplateTyped("{{ Data.port + 100 }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeAssignableToTypeOf(int(0)))
			Expect(result).To(Equal(8180))
		})

		It("Should fail with invalid expression", func() {
			result, err := ResolveTemplateTyped("{{ invalid syntax }}", env)
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(""))
		})
	})

	Describe("lookup function", func() {
		It("Should lookup nested data with single key", func() {
			env.Data["nested"] = map[string]any{
				"key": "value",
			}

			result, err := ResolveTemplateString("{{ lookup('data.nested.key') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("value"))
		})

		It("Should return default value for non-existent key", func() {
			result, err := ResolveTemplateString("{{ lookup('data.nonexistent', 'default') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("default"))
		})

		It("Should error for non-existent key without default", func() {
			result, err := ResolveTemplateString("{{ lookup('data.nonexistent') }}", env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing key 'data.nonexistent' in environment"))
			Expect(result).To(Equal(""))
		})

		It("Should handle integer lookups", func() {
			result, err := ResolveTemplateString("{{ lookup('data.port') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("8080"))
		})

		It("Should handle float lookups", func() {
			env.Data["pi"] = 3.14159

			result, err := ResolveTemplateString("{{ lookup('data.pi') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("3.14159"))
		})

		It("Should handle deep nested lookups", func() {
			env.Data["level1"] = map[string]any{
				"level2": map[string]any{
					"level3": "deep value",
				},
			}

			result, err := ResolveTemplateString("{{ lookup('data.level1.level2.level3') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("deep value"))
		})

		It("Should handle array element lookups", func() {
			env.Data["items"] = []any{"first", "second", "third"}

			result, err := ResolveTemplateString("{{ lookup('data.items.1') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("second"))
		})
	})

	Describe("Thread safety", func() {
		It("Should handle concurrent ResolveTemplateString calls", func() {
			done := make(chan bool)

			for i := 0; i < 10; i++ {
				go func() {
					defer GinkgoRecover()
					result, err := ResolveTemplateString("{{ Data.app_name }}", env)
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal("myapp"))
					done <- true
				}()
			}

			for i := 0; i < 10; i++ {
				<-done
			}
		})

		It("Should handle concurrent lookup calls", func() {
			env.Data["nested"] = map[string]any{
				"key": "value",
			}

			done := make(chan bool)

			for i := 0; i < 10; i++ {
				go func() {
					defer GinkgoRecover()
					result, err := ResolveTemplateString("{{ lookup('data.nested.key') }}", env)
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal("value"))
					done <- true
				}()
			}

			for i := 0; i < 10; i++ {
				<-done
			}
		})
	})

	Describe("jet function", func() {
		// Note: In Jet templates, use VarMap variables without leading dot (e.g., data.field)
		// or use .Field to access fields on the context object (Env struct)

		DescribeTable("successful template rendering",
			func(template, expected string) {
				result, err := ResolveTemplateString(template, env)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(expected))
			},
			// Variable access - VarMap
			Entry("lowercase VarMap data variable", "{{ jet('[[ data.app_name ]]') }}", "myapp"),
			Entry("lowercase VarMap facts variable", "{{ jet('[[ facts.os ]]') }}", "linux"),
			Entry("capitalized VarMap Data variable", "{{ jet('[[ Data.app_name ]]') }}", "myapp"),
			Entry("capitalized VarMap Facts variable", "{{ jet('[[ Facts.hostname ]]') }}", "test-server"),

			// Variable access - context dot notation
			Entry("context dot notation for Data", "{{ jet('[[ .Data.app_name ]]') }}", "myapp"),
			Entry("context dot notation for Facts", "{{ jet('[[ .Facts.os ]]') }}", "linux"),

			// Custom delimiters
			Entry("custom delimiters << >>", "{{ jet('<< data.app_name >>', '<<', '>>') }}", "myapp"),

			// Control structures
			Entry("if conditional true", "{{ jet('[[ if data.enabled ]]yes[[ end ]]') }}", "yes"),
			Entry("if-else true branch", "{{ jet('[[ if data.enabled ]]enabled[[ else ]]disabled[[ end ]]') }}", "enabled"),
			Entry("numeric comparison >", "{{ jet('[[ if data.port > 80 ]]high[[ else ]]low[[ end ]]') }}", "high"),
			Entry("string equality", "{{ jet('[[ if facts.os == \"linux\" ]]penguin[[ end ]]') }}", "penguin"),

			// Variable assignment
			Entry("variable assignment", "{{ jet('[[ name := data.app_name ]][[ name ]]') }}", "myapp"),

			// Built-in functions
			Entry("isset function", "{{ jet('[[ if isset(data.app_name) ]]exists[[ end ]]') }}", "exists"),

			// Context map
			Entry("context map variables", `{{ jet('[[ name ]]-[[ container.version ]]', {"name": "app", "container": {"version": "v1"} }) }}`, "app-v1"),
			Entry("context from lookup path", "{{ jet('[[ context_name ]]-[[ version ]]', 'data.container') }}", "container-v1"),

			// Edge cases
			Entry("empty template body", "{{ jet('') }}", ""),
			Entry("plain text without expressions", "{{ jet('plain text without expressions') }}", "plain text without expressions"),
		)

		DescribeTable("error cases",
			func(template, expectedError string) {
				_, err := ResolveTemplateString(template, env)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedError))
			},
			Entry("invalid jet syntax", "{{ jet('[[ if ]]') }}", ""),
			Entry("no arguments", "{{ jet() }}", "jet requires 1, 2, 3 or 4 arguments"),
			Entry("non-string body", "{{ jet(123) }}", "jet requires a string argument for template body"),
			Entry("non-string left delimiter", "{{ jet('body', 123, '>>') }}", "jet requires a string argument for left delimiter"),
			Entry("non-string right delimiter", "{{ jet('body', '<<', 123) }}", "jet requires a string argument for right delimiter"),
		)

		It("Should support if-else false branch", func() {
			env.Data["enabled"] = false
			result, err := ResolveTemplateString("{{ jet('[[ if data.enabled ]]enabled[[ else ]]disabled[[ end ]]') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("disabled"))
		})

		It("Should support range over maps", func() {
			env.Data["items"] = map[string]string{"a": "alpha"}
			result, err := ResolveTemplateString("{{ jet('[[ range k, v := data.items ]][[ k ]]=[[ v ]][[ end ]]') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("a=alpha"))
		})

		It("Should support range over slices", func() {
			env.Data["list"] = []string{"one", "two", "three"}
			result, err := ResolveTemplateString("{{ jet('[[ range i, v := data.list ]][[ i ]]:[[ v ]] [[ end ]]') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("0:one 1:two 2:three "))
		})

		It("Should support len function", func() {
			env.Data["list"] = []string{"a", "b", "c"}
			result, err := ResolveTemplateString("{{ jet('[[ len(data.list) ]]') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("3"))
		})

		It("Should access environ VarMap variable", func() {
			env.Environ = map[string]string{"HOME": "/home/test"}
			result, err := ResolveTemplateString("{{ jet('[[ environ.HOME ]]') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("/home/test"))
		})

		It("Should access Environ via context dot notation", func() {
			env.Environ = map[string]string{"PATH": "/usr/bin"}
			result, err := ResolveTemplateString("{{ jet('[[ .Environ.PATH ]]') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("/usr/bin"))
		})
	})

	Describe("Edge cases", func() {
		It("Should handle nil values", func() {
			env.Data["null_value"] = nil

			result, err := ResolveTemplateString("{{ Data.null_value }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("<nil>"))
		})

		It("Should handle empty string values", func() {
			env.Data["empty"] = ""

			result, err := ResolveTemplateString("{{ Data.empty }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(""))
		})

		It("Should handle zero numeric values", func() {
			env.Data["zero"] = 0

			result, err := ResolveTemplateString("{{ Data.zero }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("0"))
		})

		It("Should handle false boolean values", func() {
			env.Data["disabled"] = false

			result, err := ResolveTemplateString("{{ Data.disabled }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("false"))
		})

		It("Should handle special characters in output", func() {
			env.Data["special"] = "Hello {{ World }}"

			result, err := ResolveTemplateString("{{ Data.special }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("Hello {{ World }}"))
		})

		It("Should handle consecutive templates", func() {
			result, err := ResolveTemplateString("{{ Data.app_name }}{{ Data.app_version }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("myapp1.2.3"))
		})

		It("Should handle negative numbers", func() {
			env.Data["negative"] = -42

			result, err := ResolveTemplateString("{{ Data.negative }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("-42"))
		})
	})
})
