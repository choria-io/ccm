// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
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

			Entry("standard go duration seconds", `{"command": "/bin/check", "timeout": "30s"}`, 30*time.Second, false),
			Entry("standard go duration minutes", `{"command": "/bin/check", "timeout": "5m"}`, 5*time.Minute, false),
			Entry("standard go duration hours", `{"command": "/bin/check", "timeout": "2h"}`, 2*time.Hour, false),
			Entry("standard go duration milliseconds", `{"command": "/bin/check", "timeout": "500ms"}`, 500*time.Millisecond, false),
			Entry("fisk extended duration days", `{"command": "/bin/check", "timeout": "1d"}`, 24*time.Hour, false),
			Entry("fisk extended duration weeks", `{"command": "/bin/check", "timeout": "1w"}`, 7*24*time.Hour, false),
			Entry("fisk extended duration months", `{"command": "/bin/check", "timeout": "1M"}`, 30*24*time.Hour, false),
			Entry("fisk extended duration years", `{"command": "/bin/check", "timeout": "1y"}`, 365*24*time.Hour, false),
			Entry("combined duration", `{"command": "/bin/check", "timeout": "1h30m"}`, time.Hour+30*time.Minute, false),
			Entry("empty timeout", `{"command": "/bin/check", "timeout": ""}`, time.Duration(0), false),
			Entry("no timeout field", `{"command": "/bin/check"}`, time.Duration(0), false),
			Entry("invalid duration", `{"command": "/bin/check", "timeout": "invalid"}`, time.Duration(0), true),
			Entry("negative duration", `{"command": "/bin/check", "timeout": "-1s"}`, -1*time.Second, false),
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

			Entry("standard go duration seconds", "command: /bin/check\ntimeout: 30s", 30*time.Second, false),
			Entry("standard go duration minutes", "command: /bin/check\ntimeout: 5m", 5*time.Minute, false),
			Entry("standard go duration hours", "command: /bin/check\ntimeout: 2h", 2*time.Hour, false),
			Entry("standard go duration milliseconds", "command: /bin/check\ntimeout: 500ms", 500*time.Millisecond, false),
			Entry("fisk extended duration days", "command: /bin/check\ntimeout: 1d", 24*time.Hour, false),
			Entry("fisk extended duration weeks", "command: /bin/check\ntimeout: 1w", 7*24*time.Hour, false),
			Entry("fisk extended duration months", "command: /bin/check\ntimeout: 1M", 30*24*time.Hour, false),
			Entry("fisk extended duration years", "command: /bin/check\ntimeout: 1y", 365*24*time.Hour, false),
			Entry("combined duration", "command: /bin/check\ntimeout: 1h30m", time.Hour+30*time.Minute, false),
			Entry("empty timeout", "command: /bin/check\ntimeout: \"\"", time.Duration(0), false),
			Entry("no timeout field", "command: /bin/check", time.Duration(0), false),
			Entry("invalid duration", "command: /bin/check\ntimeout: invalid", time.Duration(0), true),
			Entry("negative duration", "command: /bin/check\ntimeout: -1s", -1*time.Second, false),
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

	Describe("Automatic Name from Command", func() {
		DescribeTable("sets name from command basename via JSON",
			func(jsonInput string, expectedName string) {
				var hc CommonHealthCheck
				err := json.Unmarshal([]byte(jsonInput), &hc)
				Expect(err).ToNot(HaveOccurred())
				Expect(hc.Name).To(Equal(expectedName))
			},
			Entry("full path command", `{"command": "/usr/lib/nagios/plugins/check_disk"}`, "check_disk"),
			Entry("command with arguments", `{"command": "/usr/bin/check_http -H localhost"}`, "check_http -H localhost"),
			Entry("simple command", `{"command": "check_memory"}`, "check_memory"),
			Entry("relative path", `{"command": "./scripts/check_app.sh"}`, "check_app.sh"),
		)

		DescribeTable("sets name from command basename via YAML",
			func(yamlInput string, expectedName string) {
				var hc CommonHealthCheck
				err := yaml.Unmarshal([]byte(yamlInput), &hc)
				Expect(err).ToNot(HaveOccurred())
				Expect(hc.Name).To(Equal(expectedName))
			},
			Entry("full path command", "command: /usr/lib/nagios/plugins/check_disk", "check_disk"),
			Entry("command with arguments", "command: /usr/bin/check_http -H localhost", "check_http -H localhost"),
			Entry("simple command", "command: check_memory", "check_memory"),
			Entry("relative path", "command: ./scripts/check_app.sh", "check_app.sh"),
		)

		It("should preserve explicit name when provided via JSON", func() {
			jsonInput := `{"command": "/usr/lib/nagios/plugins/check_disk", "name": "disk_space"}`
			var hc CommonHealthCheck
			err := json.Unmarshal([]byte(jsonInput), &hc)
			Expect(err).ToNot(HaveOccurred())
			Expect(hc.Name).To(Equal("disk_space"))
		})

		It("should preserve explicit name when provided via YAML", func() {
			yamlInput := `command: /usr/lib/nagios/plugins/check_disk
name: disk_space`
			var hc CommonHealthCheck
			err := yaml.Unmarshal([]byte(yamlInput), &hc)
			Expect(err).ToNot(HaveOccurred())
			Expect(hc.Name).To(Equal("disk_space"))
		})
	})

	Describe("Format Auto-Detection", func() {
		It("should default to nagios format when command is set", func() {
			var hc CommonHealthCheck
			err := json.Unmarshal([]byte(`{"command": "/bin/check"}`), &hc)
			Expect(err).ToNot(HaveOccurred())
			Expect(hc.Format).To(Equal(HealthCheckNagiosFormat))
		})

		It("should default to goss format when goss_check is set", func() {
			var hc CommonHealthCheck
			err := json.Unmarshal([]byte(`{"goss_rules": "/etc/goss/check.yaml"}`), &hc)
			Expect(err).ToNot(HaveOccurred())
			Expect(hc.Format).To(Equal(HealthCheckGossFormat))
		})

		It("should error when neither command nor goss_check is set and no format", func() {
			var hc CommonHealthCheck
			err := json.Unmarshal([]byte(`{"timeout": "30s"}`), &hc)
			Expect(err).To(MatchError(ContainSubstring("'format' flag is required")))
		})
	})

	Describe("GossRules", func() {
		It("should error when both command and goss_check are set via JSON", func() {
			var hc CommonHealthCheck
			err := json.Unmarshal([]byte(`{"command": "/bin/check", "goss_rules": "/etc/goss/check.yaml"}`), &hc)
			Expect(err).To(MatchError(ContainSubstring("mutually exclusive")))
		})

		It("should error when both command and goss_check are set via YAML", func() {
			yamlInput := "command: /bin/check\ngoss_rules: /etc/goss/check.yaml"
			var hc CommonHealthCheck
			err := yaml.Unmarshal([]byte(yamlInput), &hc)
			Expect(err).To(MatchError(ContainSubstring("mutually exclusive")))
		})

		It("should parse goss_check field via YAML", func() {
			var hc CommonHealthCheck
			err := yaml.Unmarshal([]byte("goss_rules: /etc/goss/check.yaml"), &hc)
			Expect(err).ToNot(HaveOccurred())
			Expect(hc.GossRules).To(Equal(yaml.RawMessage("/etc/goss/check.yaml")))
			Expect(hc.Command).To(BeEmpty())
			Expect(hc.Format).To(Equal(HealthCheckGossFormat))
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
