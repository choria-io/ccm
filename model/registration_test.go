// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/choria-io/ccm/templates"
)

var _ = Describe("RegistrationEntry", func() {
	Describe("NewRegistrationEntry", func() {
		It("should create an entry with all fields set", func() {
			entry, err := NewRegistrationEntry("production", "web", "tcp", "192.168.1.1", 8080, 1, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
			Expect(entry.Cluster).To(Equal("production"))
			Expect(entry.Service).To(Equal("web"))
			Expect(entry.Protocol).To(Equal("tcp"))
			Expect(entry.IP).To(Equal("192.168.1.1"))
			Expect(entry.Port).To(Equal(int64(8080)))
			Expect(entry.Priority).To(Equal(int64(1)))
			Expect(entry.TTL).To(Equal(30 * time.Second))
		})

		It("should not set annotations", func() {
			entry, err := NewRegistrationEntry("production", "web", "tcp", "192.168.1.1", 8080, 1, time.Minute)
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

		It("should resolve IP templates", func() {
			entry := &RegistrationEntry{
				Cluster:  "production",
				Service:  "web",
				Protocol: "tcp",
				IP:       "{{ Facts.ip }}",
				Port:     int64(8080),
				Priority: 1,
			}

			err := entry.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(entry.IP).To(Equal("10.0.0.1"))
		})

		It("should resolve cluster templates", func() {
			entry := &RegistrationEntry{
				Cluster:  "{{ Facts.cluster }}",
				Service:  "web",
				Protocol: "tcp",
				IP:       "10.0.0.1",
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
				IP:       "10.0.0.1",
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
				IP:       "10.0.0.1",
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
				IP:       "10.0.0.1",
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
				IP:       "10.0.0.1",
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
				IP:       "10.0.0.1",
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
				IP:       "10.0.0.1",
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
				IP:       "{{ Facts.ip }}",
				Port:     "{{ Facts.port }}",
				Priority: 1,
				Annotations: map[string]string{
					"host": "{{ Facts.hostname }}",
				},
			}

			err := entry.ResolveTemplates(env)
			Expect(err).ToNot(HaveOccurred())
			Expect(entry.Cluster).To(Equal("production"))
			Expect(entry.IP).To(Equal("10.0.0.1"))
			Expect(entry.Port).To(Equal(int64(9090)))
			Expect(entry.Annotations["host"]).To(Equal("web01"))
		})
	})

	Describe("Validate", func() {
		var valid RegistrationEntry

		BeforeEach(func() {
			valid = RegistrationEntry{
				Cluster:  "production",
				Protocol: "tcp",
				Service:  "web",
				IP:       "192.168.1.1",
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
			Entry("empty address", func(e *RegistrationEntry) { e.IP = "" }, "address is required"),
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

		DescribeTable("IP address validation",
			func(ip, errorText string) {
				valid.IP = ip
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
			Entry("invalid IP", "not-an-ip", "not a valid IP address"),
			Entry("invalid IP with port", "192.168.1.1:8080", "not a valid IP address"),
			Entry("hostname", "example.com", "not a valid IP address"),
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
