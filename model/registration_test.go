// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/choria-io/ccm/templates"
)

var _ = Describe("RegistrationEntry", func() {
	Describe("SubjectAddress", func() {
		It("should replace dots with underscores for IPv4", func() {
			entry := &RegistrationEntry{Address: "10.0.0.1"}
			Expect(entry.SubjectAddress()).To(Equal("10_0_0_1"))
		})

		It("should return IPv6 addresses unchanged", func() {
			entry := &RegistrationEntry{Address: "::1"}
			Expect(entry.SubjectAddress()).To(Equal("::1"))
		})

		It("should handle full IPv6 addresses", func() {
			entry := &RegistrationEntry{Address: "2001:db8::1"}
			Expect(entry.SubjectAddress()).To(Equal("2001:db8::1"))
		})
	})

	Describe("NewRegistrationEntry", func() {
		It("should create an entry with all fields set", func() {
			entry, err := NewRegistrationEntry("production", "web", "tcp", "192.168.1.1", 8080, 1, NewRegistrationTTL(30*time.Second))
			Expect(err).ToNot(HaveOccurred())
			Expect(entry.Cluster).To(Equal("production"))
			Expect(entry.Service).To(Equal("web"))
			Expect(entry.Protocol).To(Equal("tcp"))
			Expect(entry.Address).To(Equal("192.168.1.1"))
			Expect(entry.Port).To(Equal(int64(8080)))
			Expect(entry.Priority).To(Equal(int64(1)))
			Expect(entry.TTL.Duration()).To(Equal(30 * time.Second))
		})

		It("should not set annotations", func() {
			entry, err := NewRegistrationEntry("production", "web", "tcp", "192.168.1.1", 8080, 1, NewRegistrationTTL(time.Minute))
			Expect(err).ToNot(HaveOccurred())
			Expect(entry.Annotations).To(BeNil())
		})
	})

	Describe("ResolveTemplates", func() {
		var env *templates.Env

		BeforeEach(func() {
			env = &templates.Env{
				Facts: map[string]any{
					"hostname": "web01",
					"ip":       "10.0.0.1",
					"cluster":  "production",
					"port":     int64(9090),
				},
				Data: map[string]any{},
			}
		})

		It("should resolve Address templates", func() {
			entry := &RegistrationEntry{
				Cluster:  "production",
				Service:  "web",
				Protocol: "tcp",
				Address:  "{{ Facts.ip }}",
				Port:     int64(8080),
				Priority: 1,
			}

			err := entry.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(entry.Address).To(Equal("10.0.0.1"))
		})

		It("should resolve cluster templates", func() {
			entry := &RegistrationEntry{
				Cluster:  "{{ Facts.cluster }}",
				Service:  "web",
				Protocol: "tcp",
				Address:  "10.0.0.1",
				Port:     int64(8080),
				Priority: 1,
			}

			err := entry.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(entry.Cluster).To(Equal("production"))
		})

		It("should resolve annotation templates", func() {
			entry := &RegistrationEntry{
				Cluster:  "production",
				Service:  "web",
				Protocol: "tcp",
				Address:  "10.0.0.1",
				Port:     int64(8080),
				Priority: 1,
				Annotations: map[string]string{
					"host": "{{ Facts.hostname }}",
				},
			}

			err := entry.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(entry.Annotations["host"]).To(Equal("web01"))
		})

		It("should resolve string port templates to int64", func() {
			entry := &RegistrationEntry{
				Cluster:  "production",
				Service:  "web",
				Protocol: "tcp",
				Address:  "10.0.0.1",
				Port:     "{{ Facts.port }}",
				Priority: 1,
			}

			err := entry.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(entry.Port).To(Equal(int64(9090)))
		})

		It("should accept int64 port without change", func() {
			entry := &RegistrationEntry{
				Cluster:  "production",
				Service:  "web",
				Protocol: "tcp",
				Address:  "10.0.0.1",
				Port:     int64(8080),
				Priority: 1,
			}

			err := entry.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(entry.Port).To(Equal(int64(8080)))
		})

		It("should accept nil port", func() {
			entry := &RegistrationEntry{
				Cluster:  "production",
				Service:  "web",
				Protocol: "tcp",
				Address:  "10.0.0.1",
				Port:     nil,
				Priority: 1,
			}

			err := entry.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(entry.Port).To(BeNil())
		})

		It("should reject port that resolves to non-integer", func() {
			entry := &RegistrationEntry{
				Cluster:  "production",
				Service:  "web",
				Protocol: "tcp",
				Address:  "10.0.0.1",
				Port:     "{{ Facts.hostname }}",
				Priority: 1,
			}

			err := entry.ResolveTemplates(env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("port must be an integer number"))
		})

		It("should reject unsupported port types", func() {
			entry := &RegistrationEntry{
				Cluster:  "production",
				Service:  "web",
				Protocol: "tcp",
				Address:  "10.0.0.1",
				Port:     float64(8080),
				Priority: 1,
			}

			err := entry.ResolveTemplates(env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("port must be an integer number"))
		})

		It("should resolve multiple fields simultaneously", func() {
			entry := &RegistrationEntry{
				Cluster:  "{{ Facts.cluster }}",
				Service:  "web",
				Protocol: "tcp",
				Address:  "{{ Facts.ip }}",
				Port:     "{{ Facts.port }}",
				Priority: 1,
				Annotations: map[string]string{
					"host": "{{ Facts.hostname }}",
				},
			}

			err := entry.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(entry.Cluster).To(Equal("production"))
			Expect(entry.Address).To(Equal("10.0.0.1"))
			Expect(entry.Port).To(Equal(int64(9090)))
			Expect(entry.Annotations["host"]).To(Equal("web01"))
		})
	})

	Describe("PrometheusFileSD", func() {
		It("should produce valid file SD JSON for prometheus service entries", func() {
			entries := RegistrationEntries{
				{Cluster: "dev", Service: "prometheus", Protocol: "tcp", Address: "10.0.0.1", Port: int64(8080)},
				{Cluster: "dev", Service: "prometheus", Protocol: "tcp", Address: "10.0.0.2", Port: int64(8080)},
				{Cluster: "dev", Service: "prometheus", Protocol: "tcp", Address: "10.0.0.3", Port: int64(9090)},
			}

			result, err := entries.PrometheusFileSD()
			Expect(err).ToNot(HaveOccurred())

			var parsed []map[string]any
			Expect(json.Unmarshal([]byte(result), &parsed)).To(Succeed())
			Expect(parsed).To(HaveLen(1))

			Expect(parsed[0]["labels"]).To(Equal(map[string]any{
				"cluster": "dev", "service": "prometheus", "protocol": "tcp",
			}))
			Expect(parsed[0]["targets"]).To(Equal([]any{"10.0.0.1:8080", "10.0.0.2:8080", "10.0.0.3:9090"}))
		})

		It("should include prometheus protocol entries by default", func() {
			entries := RegistrationEntries{
				{Cluster: "dev", Service: "web", Protocol: "prometheus", Address: "10.0.0.1", Port: int64(8080)},
				{Cluster: "dev", Service: "web", Protocol: "prometheus", Address: "10.0.0.2", Port: int64(8080)},
			}

			result, err := entries.PrometheusFileSD()
			Expect(err).ToNot(HaveOccurred())

			var parsed []map[string]any
			Expect(json.Unmarshal([]byte(result), &parsed)).To(Succeed())
			Expect(parsed).To(HaveLen(1))
			Expect(parsed[0]["targets"]).To(Equal([]any{"10.0.0.1:8080", "10.0.0.2:8080"}))
		})

		It("should skip prometheus service entries explicitly opted out", func() {
			entries := RegistrationEntries{
				{Cluster: "dev", Service: "prometheus", Protocol: "tcp", Address: "10.0.0.1", Port: int64(8080),
					Annotations: map[string]string{"prometheus.io/scrape": "false"}},
				{Cluster: "dev", Service: "prometheus", Protocol: "tcp", Address: "10.0.0.2", Port: int64(8080)},
			}

			result, err := entries.PrometheusFileSD()
			Expect(err).ToNot(HaveOccurred())

			var parsed []map[string]any
			Expect(json.Unmarshal([]byte(result), &parsed)).To(Succeed())
			Expect(parsed).To(HaveLen(1))
			Expect(parsed[0]["targets"]).To(Equal([]any{"10.0.0.2:8080"}))
		})

		It("should require explicit opt-in for non-prometheus entries", func() {
			entries := RegistrationEntries{
				{Cluster: "dev", Service: "web", Protocol: "tcp", Address: "10.0.0.1", Port: int64(8080)},
				{Cluster: "dev", Service: "web", Protocol: "tcp", Address: "10.0.0.2", Port: int64(8080),
					Annotations: map[string]string{"prometheus.io/scrape": "true"}},
				{Cluster: "dev", Service: "web", Protocol: "tcp", Address: "10.0.0.3", Port: int64(8080),
					Annotations: map[string]string{"prometheus.io/scrape": "false"}},
			}

			result, err := entries.PrometheusFileSD()
			Expect(err).ToNot(HaveOccurred())

			var parsed []map[string]any
			Expect(json.Unmarshal([]byte(result), &parsed)).To(Succeed())
			Expect(parsed).To(HaveLen(1))
			Expect(parsed[0]["targets"]).To(Equal([]any{"10.0.0.2:8080"}))
		})

		It("should skip entries without a port", func() {
			entries := RegistrationEntries{
				{Cluster: "dev", Service: "prometheus", Protocol: "tcp", Address: "10.0.0.1", Port: int64(8080)},
				{Cluster: "dev", Service: "prometheus", Protocol: "tcp", Address: "10.0.0.2", Port: nil},
			}

			result, err := entries.PrometheusFileSD()
			Expect(err).ToNot(HaveOccurred())

			var parsed []map[string]any
			Expect(json.Unmarshal([]byte(result), &parsed)).To(Succeed())
			Expect(parsed).To(HaveLen(1))
			Expect(parsed[0]["targets"]).To(Equal([]any{"10.0.0.1:8080"}))
		})

		It("should include annotations as labels for prometheus entries", func() {
			entries := RegistrationEntries{
				{Cluster: "dev", Service: "prometheus", Protocol: "tcp", Address: "10.0.0.1", Port: int64(8080),
					Annotations: map[string]string{"env": "staging", "team": "platform"}},
			}

			result, err := entries.PrometheusFileSD()
			Expect(err).ToNot(HaveOccurred())

			var parsed []map[string]any
			Expect(json.Unmarshal([]byte(result), &parsed)).To(Succeed())
			Expect(parsed[0]["labels"]).To(Equal(map[string]any{
				"cluster": "dev", "service": "prometheus", "protocol": "tcp",
				"env": "staging", "team": "platform",
			}))
		})

		It("should include annotations as labels for opted-in non-prometheus entries", func() {
			entries := RegistrationEntries{
				{Cluster: "dev", Service: "web", Protocol: "tcp", Address: "10.0.0.1", Port: int64(8080),
					Annotations: map[string]string{"env": "staging", "team": "platform", "prometheus.io/scrape": "true"}},
			}

			result, err := entries.PrometheusFileSD()
			Expect(err).ToNot(HaveOccurred())

			var parsed []map[string]any
			Expect(json.Unmarshal([]byte(result), &parsed)).To(Succeed())
			Expect(parsed[0]["labels"]).To(Equal(map[string]any{
				"cluster": "dev", "service": "web", "protocol": "tcp",
				"env": "staging", "team": "platform",
			}))
		})

		It("should skip annotations with empty values", func() {
			entries := RegistrationEntries{
				{Cluster: "dev", Service: "prometheus", Protocol: "tcp", Address: "10.0.0.1", Port: int64(8080),
					Annotations: map[string]string{"env": "staging", "empty": ""}},
			}

			result, err := entries.PrometheusFileSD()
			Expect(err).ToNot(HaveOccurred())

			var parsed []map[string]any
			Expect(json.Unmarshal([]byte(result), &parsed)).To(Succeed())
			Expect(parsed[0]["labels"]).To(Equal(map[string]any{
				"cluster": "dev", "service": "prometheus", "protocol": "tcp",
				"env": "staging",
			}))
		})

		It("should skip annotations with __ prefix", func() {
			entries := RegistrationEntries{
				{Cluster: "dev", Service: "prometheus", Protocol: "tcp", Address: "10.0.0.1", Port: int64(8080),
					Annotations: map[string]string{"env": "staging", "__internal": "hidden", "__meta_role": "worker"}},
			}

			result, err := entries.PrometheusFileSD()
			Expect(err).ToNot(HaveOccurred())

			var parsed []map[string]any
			Expect(json.Unmarshal([]byte(result), &parsed)).To(Succeed())
			Expect(parsed[0]["labels"]).To(Equal(map[string]any{
				"cluster": "dev", "service": "prometheus", "protocol": "tcp",
				"env": "staging",
			}))
		})

		It("should skip annotations with invalid label names", func() {
			entries := RegistrationEntries{
				{Cluster: "dev", Service: "web", Protocol: "tcp", Address: "10.0.0.1", Port: int64(8080),
					Annotations: map[string]string{
						"valid_label":          "ok",
						"_also_valid":          "ok",
						"prometheus.io/scrape": "true",
						"1starts_with_digit":   "bad",
						"has-hyphen":           "bad",
						"has space":            "bad",
						"has.dot":              "bad",
					}},
			}

			result, err := entries.PrometheusFileSD()
			Expect(err).ToNot(HaveOccurred())

			var parsed []map[string]any
			Expect(json.Unmarshal([]byte(result), &parsed)).To(Succeed())
			Expect(parsed[0]["labels"]).To(Equal(map[string]any{
				"cluster":     "dev",
				"service":     "web",
				"protocol":    "tcp",
				"valid_label": "ok",
				"_also_valid": "ok",
			}))
		})

		It("should group by protocol separately", func() {
			entries := RegistrationEntries{
				{Cluster: "dev", Service: "prometheus", Protocol: "tcp", Address: "10.0.0.1", Port: int64(8080)},
				{Cluster: "dev", Service: "prometheus", Protocol: "udp", Address: "10.0.0.1", Port: int64(8080)},
			}

			result, err := entries.PrometheusFileSD()
			Expect(err).ToNot(HaveOccurred())

			var parsed []map[string]any
			Expect(json.Unmarshal([]byte(result), &parsed)).To(Succeed())
			Expect(parsed).To(HaveLen(2))
		})

		It("should return an empty array for no entries", func() {
			entries := RegistrationEntries{}

			result, err := entries.PrometheusFileSD()
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("[]"))
		})

		It("should handle mixed prometheus and non-prometheus entries", func() {
			entries := RegistrationEntries{
				{Cluster: "dev", Service: "prometheus", Protocol: "tcp", Address: "10.0.0.1", Port: int64(9090)},
				{Cluster: "dev", Service: "web", Protocol: "tcp", Address: "10.0.0.2", Port: int64(8080),
					Annotations: map[string]string{"prometheus.io/scrape": "true"}},
				{Cluster: "dev", Service: "api", Protocol: "tcp", Address: "10.0.0.3", Port: int64(3000)},
			}

			result, err := entries.PrometheusFileSD()
			Expect(err).ToNot(HaveOccurred())

			var parsed []map[string]any
			Expect(json.Unmarshal([]byte(result), &parsed)).To(Succeed())
			Expect(parsed).To(HaveLen(2))

			Expect(parsed[0]["labels"]).To(Equal(map[string]any{
				"cluster": "dev", "service": "prometheus", "protocol": "tcp",
			}))
			Expect(parsed[0]["targets"]).To(Equal([]any{"10.0.0.1:9090"}))

			Expect(parsed[1]["labels"]).To(Equal(map[string]any{
				"cluster": "dev", "service": "web", "protocol": "tcp",
			}))
			Expect(parsed[1]["targets"]).To(Equal([]any{"10.0.0.2:8080"}))
		})
	})

	Describe("Validate", func() {
		var valid RegistrationEntry

		BeforeEach(func() {
			valid = RegistrationEntry{
				Cluster:  "production",
				Protocol: "tcp",
				Service:  "web",
				Address:  "192.168.1.1",
				Port:     int64(8080),
				Priority: 1,
			}
		})

		It("should accept a valid entry", func() {
			Expect(valid.Validate()).ToNot(HaveOccurred())
		})

		DescribeTable("required fields",
			func(mutate func(*RegistrationEntry), errorText string) {
				mutate(&valid)
				err := valid.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(ErrRegistrationInvalid.Error()))
				Expect(err.Error()).To(ContainSubstring(errorText))
			},

			Entry("empty cluster", func(e *RegistrationEntry) { e.Cluster = "" }, "cluster is required"),
			Entry("empty protocol", func(e *RegistrationEntry) { e.Protocol = "" }, "protocol is required"),
			Entry("empty service", func(e *RegistrationEntry) { e.Service = "" }, "service is required"),
			Entry("empty address", func(e *RegistrationEntry) { e.Address = "" }, "address is required"),
			Entry("zero port", func(e *RegistrationEntry) { e.Port = int64(0) }, "port"),
			Entry("zero priority", func(e *RegistrationEntry) { e.Priority = 0 }, "priority"),
		)

		DescribeTable("service name validation",
			func(service, errorText string) {
				valid.Service = service
				err := valid.Validate()

				if errorText != "" {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(errorText))
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},

			Entry("simple name", "web", ""),
			Entry("name with hyphens", "my-service", ""),
			Entry("name with underscores", "my_service", ""),
			Entry("name with digits", "web01", ""),
			Entry("uppercase", "MyService", ""),
			Entry("starts with digit", "1service", "not a valid name"),
			Entry("contains spaces", "my service", "not a valid name"),
			Entry("contains dots", "my.service", "not a valid name"),
			Entry("contains special chars", "my@service", "not a valid name"),
		)

		DescribeTable("Address address validation",
			func(ip, errorText string) {
				valid.Address = ip
				err := valid.Validate()

				if errorText != "" {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(errorText))
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},

			Entry("valid IPv4", "192.168.1.1", ""),
			Entry("valid IPv4 loopback", "127.0.0.1", ""),
			Entry("valid IPv4 all zeros", "0.0.0.0", ""),
			Entry("valid IPv6", "::1", ""),
			Entry("valid IPv6 full", "2001:db8::1", ""),
			Entry("invalid Address", "not-an-ip", "not a valid Address address"),
			Entry("invalid Address with port", "192.168.1.1:8080", "not a valid Address address"),
			Entry("hostname", "example.com", "not a valid Address address"),
		)

		DescribeTable("port validation",
			func(port int64, errorText string) {
				valid.Port = port
				err := valid.Validate()

				if errorText != "" {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(errorText))
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},

			Entry("port 1", int64(1), ""),
			Entry("port 80", int64(80), ""),
			Entry("port 443", int64(443), ""),
			Entry("port 65535", int64(65535), ""),
			Entry("port 0", int64(0), "port"),
			Entry("port above max", int64(65536), "port"),
		)

		It("should not require annotations", func() {
			valid.Annotations = nil
			Expect(valid.Validate()).ToNot(HaveOccurred())
		})

		It("should accept annotations", func() {
			valid.Annotations = map[string]string{"env": "prod"}
			Expect(valid.Validate()).ToNot(HaveOccurred())
		})

		It("should accept cluster", func() {
			valid.Cluster = "staging"
			Expect(valid.Validate()).ToNot(HaveOccurred())
		})
	})
})
