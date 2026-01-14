// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"fmt"
	"time"
)

type Option func(a *Agent) error

func WithManifests(manifests ...string) Option {
	return func(a *Agent) error {
		a.cfg.Manifests = append(a.cfg.Manifests, manifests...)
		return nil
	}
}

func WithInterval(i time.Duration) Option {
	return func(a *Agent) error {
		if i < MinInterval {
			return fmt.Errorf("agent interval must be at least %v", MinInterval)
		}

		a.cfg.intervalDuration = i
		return nil
	}
}

func WithCacheDirectory(dir string) Option {
	return func(a *Agent) error {
		a.cfg.CacheDir = dir
		return nil
	}
}

func WithExternalDataUrl(path string) Option {
	return func(a *Agent) error {
		a.cfg.ExternalDataUrl = path
		return nil
	}
}

func WithHealthCheckOnlyInterval(i time.Duration) Option {
	return func(a *Agent) error {
		a.cfg.healthCheckIntervalDuration = i
		return nil
	}
}
