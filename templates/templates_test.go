// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package templates

import (
	"bytes"
	"testing"
	"text/template"

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
			Entry("no arguments", "{{ jet() }}", "jet requires 1 or 3 arguments"),
			Entry("2 arguments", "{{ jet('body', 'left') }}", "jet requires 1 or 3 arguments"),
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

	Describe("template function", func() {
		BeforeEach(func() {
			env.WorkingDir = "testdata"
		})

		It("Should detect .jet extension and delegate to jet()", func() {
			result, err := ResolveTemplateString("{{ template('test.jet') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("myapp v1.2.3"))
		})

		It("Should detect .templ extension and process as expr template", func() {
			result, err := ResolveTemplateString("{{ template('test.templ') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("myapp v1.2.3"))
		})

		It("Should process inline content without file extension", func() {
			result, err := ExprParse("template('plain text')", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("plain text"))
		})

		It("Should process inline expr template without file extension", func() {
			result, err := ExprParse(`template("prefix-" + Data.app_name)`, env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("prefix-myapp"))
		})

		It("Should error when .jet file does not exist", func() {
			_, err := ResolveTemplateString("{{ template('nonexistent.jet') }}", env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read file"))
		})

		It("Should error when .templ file does not exist", func() {
			_, err := ResolveTemplateString("{{ template('nonexistent.templ') }}", env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read file"))
		})

		It("Should error when called without arguments", func() {
			_, err := ResolveTemplateString("{{ template() }}", env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("template requires a string argument"))
		})

		It("Should error when called with non-string argument", func() {
			_, err := ResolveTemplateString("{{ template(123) }}", env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("template requires a string argument"))
		})
	})

	Describe("registrations function", func() {
		type regEntry struct {
			Cluster  string `json:"cluster"`
			Protocol string `json:"protocol"`
			Service  string `json:"service"`
			Address  string `json:"address"`
			Port     int    `json:"port"`
		}

		BeforeEach(func() {
			env.RegistrationsFunc = func(cluster, protocol, service, ip string) (any, error) {
				return []*regEntry{
					{Cluster: cluster, Protocol: protocol, Service: service, Address: "10.0.0.1", Port: 8080},
					{Cluster: cluster, Protocol: protocol, Service: service, Address: "10.0.0.2", Port: 9090},
				}, nil
			}
		})

		It("Should call registrations from expr templates", func() {
			result, err := ExprParse("registrations('prod', 'tcp', 'web', '*')", env)
			Expect(err).ToNot(HaveOccurred())
			entries, ok := result.([]*regEntry)
			Expect(ok).To(BeTrue())
			Expect(entries).To(HaveLen(2))
			Expect(entries[0].Address).To(Equal("10.0.0.1"))
			Expect(entries[1].Address).To(Equal("10.0.0.2"))
		})

		It("Should call registrations from jet templates", func() {
			result, err := ResolveTemplateString("{{ jet('[[ range _, entry := registrations(\"prod\", \"tcp\", \"web\", \"*\") ]][[ entry.Address ]] [[ end ]]') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("10.0.0.1 10.0.0.2 "))
		})

		It("Should call registrations from go templates", func() {
			goTpl := `{{ range registrations "prod" "tcp" "web" "*" }}{{ .Address }} {{ end }}`
			tmpl, err := template.New("test").Funcs(env.GoFunctions()).Parse(goTpl)
			Expect(err).ToNot(HaveOccurred())

			var buf bytes.Buffer
			err = tmpl.Execute(&buf, env)
			Expect(err).ToNot(HaveOccurred())
			Expect(buf.String()).To(Equal("10.0.0.1 10.0.0.2 "))
		})

		It("Should error when RegistrationsFunc is nil", func() {
			env.RegistrationsFunc = nil
			_, err := ExprParse("registrations('prod', 'tcp', 'web', '*')", env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("registrations function not available"))
		})

		It("Should error with wrong number of arguments", func() {
			_, err := ExprParse("registrations('prod', 'tcp')", env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("registrations requires 4 string arguments"))
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

	Describe("ResolveTemplateStringMatch", func() {
		It("Should return matched true when placeholders resolve", func() {
			result, matched, err := ResolveTemplateStringMatch("role:{{ Data.app_name }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("role:myapp"))
			Expect(matched).To(BeTrue())
		})

		It("Should return matched false when placeholders resolve to empty", func() {
			env.DefaultOnMissing = true
			result, matched, err := ResolveTemplateStringMatch("role:{{ lookup('data.missing') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("role:"))
			Expect(matched).To(BeFalse())
		})

		It("Should return matched true for plain strings without placeholders", func() {
			result, matched, err := ResolveTemplateStringMatch("plain text", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("plain text"))
			Expect(matched).To(BeTrue())
		})
	})

	Describe("DefaultOnMissing", func() {
		It("Should error on missing keys when DefaultOnMissing is false", func() {
			_, err := ResolveTemplateString("{{ lookup('data.nonexistent') }}", env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing key"))
		})

		It("Should return empty string on missing keys when DefaultOnMissing is true", func() {
			env.DefaultOnMissing = true
			result, err := ResolveTemplateString("{{ lookup('data.nonexistent') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(""))
		})

		It("Should still use default value when provided", func() {
			env.DefaultOnMissing = true
			result, err := ResolveTemplateString("{{ lookup('data.nonexistent', 'fallback') }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("fallback"))
		})
	})

	Describe("RestrictFunctions", func() {
		It("Should allow readFile when RestrictFunctions is false", func() {
			_, err := ExprParse("readFile('/nonexistent')", env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read file"))
		})

		It("Should disallow readFile when RestrictFunctions is true", func() {
			env.RestrictFunctions = true
			_, err := ExprParse("readFile('/nonexistent')", env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("compile error"))
		})

		It("Should disallow file when RestrictFunctions is true", func() {
			env.RestrictFunctions = true
			_, err := ExprParse("file('/nonexistent')", env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("compile error"))
		})

		It("Should disallow template when RestrictFunctions is true", func() {
			env.RestrictFunctions = true
			_, err := ExprParse("template('test')", env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("compile error"))
		})

		It("Should allow lookup when RestrictFunctions is true", func() {
			env.RestrictFunctions = true
			env.DefaultOnMissing = true
			result, err := ExprParse("lookup('data.app_name')", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("myapp"))
		})
	})

	Describe("ExpandValuesRecursively", func() {
		It("Should expand string values", func() {
			result, err := ExpandValuesRecursively("App: {{ Data.app_name }}", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("App: myapp"))
		})

		It("Should return non-string primitives unchanged", func() {
			result, err := ExpandValuesRecursively(42, env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(42))

			result, err = ExpandValuesRecursively(true, env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(true))
		})

		It("Should recursively expand map values", func() {
			input := map[string]any{
				"name": "{{ Data.app_name }}",
				"port": 8080,
			}
			result, err := ExpandValuesRecursively(input, env)
			Expect(err).ToNot(HaveOccurred())
			m := result.(map[string]any)
			Expect(m["name"]).To(Equal("myapp"))
			Expect(m["port"]).To(Equal(8080))
		})

		It("Should recursively expand slice values", func() {
			input := []any{"{{ Data.app_name }}", 42, "{{ Data.app_version }}"}
			result, err := ExpandValuesRecursively(input, env)
			Expect(err).ToNot(HaveOccurred())
			s := result.([]any)
			Expect(s[0]).To(Equal("myapp"))
			Expect(s[1]).To(Equal(42))
			Expect(s[2]).To(Equal("1.2.3"))
		})

		It("Should handle deeply nested structures", func() {
			input := map[string]any{
				"outer": map[string]any{
					"inner": []any{"{{ Data.app_name }}"},
				},
			}
			result, err := ExpandValuesRecursively(input, env)
			Expect(err).ToNot(HaveOccurred())
			m := result.(map[string]any)
			outer := m["outer"].(map[string]any)
			inner := outer["inner"].([]any)
			Expect(inner[0]).To(Equal("myapp"))
		})
	})

	Describe("ExpandMapValues", func() {
		It("Should expand all map values", func() {
			input := map[string]any{
				"app":  "{{ Data.app_name }}",
				"port": "{{ Data.port }}",
			}
			result, err := ExpandMapValues(input, env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result["app"]).To(Equal("myapp"))
			Expect(result["port"]).To(Equal(8080))
		})
	})

	Describe("findTemplateExpressions", func() {
		DescribeTable("single expression matching",
			func(input string, count int, innerExprs []string) {
				matches := findTemplateExpressions(input)
				Expect(matches).To(HaveLen(count))
				for i, expr := range innerExprs {
					Expect(input[matches[i].innerStart:matches[i].innerEnd]).To(Equal(expr))
				}
			},
			// {{ }} basic
			Entry("simple {{ }} expression", "{{ Data.x }}", 1, []string{"Data.x"}),
			Entry("multiple {{ }} expressions", "a {{ x }} b {{ y }} c", 2, []string{"x", "y"}),
			Entry("empty {{ }} expression", "{{  }}", 1, []string{""}),
			Entry("single { inside {{ }}", "{{ a } b }}", 1, []string{"a } b"}),

			// {{ }} quote handling
			Entry("}} inside double quotes", `{{ x("}}") }}`, 1, []string{`x("}}")`}),
			Entry("}} inside single quotes", `{{ x('}}') }}`, 1, []string{`x('}}')`}),
			Entry("}} inside backtick strings", "{{ x(`}}`) }}", 1, []string{"x(`}}`)"}),
			Entry("escaped quote before }}", `{{ x("hello\"}}world") }}`, 1, []string{`x("hello\"}}world")`}),
			Entry("backtick raw no escapes", "{{ x(`test\\`)}}", 1, []string{"x(`test\\`)"}),

			// ${ } basic
			Entry("simple ${ } expression", "${ Data.x }", 1, []string{"Data.x"}),
			Entry("empty ${ } expression", "${  }", 1, []string{""}),
			Entry("multiple ${ } expressions", "${ a } and ${ b }", 2, []string{"a", "b"}),

			// ${ } brace depth and quote handling
			Entry("brace depth for lambdas", "${ map(list, { # + 1 }) }", 1, []string{"map(list, { # + 1 })"}),
			Entry("nested filter lambda", "${ filter(items, { # > 5 }) }", 1, []string{"filter(items, { # > 5 })"}),
			Entry("} inside quotes in ${ }", `${ x("}") }`, 1, []string{`x("}")`}),
			Entry("}} inside ${ } quotes", `${ x("}}") }`, 1, []string{`x("}}")`}),

			// mixed syntax
			Entry("mixed {{ }} and ${ }", "a {{ x }} b ${ y } c", 2, []string{"x", "y"}),

			// ${{ disambiguation
			Entry("${{ as literal $ + {{ }}", "${{ Data.x }}", 1, []string{"Data.x"}),
		)

		DescribeTable("no matches",
			func(input string) {
				Expect(findTemplateExpressions(input)).To(BeEmpty())
			},
			Entry("plain text", "plain text"),
			Entry("unterminated {{ expression", "{{ unterminated"),
			Entry("unterminated ${ expression", "${ unterminated"),
			Entry("$ without brace", "costs $5"),
		)

		It("Should have correct full bounds for {{ }}", func() {
			matches := findTemplateExpressions("{{ Data.x }}")
			Expect(matches[0].fullStart).To(Equal(0))
			Expect(matches[0].fullEnd).To(Equal(12))
		})

		It("Should have correct full bounds for ${ }", func() {
			matches := findTemplateExpressions("${ Data.x }")
			Expect(matches[0].fullStart).To(Equal(0))
			Expect(matches[0].fullEnd).To(Equal(11))
		})

		It("Should start ${{ match at position 1", func() {
			matches := findTemplateExpressions("${{ Data.x }}")
			Expect(matches[0].fullStart).To(Equal(1))
		})
	})

	Describe("Quote-aware resolution", func() {
		DescribeTable("resolving expressions with delimiters in quotes",
			func(template string, expected string) {
				result, err := ResolveTemplateString(template, env)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(expected))
			},
			Entry("}} in resolved value", `{{ Data.val }}`, "closing braces: }}"),
			Entry("string concat with }}", `{{ Data.app_name + "}}" }}`, "myapp}}"),
		)

		BeforeEach(func() {
			env.Data["val"] = "closing braces: }}"
		})

		It("Should preserve type for single expression with }} in quotes", func() {
			env.Data["items"] = []any{"a", "b"}
			result, err := ResolveTemplateTyped(`{{ Data.items }}`, env)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal([]any{"a", "b"}))
		})
	})

	Describe("Dollar-brace syntax resolution", func() {
		DescribeTable("string resolution",
			func(template string, expected string) {
				result, err := ResolveTemplateString(template, env)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(expected))
			},
			Entry("simple ${ } expression", "${ Data.app_name }", "myapp"),
			Entry("${ } with surrounding text", "app: ${ Data.app_name } v${ Data.app_version }", "app: myapp v1.2.3"),
			Entry("mixed {{ }} and ${ }", "{{ Data.app_name }} on ${ Facts.hostname }", "myapp on test-server"),
			Entry("${ } with lookup", "${ lookup('data.app_name') }", "myapp"),
			Entry("$ without brace unchanged", "costs $5", "costs $5"),
			Entry("${{ as literal $ + {{ }}", "${{ Data.app_name }}", "$myapp"),
		)

		DescribeTable("type preservation",
			func(template string, expected any) {
				result, err := ResolveTemplateTyped(template, env)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(expected))
			},
			Entry("int via ${ }", "${ Data.port }", 8080),
			Entry("bool via ${ }", "${ Data.enabled }", true),
		)

		It("Should return matched true for ${ } with value", func() {
			result, matched, err := ResolveTemplateStringMatch("role:${ Data.app_name }", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(matched).To(BeTrue())
			Expect(result).To(Equal("role:myapp"))
		})

		It("Should return matched false for missing values with ${ }", func() {
			env.DefaultOnMissing = true
			result, matched, err := ResolveTemplateStringMatch("role:${ lookup('data.missing') }", env)
			Expect(err).ToNot(HaveOccurred())
			Expect(matched).To(BeFalse())
			Expect(result).To(Equal("role:"))
		})

		It("Should resolve ${ } in ExpandValuesRecursively", func() {
			input := map[string]any{
				"app": "${ Data.app_name }",
				"os":  "{{ Facts.os }}",
			}
			result, err := ExpandValuesRecursively(input, env)
			Expect(err).ToNot(HaveOccurred())
			m := result.(map[string]any)
			Expect(m["app"]).To(Equal("myapp"))
			Expect(m["os"]).To(Equal("linux"))
		})
	})
})
