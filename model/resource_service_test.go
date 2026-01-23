// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceResourceProperties", func() {
	Describe("Validate", func() {
		DescribeTable("validation tests",
			func(serviceName, ensure, errorText string) {
				prop := &ServiceResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   serviceName,
						Ensure: ensure,
					},
				}

				err := prop.Validate()

				if errorText != "" {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(errorText))
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},

			Entry("valid service name", "nginx", "running", ""),
			Entry("valid service name with dots", "foo.service", "running", ""),
			Entry("valid service name with hyphens", "foo-bar", "running", ""),
			Entry("valid service name with underscores", "foo_bar", "running", ""),
			Entry("valid service name with plus", "service++", "running", ""),
			Entry("valid service name with colon", "service:name", "running", ""),
			Entry("valid service name with tilde", "service~test", "running", ""),
			Entry("valid ensure running", "nginx", "running", ""),
			Entry("valid ensure stopped", "nginx", "stopped", ""),
			Entry("empty ensure defaults to running", "nginx", "", ""),
			Entry("empty service name", "", "running", "name"),
			Entry("invalid ensure value", "nginx", "present", "invalid ensure value"),
			Entry("invalid ensure value absent", "nginx", "absent", "invalid ensure value"),
			Entry("invalid ensure value latest", "nginx", "latest", "invalid ensure value"),
			Entry("service name with semicolon (command separator)", "nginx; rm -rf /", "running", "dangerous characters"),
			Entry("service name with pipe (command pipe)", "nginx | cat", "running", "dangerous characters"),
			Entry("service name with ampersand (background)", "nginx & whoami", "running", "dangerous characters"),
			Entry("service name with dollar (variable expansion)", "nginx$PATH", "running", "dangerous characters"),
			Entry("service name with backtick (command substitution)", "nginx`whoami`", "running", "dangerous characters"),
			Entry("service name with single quote", "nginx'test", "running", "dangerous characters"),
			Entry("service name with double quote", "nginx\"test", "running", "dangerous characters"),
			Entry("service name with parentheses (subshell)", "nginx(whoami)", "running", "dangerous characters"),
			Entry("service name with brackets", "nginx[test]", "running", "dangerous characters"),
			Entry("service name with asterisk (wildcard)", "nginx*", "running", "dangerous characters"),
			Entry("service name with question mark (wildcard)", "nginx?", "running", "dangerous characters"),
			Entry("service name with redirect", "nginx > /tmp/file", "running", "dangerous characters"),
			Entry("service name with backslash", "nginx\\test", "running", "dangerous characters"),
			Entry("service name with newline", "nginx\nwhoami", "running", "dangerous characters"),
			Entry("service name with tab", "nginx\twhoami", "running", "dangerous characters"),
			Entry("service name with space", "nginx test", "running", "invalid characters"),
			Entry("service name with leading space", " nginx", "running", "invalid characters"),
			Entry("service name with trailing space", "nginx ", "running", "invalid characters"),
			Entry("service name with invalid characters", "nginx@test", "running", "invalid characters"),
			Entry("ensure with semicolon", "nginx", "running; rm -rf /", "invalid ensure value"),
			Entry("ensure with command substitution", "nginx", "running$(whoami)", "invalid ensure value"),
		)

		DescribeTable("legitimate services",
			func(name, ensure string) {
				prop := &ServiceResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   name,
						Ensure: ensure,
					},
				}

				err := prop.Validate()
				Expect(err).ToNot(HaveOccurred())
			},

			Entry("nginx running", "nginx", "running"),
			Entry("nginx stopped", "nginx", "stopped"),
			Entry("nginx empty ensure", "nginx", ""),
			Entry("httpd running", "httpd", "running"),
			Entry("sshd running", "sshd", "running"),
			Entry("systemd-resolved running", "systemd-resolved", "running"),
			Entry("NetworkManager running", "NetworkManager", "running"),
			Entry("docker.service running", "docker.service", "running"),
			Entry("foo_bar running", "foo_bar", "running"),
			Entry("service.name running", "service.name", "running"),
			Entry("service-name running", "service-name", "running"),
			Entry("service+extra running", "service+extra", "running"),
			Entry("service~test running", "service~test", "running"),
			Entry("service:type running", "service:type", "running"),
			Entry("crond stopped", "crond", "stopped"),
			Entry("firewalld stopped", "firewalld", "stopped"),
		)

		DescribeTable("ensure value handling",
			func(inputEnsure, expectedEnsure string) {
				prop := &ServiceResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   "nginx",
						Ensure: inputEnsure,
					},
				}

				err := prop.Validate()
				Expect(err).ToNot(HaveOccurred())
				Expect(prop.Ensure).To(Equal(expectedEnsure))
			},
			Entry("empty ensure defaults to running", "", ServiceEnsureRunning),
			Entry("explicit running is preserved", ServiceEnsureRunning, ServiceEnsureRunning),
			Entry("explicit stopped is preserved", ServiceEnsureStopped, ServiceEnsureStopped),
		)

		DescribeTable("subscribe validation",
			func(subscribe []string, errorText string) {
				prop := &ServiceResourceProperties{
					CommonResourceProperties: CommonResourceProperties{
						Name:   "nginx",
						Ensure: ServiceEnsureRunning,
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

			Entry("valid subscribe format", []string{"file#/etc/nginx/nginx.conf"}, ""),
			Entry("valid multiple subscribes", []string{"file#/etc/nginx/nginx.conf", "file#/etc/nginx/conf.d/default.conf"}, ""),
			Entry("empty subscribe array", []string{}, ""),
			Entry("invalid subscribe without hash", []string{"file:/etc/nginx/nginx.conf"}, "invalid subscribe format"),
			Entry("invalid subscribe empty string", []string{""}, "invalid subscribe format"),
			Entry("mixed valid and invalid", []string{"file#/etc/nginx/nginx.conf", "invalid"}, "invalid subscribe format"),
		)
	})
})
