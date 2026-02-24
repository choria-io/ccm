// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/SladkyCitron/slogcolor"
	"github.com/goccy/go-yaml"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/manager"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/fisk"
)

// Config holds the agent configuration
type Config struct {
	// Interval is the time between scheduled manifest apply runs (e.g. "5m", "1h").
	// Must be at least MinInterval. Defaults to DefaultInterval.
	Interval         string `yaml:"interval"`
	intervalDuration time.Duration

	// HealthCheckInterval is the time between health check runs (e.g. "1m", "30s").
	// When set, health checks run independently of apply runs and can trigger
	// remediation applies when critical issues are detected.
	HealthCheckInterval         string `yaml:"health_check_interval"`
	healthCheckIntervalDuration time.Duration

	// Manifests is the list of manifest sources to apply. Each source creates a
	// separate worker that manages its own apply cycle. Sources can be file paths
	// or object store URLs (obj://bucket/key).
	Manifests []string `yaml:"manifests"`

	// ExternalDataUrl is an optional URL to fetch external data from using hiera
	// resolution. The resolved data is merged into the manifest data context.
	ExternalDataUrl string `yaml:"external_data_url"`

	// CacheDir is the directory used to cache manifest sources fetched from
	// remote locations. Defaults to DefaultCacheDir.
	CacheDir string `yaml:"cache_dir"`

	// MonitorPort is the port to listen on for accessing Prometheus stats
	MonitorPort int `yaml:"monitor_port"`

	// LogLevel is the log level to use
	// Valid values: debug, info, warn, error
	LogLevel string `yaml:"log_level"`

	// NatsContext is the NATS context to use for remote KV and Object store access
	NatsContext string `yaml:"nats_context"`
}

func ParseConfig(c []byte) (*Config, error) {
	cfg := &Config{
		Manifests:        []string{},
		intervalDuration: DefaultInterval,
		CacheDir:         DefaultCacheDir,
		LogLevel:         "info",
		NatsContext:      "CCM",
	}

	err := yaml.Unmarshal(c, cfg)
	if err != nil {
		return nil, err
	}

	if cfg.Interval != "" {
		cfg.intervalDuration, err = fisk.ParseDuration(cfg.Interval)
		if err != nil {
			return nil, err
		}
	}

	if cfg.HealthCheckInterval != "" {
		cfg.healthCheckIntervalDuration, err = fisk.ParseDuration(cfg.HealthCheckInterval)
		if err != nil {
			return nil, err
		}
	}

	err = cfg.Validate()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.intervalDuration <= 0 {
		return fmt.Errorf("interval must be set")
	}

	if c.intervalDuration < MinInterval {
		return fmt.Errorf("interval must be at least %v", MinInterval)
	}

	if c.CacheDir == "" {
		return fmt.Errorf("cache_dir must be set")
	}

	switch c.LogLevel {
	case "debug", "info", "warn", "error":
		// valid
	default:
		return fmt.Errorf("log_level must be one of: debug, info, warn, error")
	}

	return nil
}

func (c *Config) NewLogger() (model.Logger, error) {
	var level slog.Level

	switch c.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelWarn
	}

	if iu.IsTerminal() {
		return manager.NewSlogLogger(
			slog.New(
				slogcolor.NewHandler(os.Stdout, &slogcolor.Options{
					Level: level,
				}))), nil
	} else {
		return manager.NewSlogLogger(
			slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			}))), nil
	}
}
