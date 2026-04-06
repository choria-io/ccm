// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package templates

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type EmbeddedStruct struct {
	Base string `json:"base"`
}

type outerStruct struct {
	EmbeddedStruct `yaml:",inline"`
	Extra          string `json:"extra"`
}

type namedStringType string

type innerStruct struct {
	Command string `json:"command"`
	Name    string `json:"name"`
}

type innerWithUnexported struct {
	_      int    // unexported field to test CanSet() safety
	Public string `json:"public"`
}

type controlStruct struct {
	ManageIf string `json:"if"`
}

type testStruct struct {
	// Regular string fields
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`

	// Named string type
	Engine namedStringType `json:"engine"`

	// Skip via template tag
	SkipMe string `json:"skip_me" template:"-"`

	// Skip via json tag
	Internal string `json:"-"`

	// Deferred field
	Content string `json:"content" template:"deferred"`

	// Slice of strings
	Tags []string `json:"tags"`

	// Map string to string (values only)
	Headers map[string]string `json:"headers"`

	// Map string to any with resolve_keys
	Data map[string]any `json:"data" template:"resolve_keys"`

	// Nested struct
	Inner innerStruct `json:"inner"`

	// Pointer to struct (non-nil)
	Control *controlStruct `json:"control"`

	// Slice of structs
	Checks []innerStruct `json:"checks"`

	// Slice of pointer to structs
	Entries []*innerStruct `json:"entries"`

	// Slice of map[string]string
	Post []map[string]string `json:"post"`

	// Byte slice (should be skipped)
	RawData []byte `json:"raw_data"`

	// Non-string types (should be skipped)
	Count   int  `json:"count"`
	Enabled bool `json:"enabled"`

	// Any typed field (should be skipped)
	Port any `json:"port" template:"-"`

	// Struct with unexported fields
	Unexported innerWithUnexported `json:"unexported"`

	// Additional composite types for generic recursion
	SliceOfMapAny []map[string]any       `json:"slice_of_map_any"`
	MapOfSlice    map[string][]string    `json:"map_of_slice"`
	MapOfStruct   map[string]innerStruct `json:"map_of_struct"`
	AnyField      any                    `json:"any_field"`
	SliceOfAny    []any                  `json:"slice_of_any"`
}

var _ = Describe("ResolveStructTemplates", func() {
	var env *Env

	BeforeEach(func() {
		env = &Env{
			Facts: map[string]any{
				"os":       "linux",
				"hostname": "test-server",
			},
			Data: map[string]any{
				"app":     "myapp",
				"version": "1.2.3",
				"region":  "us-east",
			},
		}
	})

	It("requires a pointer to a struct", func() {
		s := testStruct{}
		Expect(ResolveStructTemplates(s, env, false)).To(MatchError(ContainSubstring("requires a pointer to a struct")))
	})

	It("resolves string fields", func() {
		s := testStruct{
			Title:    "{{ lookup('data.app') }}",
			Subtitle: "version {{ lookup('data.version') }}",
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Title).To(Equal("myapp"))
		Expect(s.Subtitle).To(Equal("version 1.2.3"))
	})

	It("resolves named string type fields", func() {
		s := testStruct{
			Engine: namedStringType("{{ lookup('data.app') }}"),
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(string(s.Engine)).To(Equal("myapp"))
	})

	It("skips empty strings", func() {
		s := testStruct{Title: ""}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Title).To(Equal(""))
	})

	It("skips fields tagged template:\"-\"", func() {
		s := testStruct{
			SkipMe: "{{ lookup('data.app') }}",
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.SkipMe).To(Equal("{{ lookup('data.app') }}"))
	})

	It("skips fields tagged json:\"-\"", func() {
		s := testStruct{
			Internal: "{{ lookup('data.app') }}",
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Internal).To(Equal("{{ lookup('data.app') }}"))
	})

	It("skips deferred fields in non-deferred mode", func() {
		s := testStruct{
			Title:   "{{ lookup('data.app') }}",
			Content: "{{ lookup('data.version') }}",
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Title).To(Equal("myapp"))
		Expect(s.Content).To(Equal("{{ lookup('data.version') }}"))
	})

	It("resolves only deferred fields in deferred mode", func() {
		s := testStruct{
			Title:   "{{ lookup('data.app') }}",
			Content: "{{ lookup('data.version') }}",
		}
		Expect(ResolveStructTemplates(&s, env, true)).To(Succeed())
		Expect(s.Title).To(Equal("{{ lookup('data.app') }}"))
		Expect(s.Content).To(Equal("1.2.3"))
	})

	It("resolves []string fields", func() {
		s := testStruct{
			Tags: []string{"{{ lookup('data.app') }}", "static", "{{ lookup('data.region') }}"},
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Tags).To(Equal([]string{"myapp", "static", "us-east"}))
	})

	It("resolves map[string]string values without resolving keys", func() {
		s := testStruct{
			Headers: map[string]string{
				"X-App":    "{{ lookup('data.app') }}",
				"X-Region": "{{ lookup('data.region') }}",
			},
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Headers).To(HaveKeyWithValue("X-App", "myapp"))
		Expect(s.Headers).To(HaveKeyWithValue("X-Region", "us-east"))
	})

	It("resolves map[string]any with resolve_keys tag", func() {
		s := testStruct{
			Data: map[string]any{
				"{{ lookup('data.app') }}": "{{ lookup('data.version') }}",
				"static":                   42,
				"nested": map[string]any{
					"inner": "{{ lookup('data.region') }}",
				},
			},
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Data).To(HaveKeyWithValue("myapp", "1.2.3"))
		Expect(s.Data).To(HaveKeyWithValue("static", 42))
		Expect(s.Data).To(HaveKey("nested"))
		nested := s.Data["nested"].(map[string]any)
		Expect(nested["inner"]).To(Equal("us-east"))
	})

	It("resolves nested struct fields", func() {
		s := testStruct{
			Inner: innerStruct{
				Command: "{{ lookup('data.app') }} start",
				Name:    "{{ lookup('facts.hostname') }}",
			},
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Inner.Command).To(Equal("myapp start"))
		Expect(s.Inner.Name).To(Equal("test-server"))
	})

	It("resolves pointer-to-struct fields", func() {
		s := testStruct{
			Control: &controlStruct{
				ManageIf: "{{ lookup('facts.os') }}",
			},
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Control.ManageIf).To(Equal("linux"))
	})

	It("handles nil pointer-to-struct fields", func() {
		s := testStruct{
			Control: nil,
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Control).To(BeNil())
	})

	It("resolves slice of structs", func() {
		s := testStruct{
			Checks: []innerStruct{
				{Command: "{{ lookup('data.app') }}", Name: "check1"},
				{Command: "static", Name: "{{ lookup('facts.hostname') }}"},
			},
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Checks[0].Command).To(Equal("myapp"))
		Expect(s.Checks[1].Name).To(Equal("test-server"))
	})

	It("resolves slice of pointer to structs", func() {
		s := testStruct{
			Entries: []*innerStruct{
				{Command: "{{ lookup('data.app') }}", Name: "e1"},
				nil,
				{Command: "static", Name: "{{ lookup('data.region') }}"},
			},
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Entries[0].Command).To(Equal("myapp"))
		Expect(s.Entries[1]).To(BeNil())
		Expect(s.Entries[2].Name).To(Equal("us-east"))
	})

	It("resolves slice of map[string]string", func() {
		s := testStruct{
			Post: []map[string]string{
				{"cmd": "{{ lookup('data.app') }} reload"},
			},
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Post[0]).To(HaveKeyWithValue("cmd", "myapp reload"))
	})

	It("skips []byte fields", func() {
		raw := []byte("{{ lookup('data.app') }}")
		s := testStruct{
			RawData: raw,
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.RawData).To(Equal(raw))
	})

	It("skips non-string types", func() {
		s := testStruct{
			Count:   42,
			Enabled: true,
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Count).To(Equal(42))
		Expect(s.Enabled).To(BeTrue())
	})

	It("handles unexported fields without panicking", func() {
		s := testStruct{
			Unexported: innerWithUnexported{
				Public: "{{ lookup('data.app') }}",
			},
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Unexported.Public).To(Equal("myapp"))
	})

	It("leaves strings without templates unchanged", func() {
		s := testStruct{
			Title: "no templates here",
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
		Expect(s.Title).To(Equal("no templates here"))
	})

	It("wraps errors with field name", func() {
		s := testStruct{
			Title: "{{ invalidFunc() }}",
		}
		err := ResolveStructTemplates(&s, env, false)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Title"))
	})

	It("handles nil slices and maps", func() {
		s := testStruct{}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
	})

	It("handles empty slices and maps", func() {
		s := testStruct{
			Tags:    []string{},
			Headers: map[string]string{},
			Data:    map[string]any{},
		}
		Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
	})

	Describe("embedded structs", func() {
		It("resolves fields in embedded structs", func() {
			s := outerStruct{
				EmbeddedStruct: EmbeddedStruct{Base: "{{ lookup('data.app') }}"},
				Extra:          "{{ lookup('data.version') }}",
			}
			Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
			Expect(s.Base).To(Equal("myapp"))
			Expect(s.Extra).To(Equal("1.2.3"))
		})
	})

	Describe("type-preserving resolution in map[string]any", func() {
		It("preserves map type when template resolves to a map", func() {
			env.Data["user"] = map[string]any{"name": "John", "age": 30}

			s := testStruct{
				Data: map[string]any{
					"user": "{{ Data.user }}",
				},
			}
			Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
			user, ok := s.Data["user"].(map[string]any)
			Expect(ok).To(BeTrue(), "expected map[string]any, got %T", s.Data["user"])
			Expect(user["name"]).To(Equal("John"))
			Expect(user["age"]).To(Equal(30))
		})

		It("preserves integer type when template resolves to an integer", func() {
			env.Data["port"] = 8080

			s := testStruct{
				Data: map[string]any{
					"port": "{{ Data.port }}",
				},
			}
			Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
			Expect(s.Data["port"]).To(BeAssignableToTypeOf(int(0)))
			Expect(s.Data["port"]).To(Equal(8080))
		})

		It("preserves boolean type when template resolves to a boolean", func() {
			env.Data["enabled"] = true

			s := testStruct{
				Data: map[string]any{
					"enabled": "{{ Data.enabled }}",
				},
			}
			Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
			Expect(s.Data["enabled"]).To(BeAssignableToTypeOf(true))
			Expect(s.Data["enabled"]).To(Equal(true))
		})

		It("returns string when template has surrounding text", func() {
			env.Data["port"] = 8080

			s := testStruct{
				Data: map[string]any{
					"addr": "localhost:{{ Data.port }}",
				},
			}
			Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
			Expect(s.Data["addr"]).To(BeAssignableToTypeOf(""))
			Expect(s.Data["addr"]).To(Equal("localhost:8080"))
		})

		It("preserves nested map type through multiple levels", func() {
			env.Data["config"] = map[string]any{
				"db": map[string]any{
					"host": "localhost",
					"port": 5432,
				},
			}

			s := testStruct{
				Data: map[string]any{
					"config": "{{ Data.config }}",
				},
			}
			Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
			config, ok := s.Data["config"].(map[string]any)
			Expect(ok).To(BeTrue(), "expected map[string]any, got %T", s.Data["config"])
			db, ok := config["db"].(map[string]any)
			Expect(ok).To(BeTrue(), "expected nested map[string]any, got %T", config["db"])
			Expect(db["host"]).To(Equal("localhost"))
			Expect(db["port"]).To(Equal(5432))
		})

		It("preserves slice type when template resolves to a slice", func() {
			env.Data["tags"] = []any{"web", "prod"}

			s := testStruct{
				Data: map[string]any{
					"tags": "{{ Data.tags }}",
				},
			}
			Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
			tags, ok := s.Data["tags"].([]any)
			Expect(ok).To(BeTrue(), "expected []any, got %T", s.Data["tags"])
			Expect(tags).To(Equal([]any{"web", "prod"}))
		})

		It("preserves types in []map[string]any", func() {
			env.Data["user"] = map[string]any{"name": "John"}

			s := testStruct{
				SliceOfMapAny: []map[string]any{
					{"user": "{{ Data.user }}"},
				},
			}
			Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
			user, ok := s.SliceOfMapAny[0]["user"].(map[string]any)
			Expect(ok).To(BeTrue(), "expected map[string]any, got %T", s.SliceOfMapAny[0]["user"])
			Expect(user["name"]).To(Equal("John"))
		})
	})

	Describe("generic composite type recursion", func() {
		It("resolves []map[string]any", func() {
			s := testStruct{
				SliceOfMapAny: []map[string]any{
					{"key": "{{ lookup('data.app') }}", "num": 42},
					{"nested": map[string]any{"deep": "{{ lookup('data.region') }}"}},
				},
			}
			Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
			Expect(s.SliceOfMapAny[0]["key"]).To(Equal("myapp"))
			Expect(s.SliceOfMapAny[0]["num"]).To(Equal(42))
			nested := s.SliceOfMapAny[1]["nested"].(map[string]any)
			Expect(nested["deep"]).To(Equal("us-east"))
		})

		It("resolves map[string][]string", func() {
			s := testStruct{
				MapOfSlice: map[string][]string{
					"cmds": {"{{ lookup('data.app') }}", "static", "{{ lookup('data.region') }}"},
				},
			}
			Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
			Expect(s.MapOfSlice["cmds"]).To(Equal([]string{"myapp", "static", "us-east"}))
		})

		It("resolves any-typed fields holding strings", func() {
			s := testStruct{
				AnyField: "{{ lookup('data.app') }}",
			}
			Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
			Expect(s.AnyField).To(Equal("myapp"))
		})

		It("resolves any-typed fields holding maps", func() {
			s := testStruct{
				AnyField: map[string]any{
					"key": "{{ lookup('data.version') }}",
				},
			}
			Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
			m := s.AnyField.(map[string]any)
			Expect(m["key"]).To(Equal("1.2.3"))
		})

		It("resolves []any with mixed types", func() {
			s := testStruct{
				SliceOfAny: []any{
					"{{ lookup('data.app') }}",
					42,
					map[string]any{"inner": "{{ lookup('data.region') }}"},
				},
			}
			Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
			Expect(s.SliceOfAny[0]).To(Equal("myapp"))
			Expect(s.SliceOfAny[1]).To(Equal(42))
			m := s.SliceOfAny[2].(map[string]any)
			Expect(m["inner"]).To(Equal("us-east"))
		})

		It("leaves non-string any values unchanged", func() {
			s := testStruct{
				AnyField: 42,
			}
			Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
			Expect(s.AnyField).To(Equal(42))
		})

		It("handles nil any fields", func() {
			s := testStruct{
				AnyField: nil,
			}
			Expect(ResolveStructTemplates(&s, env, false)).To(Succeed())
			Expect(s.AnyField).To(BeNil())
		})
	})
})
