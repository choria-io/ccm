// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
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

var _ = Describe("CommonHealthCheck", func() {
	Describe("UnmarshalJSON", func() {
		DescribeTable("parses timeout durations",
			func(jsonInput string, expectedTimeout time.Duration, expectError bool) {
				var hc CommonHealthCheck
				err := json.Unmarshal([]byte(jsonInput), &hc)

				if expectError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
					Expect(hc.ParsedTimeout).To(Equal(expectedTimeout))
				}
			},

			Entry("standard go duration seconds", `{"timeout": "30s"}`, 30*time.Second, false),
			Entry("standard go duration minutes", `{"timeout": "5m"}`, 5*time.Minute, false),
			Entry("standard go duration hours", `{"timeout": "2h"}`, 2*time.Hour, false),
			Entry("standard go duration milliseconds", `{"timeout": "500ms"}`, 500*time.Millisecond, false),
			Entry("fisk extended duration days", `{"timeout": "1d"}`, 24*time.Hour, false),
			Entry("fisk extended duration weeks", `{"timeout": "1w"}`, 7*24*time.Hour, false),
			Entry("fisk extended duration months", `{"timeout": "1M"}`, 30*24*time.Hour, false),
			Entry("fisk extended duration years", `{"timeout": "1y"}`, 365*24*time.Hour, false),
			Entry("combined duration", `{"timeout": "1h30m"}`, time.Hour+30*time.Minute, false),
			Entry("empty timeout", `{"timeout": ""}`, time.Duration(0), false),
			Entry("no timeout field", `{"command": "/bin/check"}`, time.Duration(0), false),
			Entry("invalid duration", `{"timeout": "invalid"}`, time.Duration(0), true),
			Entry("negative duration", `{"timeout": "-1s"}`, -1*time.Second, false),
		)

		It("should preserve other fields", func() {
			jsonInput := `{"command": "/bin/check", "timeout": "30s", "format": "nagios"}`
			var hc CommonHealthCheck
			err := json.Unmarshal([]byte(jsonInput), &hc)

			Expect(err).ToNot(HaveOccurred())
			Expect(hc.Command).To(Equal("/bin/check"))
			Expect(hc.Timeout).To(Equal("30s"))
			Expect(hc.Format).To(Equal(HealthCheckNagiosFormat))
			Expect(hc.ParsedTimeout).To(Equal(30 * time.Second))
		})
	})

	Describe("UnmarshalYAML", func() {
		DescribeTable("parses timeout durations",
			func(yamlInput string, expectedTimeout time.Duration, expectError bool) {
				var hc CommonHealthCheck
				err := yaml.Unmarshal([]byte(yamlInput), &hc)

				if expectError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
					Expect(hc.ParsedTimeout).To(Equal(expectedTimeout))
				}
			},

			Entry("standard go duration seconds", "timeout: 30s", 30*time.Second, false),
			Entry("standard go duration minutes", "timeout: 5m", 5*time.Minute, false),
			Entry("standard go duration hours", "timeout: 2h", 2*time.Hour, false),
			Entry("standard go duration milliseconds", "timeout: 500ms", 500*time.Millisecond, false),
			Entry("fisk extended duration days", "timeout: 1d", 24*time.Hour, false),
			Entry("fisk extended duration weeks", "timeout: 1w", 7*24*time.Hour, false),
			Entry("fisk extended duration months", "timeout: 1M", 30*24*time.Hour, false),
			Entry("fisk extended duration years", "timeout: 1y", 365*24*time.Hour, false),
			Entry("combined duration", "timeout: 1h30m", time.Hour+30*time.Minute, false),
			Entry("empty timeout", "timeout: \"\"", time.Duration(0), false),
			Entry("no timeout field", "command: /bin/check", time.Duration(0), false),
			Entry("invalid duration", "timeout: invalid", time.Duration(0), true),
			Entry("negative duration", "timeout: -1s", -1*time.Second, false),
		)

		It("should preserve other fields", func() {
			yamlInput := `command: /bin/check
timeout: 30s
format: nagios`
			var hc CommonHealthCheck
			err := yaml.Unmarshal([]byte(yamlInput), &hc)

			Expect(err).ToNot(HaveOccurred())
			Expect(hc.Command).To(Equal("/bin/check"))
			Expect(hc.Timeout).To(Equal("30s"))
			Expect(hc.Format).To(Equal(HealthCheckNagiosFormat))
			Expect(hc.ParsedTimeout).To(Equal(30 * time.Second))
		})
	})

	Describe("Embedded in CommonResourceProperties", func() {
		It("should parse health_checks timeout from JSON", func() {
			jsonInput := `{
				"name": "test",
				"ensure": "present",
				"health_checks": [{
					"command": "/bin/check",
					"timeout": "1m30s"
				}]
			}`
			var props CommonResourceProperties
			err := json.Unmarshal([]byte(jsonInput), &props)

			Expect(err).ToNot(HaveOccurred())
			Expect(props.HealthChecks).To(HaveLen(1))
			Expect(props.HealthChecks[0].Command).To(Equal("/bin/check"))
			Expect(props.HealthChecks[0].Timeout).To(Equal("1m30s"))
			Expect(props.HealthChecks[0].ParsedTimeout).To(Equal(90 * time.Second))
		})

		It("should parse health_checks timeout from YAML", func() {
			yamlInput := `name: test
ensure: present
health_checks:
  - command: /bin/check
    timeout: 1m30s`
			var props CommonResourceProperties
			err := yaml.Unmarshal([]byte(yamlInput), &props)

			Expect(err).ToNot(HaveOccurred())
			Expect(props.HealthChecks).To(HaveLen(1))
			Expect(props.HealthChecks[0].Command).To(Equal("/bin/check"))
			Expect(props.HealthChecks[0].Timeout).To(Equal("1m30s"))
			Expect(props.HealthChecks[0].ParsedTimeout).To(Equal(90 * time.Second))
		})

		It("should handle empty health_checks in JSON", func() {
			jsonInput := `{"name": "test", "ensure": "present"}`
			var props CommonResourceProperties
			err := json.Unmarshal([]byte(jsonInput), &props)

			Expect(err).ToNot(HaveOccurred())
			Expect(props.HealthChecks).To(BeEmpty())
		})

		It("should handle empty health_checks in YAML", func() {
			yamlInput := `name: test
ensure: present`
			var props CommonResourceProperties
			err := yaml.Unmarshal([]byte(yamlInput), &props)

			Expect(err).ToNot(HaveOccurred())
			Expect(props.HealthChecks).To(BeEmpty())
		})
	})
})
