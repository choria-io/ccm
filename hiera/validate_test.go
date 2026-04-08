// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package hiera

import (
	"github.com/goccy/go-yaml"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model/modelmocks"
)

var _ = Describe("ParseAnnotations", func() {
	var (
		mockctl *gomock.Controller
		logger  *modelmocks.MockLogger
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		logger = modelmocks.NewMockLogger(mockctl)
		logger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	It("extracts @require from head comments", func() {
		cm := yaml.CommentMap{
			"$.data.user": []*yaml.Comment{
				yaml.HeadComment(" @require"),
			},
		}

		rules := ParseAnnotations(cm, "data", logger)
		Expect(rules).To(HaveLen(1))
		Expect(rules[0].Key).To(Equal("user"))
		Expect(rules[0].Required).To(BeTrue())
		Expect(rules[0].Validation).To(BeEmpty())
	})

	It("extracts @required alias", func() {
		cm := yaml.CommentMap{
			"$.data.user": []*yaml.Comment{
				yaml.HeadComment(" @required"),
			},
		}

		rules := ParseAnnotations(cm, "data", logger)
		Expect(rules).To(HaveLen(1))
		Expect(rules[0].Required).To(BeTrue())
	})

	It("extracts @validate expr", func() {
		cm := yaml.CommentMap{
			"$.data.command": []*yaml.Comment{
				yaml.HeadComment(" @validate isShellSafe(value)"),
			},
		}

		rules := ParseAnnotations(cm, "data", logger)
		Expect(rules).To(HaveLen(1))
		Expect(rules[0].Key).To(Equal("command"))
		Expect(rules[0].Required).To(BeFalse())
		Expect(rules[0].Validation).To(Equal("isShellSafe(value)"))
	})

	It("handles multiple annotations on the same key", func() {
		cm := yaml.CommentMap{
			"$.data.listen": []*yaml.Comment{
				yaml.HeadComment(" @require", " @validate isIPv4(value)"),
			},
		}

		rules := ParseAnnotations(cm, "data", logger)
		Expect(rules).To(HaveLen(1))
		Expect(rules[0].Key).To(Equal("listen"))
		Expect(rules[0].Required).To(BeTrue())
		Expect(rules[0].Validation).To(Equal("isIPv4(value)"))
	})

	It("handles nested paths", func() {
		cm := yaml.CommentMap{
			"$.data.web.listen_address": []*yaml.Comment{
				yaml.HeadComment(" @require"),
			},
		}

		rules := ParseAnnotations(cm, "data", logger)
		Expect(rules).To(HaveLen(1))
		Expect(rules[0].Key).To(Equal("web.listen_address"))
		Expect(rules[0].Required).To(BeTrue())
	})

	It("ignores human comments without @ prefix", func() {
		cm := yaml.CommentMap{
			"$.data.user": []*yaml.Comment{
				yaml.HeadComment(" The username to use", " @require"),
			},
		}

		rules := ParseAnnotations(cm, "data", logger)
		Expect(rules).To(HaveLen(1))
		Expect(rules[0].Required).To(BeTrue())
	})

	It("ignores entries outside the data section", func() {
		cm := yaml.CommentMap{
			"$.overrides.production.user": []*yaml.Comment{
				yaml.HeadComment(" @require"),
			},
			"$.other.key": []*yaml.Comment{
				yaml.HeadComment(" @require"),
			},
		}

		rules := ParseAnnotations(cm, "data", logger)
		Expect(rules).To(BeEmpty())
	})

	It("skips array element paths", func() {
		cm := yaml.CommentMap{
			"$.data.packages[0]": []*yaml.Comment{
				yaml.HeadComment(" @require"),
			},
		}

		rules := ParseAnnotations(cm, "data", logger)
		Expect(rules).To(BeEmpty())
	})

	It("warns on unrecognized @ directives", func() {
		logger = modelmocks.NewMockLogger(mockctl)
		logger.EXPECT().Warn("Unrecognized annotation in hiera data", "key", "user", "annotation", "@requiired").Times(1)

		cm := yaml.CommentMap{
			"$.data.user": []*yaml.Comment{
				yaml.HeadComment(" @requiired"),
			},
		}

		rules := ParseAnnotations(cm, "data", logger)
		Expect(rules).To(BeEmpty())
	})

	It("respects a custom dataKey", func() {
		cm := yaml.CommentMap{
			"$.config.user": []*yaml.Comment{
				yaml.HeadComment(" @require"),
			},
			"$.data.user": []*yaml.Comment{
				yaml.HeadComment(" @require"),
			},
		}

		rules := ParseAnnotations(cm, "config", logger)
		Expect(rules).To(HaveLen(1))
		Expect(rules[0].Key).To(Equal("user"))
	})

	It("handles nil logger without panic", func() {
		cm := yaml.CommentMap{
			"$.data.user": []*yaml.Comment{
				yaml.HeadComment(" @unknown_directive"),
			},
		}

		Expect(func() {
			ParseAnnotations(cm, "data", nil)
		}).NotTo(Panic())
	})
})

var _ = Describe("ValidateData", func() {
	Describe("@require", func() {
		It("passes for non-nil non-empty values", func() {
			rules := []ValidationRule{{Key: "user", Required: true}}
			data := map[string]any{"user": "bob"}

			err := ValidateData(data, rules)
			Expect(err).NotTo(HaveOccurred())
		})

		It("fails for nil values", func() {
			rules := []ValidationRule{{Key: "user", Required: true}}
			data := map[string]any{"user": nil}

			err := ValidateData(data, rules)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("user"))
			Expect(err.Error()).To(ContainSubstring("required"))
		})

		It("fails for missing keys", func() {
			rules := []ValidationRule{{Key: "user", Required: true}}
			data := map[string]any{}

			err := ValidateData(data, rules)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("user"))
		})

		It("fails for empty string", func() {
			rules := []ValidationRule{{Key: "user", Required: true}}
			data := map[string]any{"user": ""}

			err := ValidateData(data, rules)
			Expect(err).To(HaveOccurred())
		})

		It("passes for false", func() {
			rules := []ValidationRule{{Key: "tls", Required: true}}
			data := map[string]any{"tls": false}

			err := ValidateData(data, rules)
			Expect(err).NotTo(HaveOccurred())
		})

		It("passes for zero", func() {
			rules := []ValidationRule{{Key: "port", Required: true}}
			data := map[string]any{"port": 0}

			err := ValidateData(data, rules)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("@validate", func() {
		It("passes for valid values", func() {
			rules := []ValidationRule{{Key: "addr", Validation: "isIPv4(value)"}}
			data := map[string]any{"addr": "10.0.0.1"}

			err := ValidateData(data, rules)
			Expect(err).NotTo(HaveOccurred())
		})

		It("fails for invalid values", func() {
			rules := []ValidationRule{{Key: "addr", Validation: "isIPv4(value)"}}
			data := map[string]any{"addr": "not-an-ip"}

			err := ValidateData(data, rules)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("addr"))
		})

		It("skips validation for nil values", func() {
			rules := []ValidationRule{{Key: "addr", Validation: "isIPv4(value)"}}
			data := map[string]any{"addr": nil}

			err := ValidateData(data, rules)
			Expect(err).NotTo(HaveOccurred())
		})

		It("converts non-string scalars to string before validating", func() {
			rules := []ValidationRule{{Key: "port", Validation: "isInt(value)"}}
			data := map[string]any{"port": 8080}

			err := ValidateData(data, rules)
			Expect(err).NotTo(HaveOccurred())
		})

		It("skips validation for map values", func() {
			rules := []ValidationRule{{Key: "web", Validation: "isIPv4(value)"}}
			data := map[string]any{"web": map[string]any{"port": 80}}

			err := ValidateData(data, rules)
			Expect(err).NotTo(HaveOccurred())
		})

		It("skips validation for slice values", func() {
			rules := []ValidationRule{{Key: "items", Validation: "isShellSafe(value)"}}
			data := map[string]any{"items": []any{"a", "b"}}

			err := ValidateData(data, rules)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("combined rules", func() {
		It("short-circuits @validate when @require fails on nil", func() {
			rules := []ValidationRule{{Key: "addr", Required: true, Validation: "isIPv4(value)"}}
			data := map[string]any{"addr": nil}

			err := ValidateData(data, rules)
			Expect(err).To(HaveOccurred())
			// Should have exactly one error (require), not two
			Expect(err.Error()).To(ContainSubstring("required"))
			Expect(err.Error()).NotTo(ContainSubstring("isIPv4"))
		})

		It("runs @validate when @require passes", func() {
			rules := []ValidationRule{{Key: "addr", Required: true, Validation: "isIPv4(value)"}}
			data := map[string]any{"addr": "not-an-ip"}

			err := ValidateData(data, rules)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("isIPv4"))
		})

		It("passes when both @require and @validate succeed", func() {
			rules := []ValidationRule{{Key: "addr", Required: true, Validation: "isIPv4(value)"}}
			data := map[string]any{"addr": "10.0.0.1"}

			err := ValidateData(data, rules)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("nested key lookup", func() {
		It("validates nested keys", func() {
			rules := []ValidationRule{{Key: "web.listen_address", Required: true, Validation: "isIPv4(value)"}}
			data := map[string]any{
				"web": map[string]any{
					"listen_address": "10.0.0.1",
				},
			}

			err := ValidateData(data, rules)
			Expect(err).NotTo(HaveOccurred())
		})

		It("fails for missing nested keys", func() {
			rules := []ValidationRule{{Key: "web.listen_address", Required: true}}
			data := map[string]any{
				"web": map[string]any{},
			}

			err := ValidateData(data, rules)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("multi-error aggregation", func() {
		It("collects all failures", func() {
			rules := []ValidationRule{
				{Key: "user", Required: true},
				{Key: "addr", Required: true, Validation: "isIPv4(value)"},
			}
			data := map[string]any{
				"user": nil,
				"addr": "not-an-ip",
			}

			err := ValidateData(data, rules)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("user"))
			Expect(err.Error()).To(ContainSubstring("addr"))
		})
	})

	It("returns nil when there are no rules", func() {
		data := map[string]any{"user": "bob"}

		err := ValidateData(data, nil)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("isRequiredZero", func() {
	It("returns true for nil", func() {
		Expect(isRequiredZero(nil)).To(BeTrue())
	})

	It("returns true for empty string", func() {
		Expect(isRequiredZero("")).To(BeTrue())
	})

	It("returns false for false", func() {
		Expect(isRequiredZero(false)).To(BeFalse())
	})

	It("returns false for zero", func() {
		Expect(isRequiredZero(0)).To(BeFalse())
	})

	It("returns false for a non-empty string", func() {
		Expect(isRequiredZero("hello")).To(BeFalse())
	})
})
