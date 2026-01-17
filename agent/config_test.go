// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config")
}

var _ = Describe("Config", func() {
	Describe("ParseConfig", func() {
		It("Should parse valid config with defaults", func() {
			yamlData := `{}`

			cfg, err := ParseConfig([]byte(yamlData))
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg).NotTo(BeNil())
			Expect(cfg.intervalDuration).To(Equal(DefaultInterval))
			Expect(cfg.CacheDir).To(Equal(DefaultCacheDir))
			Expect(cfg.Manifests).To(BeEmpty())
			Expect(cfg.LogLevel).To(Equal("info"))
			Expect(cfg.NatsContext).To(Equal("CCM"))
		})

		It("Should parse interval duration", func() {
			yamlData := `interval: 10m`

			cfg, err := ParseConfig([]byte(yamlData))
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.intervalDuration).To(Equal(10 * time.Minute))
		})

		It("Should parse health check interval duration", func() {
			yamlData := `health_check_interval: 2m`

			cfg, err := ParseConfig([]byte(yamlData))
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.healthCheckIntervalDuration).To(Equal(2 * time.Minute))
		})

		It("Should parse all fields", func() {
			yamlData := `
interval: 5m
health_check_interval: 1m
manifests:
  - /path/to/manifest1.yaml
  - /path/to/manifest2.yaml
external_data_url: http://example.com/data
cache_dir: /custom/cache
monitor_port: 8080
log_level: debug
nats_context: custom-context
`

			cfg, err := ParseConfig([]byte(yamlData))
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.intervalDuration).To(Equal(5 * time.Minute))
			Expect(cfg.healthCheckIntervalDuration).To(Equal(1 * time.Minute))
			Expect(cfg.Manifests).To(Equal([]string{"/path/to/manifest1.yaml", "/path/to/manifest2.yaml"}))
			Expect(cfg.ExternalDataUrl).To(Equal("http://example.com/data"))
			Expect(cfg.CacheDir).To(Equal("/custom/cache"))
			Expect(cfg.MonitorPort).To(Equal(8080))
			Expect(cfg.LogLevel).To(Equal("debug"))
			Expect(cfg.NatsContext).To(Equal("custom-context"))
		})

		It("Should return error for invalid YAML", func() {
			yamlData := `invalid: yaml: data:`

			cfg, err := ParseConfig([]byte(yamlData))
			Expect(err).To(HaveOccurred())
			Expect(cfg).To(BeNil())
		})

		It("Should return error for invalid interval duration", func() {
			yamlData := `interval: not-a-duration`

			cfg, err := ParseConfig([]byte(yamlData))
			Expect(err).To(HaveOccurred())
			Expect(cfg).To(BeNil())
		})

		It("Should return error for invalid health check interval duration", func() {
			yamlData := `health_check_interval: invalid`

			cfg, err := ParseConfig([]byte(yamlData))
			Expect(err).To(HaveOccurred())
			Expect(cfg).To(BeNil())
		})

		It("Should return error when interval is below minimum", func() {
			yamlData := `interval: 10s`

			cfg, err := ParseConfig([]byte(yamlData))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("interval must be at least"))
			Expect(cfg).To(BeNil())
		})

		It("Should return error when cache_dir is empty", func() {
			yamlData := `cache_dir: ""`

			cfg, err := ParseConfig([]byte(yamlData))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cache_dir must be set"))
			Expect(cfg).To(BeNil())
		})

		It("Should return error for invalid log level", func() {
			yamlData := `log_level: invalid`

			cfg, err := ParseConfig([]byte(yamlData))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("log_level must be one of"))
			Expect(cfg).To(BeNil())
		})
	})

	Describe("Validate", func() {
		It("Should pass with valid config", func() {
			cfg := &Config{
				intervalDuration: 5 * time.Minute,
				CacheDir:         "/some/path",
				LogLevel:         "info",
				NatsContext:      "CCM",
			}

			err := cfg.Validate()
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should fail when interval is zero", func() {
			cfg := &Config{
				intervalDuration: 0,
				CacheDir:         "/some/path",
				LogLevel:         "info",
				NatsContext:      "CCM",
			}

			err := cfg.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("interval must be set"))
		})

		It("Should fail when interval is negative", func() {
			cfg := &Config{
				intervalDuration: -1 * time.Minute,
				CacheDir:         "/some/path",
				LogLevel:         "info",
				NatsContext:      "CCM",
			}

			err := cfg.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("interval must be set"))
		})

		It("Should fail when interval is below minimum", func() {
			cfg := &Config{
				intervalDuration: 10 * time.Second,
				CacheDir:         "/some/path",
				LogLevel:         "info",
				NatsContext:      "CCM",
			}

			err := cfg.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("interval must be at least"))
		})

		It("Should pass when interval equals minimum", func() {
			cfg := &Config{
				intervalDuration: MinInterval,
				CacheDir:         "/some/path",
				LogLevel:         "info",
				NatsContext:      "CCM",
			}

			err := cfg.Validate()
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should fail when cache_dir is empty", func() {
			cfg := &Config{
				intervalDuration: 5 * time.Minute,
				CacheDir:         "",
				LogLevel:         "info",
				NatsContext:      "CCM",
			}

			err := cfg.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cache_dir must be set"))
		})

		It("Should accept valid log levels", func() {
			for _, level := range []string{"debug", "info", "warn", "error"} {
				cfg := &Config{
					intervalDuration: 5 * time.Minute,
					CacheDir:         "/some/path",
					LogLevel:         level,
					NatsContext:      "CCM",
				}

				err := cfg.Validate()
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("Should fail with invalid log level", func() {
			cfg := &Config{
				intervalDuration: 5 * time.Minute,
				CacheDir:         "/some/path",
				LogLevel:         "invalid",
				NatsContext:      "CCM",
			}

			err := cfg.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("log_level must be one of"))
		})

		It("Should fail with empty log level", func() {
			cfg := &Config{
				intervalDuration: 5 * time.Minute,
				CacheDir:         "/some/path",
				LogLevel:         "",
				NatsContext:      "CCM",
			}

			err := cfg.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("log_level must be one of"))
		})

		It("Should require nats_servers when using choria token auth", func() {
			tokenFile, err := os.CreateTemp("", "ccm-token-*")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tokenFile.Name())

			seedFile, err := os.CreateTemp("", "ccm-seed-*")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(seedFile.Name())

			cfg := &Config{
				intervalDuration: 5 * time.Minute,
				CacheDir:         "/some/path",
				LogLevel:         "info",
				ChoriaTokenFile:  tokenFile.Name(),
				ChoriaSeedFile:   seedFile.Name(),
			}

			err = cfg.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("nats_servers must be set"))
		})
	})

	Describe("NewLogger", func() {
		It("Should create a logger for each valid log level", func() {
			for _, level := range []string{"debug", "info", "warn", "error"} {
				cfg := &Config{
					LogLevel: level,
				}

				logger, err := cfg.NewLogger()
				Expect(err).NotTo(HaveOccurred())
				Expect(logger).NotTo(BeNil())
			}
		})

		It("Should default to warn level for unknown log level", func() {
			cfg := &Config{
				LogLevel: "unknown",
			}

			logger, err := cfg.NewLogger()
			Expect(err).NotTo(HaveOccurred())
			Expect(logger).NotTo(BeNil())
		})
	})
})
