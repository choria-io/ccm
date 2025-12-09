// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/choria-io/ccm/internal/cmdrunner"
	"github.com/choria-io/ccm/internal/facts"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/apply"
	packageresource "github.com/choria-io/ccm/resources/package"
	"github.com/choria-io/ccm/session"
	"github.com/choria-io/ccm/templates"
	"github.com/choria-io/tinyhiera"
	"github.com/goccy/go-yaml"
)

// CCM is the main configuration and change management orchestrator
type CCM struct {
	session    model.SessionStore
	log        model.Logger
	userLogger model.Logger
	data       map[string]any

	mu sync.Mutex
}

// NewManager creates a new CCM instance with the provided loggers
func NewManager(log model.Logger, userLogger model.Logger, opts ...Option) (*CCM, error) {
	mgr := &CCM{log: log, userLogger: userLogger}

	for _, opt := range opts {
		err := opt(mgr)
		if err != nil {
			return nil, err
		}
	}

	if mgr.session == nil {
		sessionLog, err := mgr.Logger("session", "memory")
		if err != nil {
			return nil, err
		}

		mgr.session, err = session.NewMemorySessionStore(sessionLog, userLogger)
		if err != nil {
			return nil, err
		}
	}

	return mgr, nil
}

// SetData sets the resolved Hiera data for the manager
func (m *CCM) SetData(data map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = data
}

// Data returns the resolved Hiera data
func (m *CCM) Data() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data
}

func (m *CCM) applyPackageResource(ctx context.Context, properties *model.PackageResourceProperties) (model.TransactionEvent, error) {
	pkg, err := packageresource.New(ctx, m, *properties)
	if err != nil {
		return model.TransactionEvent{}, err
	}

	return pkg.Apply(ctx)
}

// ResolveManifestReader reads and resolves a manifest using Hiera, returning the resolved data and parsed manifest
func (m *CCM) ResolveManifestReader(ctx context.Context, manifest io.ReadCloser) (map[string]any, model.Apply, error) {
	mb, err := io.ReadAll(manifest)
	manifest.Close()
	if err != nil {
		return nil, nil, err
	}

	facts, err := m.Facts(ctx)
	if err != nil {
		return nil, nil, err
	}

	hieraLogger, err := m.Logger("hiera", "resources")
	if err != nil {
		return nil, nil, err
	}

	resolved, err := tinyhiera.ResolveYaml(mb, facts, tinyhiera.DefaultOptions, hieraLogger)
	if err != nil {
		return nil, nil, err
	}
	m.SetData(resolved)

	var manifestRaw map[string]any
	err = yaml.Unmarshal(mb, &manifestRaw)
	if err != nil {
		return nil, nil, err
	}

	manifestData := map[string]any{
		"data":      resolved,
		"resources": []map[string]any{},
	}

	ccm, ok := manifestRaw["ccm"].(map[string]any)
	if ok {
		manifestData["resources"] = ccm["resources"]
	}

	apply, err := apply.ParseManifestHiera(manifestData, &templates.Env{Data: resolved, Facts: facts}, hieraLogger)
	if err != nil {
		return nil, nil, err
	}

	return manifestData, apply, err
}

// ApplyManifest applies a parsed manifest and records all changes to the session store
func (m *CCM) ApplyManifest(ctx context.Context, apply model.Apply) (model.SessionStore, error) {
	err := m.session.StartSession(apply)
	if err != nil {
		return nil, err
	}

	for _, r := range apply.Resources() {
		for _, v := range r {
			switch resource := v.(type) {
			case *model.PackageResourceProperties:
				transaction, err := m.applyPackageResource(ctx, resource)
				if err != nil {
					return nil, err
				}

				m.session.RecordEvent(transaction)
			}
		}
	}

	return m.session, nil
}

// ApplyManifestReader reads, resolves, and applies a manifest from a reader
func (m *CCM) ApplyManifestReader(ctx context.Context, manifest io.ReadCloser) (model.SessionStore, error) {
	_, apply, err := m.ResolveManifestReader(ctx, manifest)
	if err != nil {
		return nil, err
	}

	return m.ApplyManifest(ctx, apply)
}

// FactsRaw returns the system facts as a JSON raw message
func (m *CCM) FactsRaw(ctx context.Context) (json.RawMessage, error) {
	f, err := m.Facts(ctx)
	if err != nil {
		return nil, err
	}

	j, err := json.Marshal(f)
	return j, err
}

// Facts gathers and returns the system facts
func (m *CCM) Facts(ctx context.Context) (map[string]any, error) {
	to, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return facts.StandardFacts(to)
}

// Logger creates a new logger with the provided key-value pairs added to the context
func (m *CCM) Logger(args ...any) (model.Logger, error) {
	if len(args)%2 != 0 {
		return nil, fmt.Errorf("invalid logger arguments, must be key value pairs")
	}

	return m.log.With(args...), nil
}

// NewRunner creates a new command runner instance
func (m *CCM) NewRunner() (model.CommandRunner, error) {
	log, err := m.Logger("component", "runner")
	if err != nil {
		return nil, err
	}

	return cmdrunner.NewCommandRunner(log)
}

func (m *CCM) RecordEvent(event model.TransactionEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session == nil {
		return fmt.Errorf("no session store available")
	}

	return m.session.RecordEvent(event)
}
