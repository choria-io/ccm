// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"time"

	"github.com/goccy/go-yaml"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RegistrationTTL", func() {
	Describe("Constructors", func() {
		It("should create a never-expire TTL", func() {
			ttl := NeverExpire()
			Expect(ttl.IsNever()).To(BeTrue())
			Expect(ttl.Duration()).To(Equal(time.Duration(0)))
			Expect(ttl.String()).To(Equal("never"))
		})

		It("should create a duration TTL", func() {
			ttl := NewRegistrationTTL(10 * time.Minute)
			Expect(ttl.IsNever()).To(BeFalse())
			Expect(ttl.Duration()).To(Equal(10 * time.Minute))
			Expect(ttl.String()).To(Equal("10m0s"))
		})
	})

	Describe("ParseRegistrationTTL", func() {
		It("should parse never", func() {
			ttl, err := ParseRegistrationTTL("never")
			Expect(err).ToNot(HaveOccurred())
			Expect(ttl.IsNever()).To(BeTrue())
		})

		It("should parse durations", func() {
			ttl, err := ParseRegistrationTTL("10m")
			Expect(err).ToNot(HaveOccurred())
			Expect(ttl.Duration()).To(Equal(10 * time.Minute))
		})

		It("should reject invalid strings", func() {
			_, err := ParseRegistrationTTL("banana")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid TTL duration"))
		})
	})

	Describe("Nil pointer safety", func() {
		It("should handle nil pointer", func() {
			var ttl *RegistrationTTL
			Expect(ttl.IsNever()).To(BeFalse())
			Expect(ttl.Duration()).To(Equal(time.Duration(0)))
			Expect(ttl.String()).To(Equal(""))
		})
	})

	Describe("JSON", func() {
		type wrapper struct {
			TTL *RegistrationTTL `json:"ttl,omitempty"`
		}

		Describe("UnmarshalJSON", func() {
			It("should parse never", func() {
				var w wrapper
				err := json.Unmarshal([]byte(`{"ttl":"never"}`), &w)
				Expect(err).ToNot(HaveOccurred())
				Expect(w.TTL).ToNot(BeNil())
				Expect(w.TTL.IsNever()).To(BeTrue())
			})

			It("should parse duration strings", func() {
				var w wrapper
				err := json.Unmarshal([]byte(`{"ttl":"10m"}`), &w)
				Expect(err).ToNot(HaveOccurred())
				Expect(w.TTL).ToNot(BeNil())
				Expect(w.TTL.Duration()).To(Equal(10 * time.Minute))
			})

			It("should parse numeric nanoseconds for backward compatibility", func() {
				var w wrapper
				err := json.Unmarshal([]byte(`{"ttl":30000000000}`), &w)
				Expect(err).ToNot(HaveOccurred())
				Expect(w.TTL).ToNot(BeNil())
				Expect(w.TTL.Duration()).To(Equal(30 * time.Second))
			})

			It("should handle null", func() {
				var w wrapper
				err := json.Unmarshal([]byte(`{"ttl":null}`), &w)
				Expect(err).ToNot(HaveOccurred())
				Expect(w.TTL).To(BeNil())
			})

			It("should handle omitted field", func() {
				var w wrapper
				err := json.Unmarshal([]byte(`{}`), &w)
				Expect(err).ToNot(HaveOccurred())
				Expect(w.TTL).To(BeNil())
			})

			It("should reject invalid strings", func() {
				var w wrapper
				err := json.Unmarshal([]byte(`{"ttl":"banana"}`), &w)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid TTL duration"))
			})
		})

		Describe("MarshalJSON", func() {
			It("should marshal never", func() {
				w := wrapper{TTL: NeverExpire()}
				data, err := json.Marshal(w)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(data)).To(Equal(`{"ttl":"never"}`))
			})

			It("should marshal duration", func() {
				w := wrapper{TTL: NewRegistrationTTL(30 * time.Second)}
				data, err := json.Marshal(w)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(data)).To(Equal(`{"ttl":"30s"}`))
			})

			It("should omit nil", func() {
				w := wrapper{}
				data, err := json.Marshal(w)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(data)).To(Equal(`{}`))
			})
		})

		Describe("Round trip", func() {
			It("should round-trip never", func() {
				w := wrapper{TTL: NeverExpire()}
				data, err := json.Marshal(w)
				Expect(err).ToNot(HaveOccurred())

				var w2 wrapper
				err = json.Unmarshal(data, &w2)
				Expect(err).ToNot(HaveOccurred())
				Expect(w2.TTL.IsNever()).To(BeTrue())
			})

			It("should round-trip duration", func() {
				w := wrapper{TTL: NewRegistrationTTL(5 * time.Minute)}
				data, err := json.Marshal(w)
				Expect(err).ToNot(HaveOccurred())

				var w2 wrapper
				err = json.Unmarshal(data, &w2)
				Expect(err).ToNot(HaveOccurred())
				Expect(w2.TTL.Duration()).To(Equal(5 * time.Minute))
			})
		})
	})

	Describe("YAML", func() {
		type wrapper struct {
			TTL *RegistrationTTL `yaml:"ttl,omitempty"`
		}

		Describe("UnmarshalYAML", func() {
			It("should parse never", func() {
				var w wrapper
				err := yaml.Unmarshal([]byte("ttl: never"), &w)
				Expect(err).ToNot(HaveOccurred())
				Expect(w.TTL).ToNot(BeNil())
				Expect(w.TTL.IsNever()).To(BeTrue())
			})

			It("should parse duration strings", func() {
				var w wrapper
				err := yaml.Unmarshal([]byte("ttl: 10m"), &w)
				Expect(err).ToNot(HaveOccurred())
				Expect(w.TTL).ToNot(BeNil())
				Expect(w.TTL.Duration()).To(Equal(10 * time.Minute))
			})

			It("should parse complex durations", func() {
				var w wrapper
				err := yaml.Unmarshal([]byte("ttl: 1h30m"), &w)
				Expect(err).ToNot(HaveOccurred())
				Expect(w.TTL).ToNot(BeNil())
				Expect(w.TTL.Duration()).To(Equal(90 * time.Minute))
			})

			It("should reject invalid strings", func() {
				var w wrapper
				err := yaml.Unmarshal([]byte("ttl: banana"), &w)
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("MarshalYAML", func() {
			It("should marshal never", func() {
				w := wrapper{TTL: NeverExpire()}
				data, err := yaml.Marshal(w)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(data)).To(ContainSubstring("never"))
			})

			It("should marshal duration", func() {
				w := wrapper{TTL: NewRegistrationTTL(30 * time.Second)}
				data, err := yaml.Marshal(w)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(data)).To(ContainSubstring("30s"))
			})
		})
	})
})
