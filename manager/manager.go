// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/synadia-io/orbit.go/natscontext"

	"github.com/choria-io/ccm/internal/backoff"
	"github.com/choria-io/ccm/internal/cmdrunner"
	"github.com/choria-io/ccm/internal/facts"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
	archiveresource "github.com/choria-io/ccm/resources/archive"
	fileresource "github.com/choria-io/ccm/resources/file"
	packageresource "github.com/choria-io/ccm/resources/package"
	serviceresource "github.com/choria-io/ccm/resources/service"
	"github.com/choria-io/ccm/session"
	"github.com/choria-io/ccm/templates"
)

// CCM is the main configuration and change management orchestrator
type CCM struct {
	session    model.SessionStore
	log        model.Logger
	userLogger model.Logger
	js         jetstream.JetStream
	nc         *nats.Conn
	ncProvider model.NatsConnProvider

	noop        bool
	workingDir  string
	externData  map[string]any
	data        map[string]any
	facts       map[string]any
	env         map[string]string
	natsContext string

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

func (m *CCM) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.nc != nil {
		m.nc.Close()
	}

	m.js = nil
	m.data = nil
	m.facts = nil
	m.env = nil
	m.externData = nil

	return nil
}

// CopyFrom copies settings, data etc from source into m
func (m *CCM) CopyFrom(source model.Manager) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	src, ok := source.(*CCM)
	if !ok {
		return fmt.Errorf("cannot copy from non *CCM source %T", source)
	}

	src.mu.Lock()
	defer src.mu.Unlock()

	m.noop = src.noop
	m.workingDir = src.workingDir
	m.data = iu.CloneMap(src.data)
	m.facts = iu.CloneMap(src.facts)
	m.env = iu.CloneMapStrings(src.env)
	m.externData = iu.CloneMap(src.externData)
	m.natsContext = src.natsContext
	m.ncProvider = src.ncProvider
	m.nc = src.nc

	return nil
}

// JetStream returns a JetStream connection, requires the context be set using WithNatsContext()
func (m *CCM) JetStream() (jetstream.JetStream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.js != nil {
		return m.js, nil
	}

	if m.natsContext == "" {
		return nil, fmt.Errorf("nats context not set")
	}

	var err error

	if m.ncProvider != nil {
		m.nc, err = m.ncProvider.Connect(m.natsContext, DefaultNatsOptions(m.log)...)
	} else {
		m.nc, _, err = natscontext.Connect(m.natsContext, DefaultNatsOptions(m.log)...)
	}
	if err != nil {
		return nil, fmt.Errorf("nats connection error: %w", err)
	}

	m.js, err = jetstream.New(m.nc, jetstream.WithDefaultTimeout(2*time.Second))
	if err != nil {
		return nil, err
	}

	return m.js, nil
}

func DefaultNatsOptions(log model.Logger) []nats.Option {
	return []nats.Option{
		nats.Name("choria-ccm"),
		nats.MaxReconnects(-1),
		nats.IgnoreAuthErrorAbort(),
		nats.CustomReconnectDelay(func(attempt int) time.Duration {
			return backoff.Default.Duration(attempt)
		}),
		nats.RetryOnFailedConnect(true),
		nats.ReconnectErrHandler(func(conn *nats.Conn, err error) {
			if conn != nil {
				log.Error("NATS connection lost, attempting to reconnect", "error", conn.LastError())
			} else {
				log.Error("NATS connection lost, attempting to reconnect", "error", err.Error())
			}
		}),
		nats.ConnectHandler(func(conn *nats.Conn) {
			log.Debug("Connected to NATS Server", "server", conn.ConnectedUrlRedacted())
		}),
		nats.ClosedHandler(func(conn *nats.Conn) {
			if conn == nil || conn.LastError() == nil {
				log.Error("NATS connection closed")
			} else {
				log.Error("NATS connection closed", "error", conn.LastError())
			}
		}),
		nats.DisconnectErrHandler(func(conn *nats.Conn, err error) {
			if err == nil && conn != nil {
				err = conn.LastError()
			}
			opts := []any{}
			if err != nil {
				opts = append(opts, "error", err.Error())
			}

			log.Error("NATS connection disconnected", opts...)
		}),
		nats.ErrorHandler(func(conn *nats.Conn, subscription *nats.Subscription, err error) {
			if subscription != nil {
				log.Error("NATS subscription error", "error", err, "subject", subscription.Subject)
			} else {
				log.Error("NATS connection error", "error", err)
			}

		}),
		nats.ReconnectHandler(func(conn *nats.Conn) {
			log.Info("NATS connection reconnected", "server", conn.ConnectedUrlRedacted())
		}),
	}
}

// SetExternalData sets the external data for the manager that will be merged with manifest data
func (m *CCM) SetExternalData(data map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.externData = data
}

// SetWorkingDirectory sets the working directory for the manager, used in templates to load files etc
func (m *CCM) SetWorkingDirectory(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.workingDir = dir
}

// WorkingDirectory returns the working directory for the manager as set by SetWorkingDirectory()
func (m *CCM) WorkingDirectory() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.workingDir
}

// SetData sets the resolved Hiera data for the manager
func (m *CCM) SetData(data map[string]any) map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()

	copied := iu.DeepMergeMap(data, m.externData)
	m.data = copied

	return m.data
}

// SetEnviron sets the environment data for the manager
func (m *CCM) SetEnviron(env map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.env = env
}

// Environment returns the environment data set with SetEnviron()
func (m *CCM) Environment() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.env
}

// Data returns the resolved Hiera data
func (m *CCM) Data() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()

	ret := make(map[string]any)
	for k, v := range m.data {
		ret[k] = v
	}

	return ret
}

func (m *CCM) infoFileResource(ctx context.Context, prop *model.FileResourceProperties) (*model.FileMetadata, error) {
	abs, err := filepath.Abs(prop.Name)
	if err != nil {
		return nil, err
	}

	prop.Name = abs
	prop.SkipValidate = true

	ft, err := fileresource.New(ctx, m, *prop)
	if err != nil {
		return nil, err
	}

	nfo, err := ft.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get file info: %w", err)
	}

	return nfo.(*model.FileState).Metadata, nil
}

func (m *CCM) infoServiceResource(ctx context.Context, prop *model.ServiceResourceProperties) (*model.ServiceMetadata, error) {
	prop.SkipValidate = true

	ft, err := serviceresource.New(ctx, m, *prop)
	if err != nil {
		return nil, err
	}

	nfo, err := ft.Info(ctx)
	if err != nil {
		return nil, err
	}

	return nfo.(*model.ServiceState).Metadata, nil
}

func (m *CCM) infoPackageResource(ctx context.Context, prop *model.PackageResourceProperties) (*model.PackageMetadata, error) {
	prop.SkipValidate = true

	ft, err := packageresource.New(ctx, m, *prop)
	if err != nil {
		return nil, err
	}

	nfo, err := ft.Info(ctx)
	if err != nil {
		return nil, err
	}

	return nfo.(*model.PackageState).Metadata, nil
}

func (m *CCM) infoArchiveResource(ctx context.Context, prop *model.ArchiveResourceProperties) (*model.ArchiveMetadata, error) {
	prop.SkipValidate = true

	ft, err := archiveresource.New(ctx, m, *prop)
	if err != nil {
		return nil, err
	}

	nfo, err := ft.Info(ctx)
	if err != nil {
		return nil, err
	}

	return nfo.(*model.ArchiveState).Metadata, nil
}

// ResourceInfo returns information about a resource of the given type and name
func (m *CCM) ResourceInfo(ctx context.Context, typeName, name string) (any, error) {
	props, err := model.NewResourcePropertiesFromYaml(typeName, yaml.RawMessage(fmt.Sprintf("name: %q", name)), &templates.Env{})
	if err != nil {
		return nil, err
	}

	if len(props) != 1 {
		return nil, fmt.Errorf("expected exactly one resource of type %s, found %d", typeName, len(props))
	}
	prop := props[0]

	switch typeName {
	case model.FileTypeName:
		return m.infoFileResource(ctx, prop.(*model.FileResourceProperties))
	case model.ServiceTypeName:
		return m.infoServiceResource(ctx, prop.(*model.ServiceResourceProperties))
	case model.PackageTypeName:
		return m.infoPackageResource(ctx, prop.(*model.PackageResourceProperties))
	case model.ArchiveTypeName:
		return m.infoArchiveResource(ctx, prop.(*model.ArchiveResourceProperties))
	case model.ExecTypeName:
		return nil, fmt.Errorf("exec resources do not support retrieving status")
	default:
		return nil, fmt.Errorf("unsupported resource type %s", typeName)
	}
}

func (m *CCM) StartSession(apply model.Apply) (model.SessionStore, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session == nil {
		return nil, fmt.Errorf("no session store available")
	}

	return m.session, m.session.StartSession(apply)
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

func (m *CCM) SetFacts(facts map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.facts = facts
}

// MergeFacts merges the provided facts with the facts as gathered by Facts(), which may have been set by SetFacts()
func (m *CCM) MergeFacts(ctx context.Context, facts map[string]any) (map[string]any, error) {
	sf, err := m.Facts(ctx)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.facts = iu.DeepMergeMap(sf, facts)

	return m.facts, nil
}

// SystemFacts returns the system facts, without caching
func (m *CCM) SystemFacts(ctx context.Context) (map[string]any, error) {
	var to context.Context
	var cancel context.CancelFunc

	_, ok := ctx.Deadline()
	if ok {
		to = ctx
	} else {
		to, cancel = context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
	}

	return facts.StandardFacts(to, m.log)
}

// Facts gather system facts, cache them, and return them, if already cached return the cache
func (m *CCM) Facts(ctx context.Context) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.facts != nil {
		return m.facts, nil
	}

	f, err := m.SystemFacts(ctx)
	if err != nil {
		return nil, err
	}

	m.facts = f

	return f, nil
}

// Logger creates a new logger with the provided key-value pairs added to the context
func (m *CCM) Logger(args ...any) (model.Logger, error) {
	if len(args)%2 != 0 {
		return nil, fmt.Errorf("invalid logger arguments, must be key value pairs")
	}

	return m.log.With(args...), nil
}

// UserLogger returns the user logger
func (m *CCM) UserLogger() model.Logger {
	return m.userLogger
}

// NewRunner creates a new command runner instance
func (m *CCM) NewRunner() (model.CommandRunner, error) {
	log, err := m.Logger("component", "runner")
	if err != nil {
		return nil, err
	}

	return cmdrunner.NewCommandRunner(log)
}

// RecordEvent records a transaction event in the session store
func (m *CCM) RecordEvent(event *model.TransactionEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session == nil {
		return fmt.Errorf("no session store available")
	}
	if event.Name == "" {
		return fmt.Errorf("event name cannot be empty")
	}
	if event.ResourceType == "" {
		return fmt.Errorf("resource type cannot be empty")
	}

	return m.session.RecordEvent(event)
}

func (m *CCM) findEvents(resourceType string, resourceName string) ([]model.TransactionEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session == nil {
		return nil, fmt.Errorf("no session store available")
	}

	events, err := m.session.EventsForResource(resourceType, resourceName)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve events for %s#%s: %w", resourceType, resourceName, err)
	}

	return events, nil
}

// ShouldRefresh returns true if the last transaction event for the resource indicated by the type and name was changed
func (m *CCM) ShouldRefresh(resourceType string, resourceName string) (bool, error) {
	events, err := m.findEvents(resourceType, resourceName)
	if err != nil {
		return false, fmt.Errorf("could not retrieve events for %s#%s: %w", resourceType, resourceName, err)
	}

	if len(events) == 0 {
		return false, fmt.Errorf("no events found for %s#%s", resourceType, resourceName)
	}

	return events[len(events)-1].Changed, nil
}

func (m *CCM) IsResourceFailed(resourceType string, resourceName string) (bool, error) {
	events, err := m.findEvents(resourceType, resourceName)
	if err != nil {
		return false, fmt.Errorf("could not retrieve events for %s#%s: %w", resourceType, resourceName, err)
	}

	if len(events) == 0 {
		return false, fmt.Errorf("no events found for %s#%s", resourceType, resourceName)
	}

	e := events[len(events)-1]

	return e.Failed || len(e.UnmetRequirements) > 0, nil
}

func (m *CCM) SessionSummary() (*model.SessionSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session == nil {
		return nil, fmt.Errorf("no session store available")
	}

	events, err := m.session.AllEvents()
	if err != nil {
		return nil, err
	}

	return model.BuildSessionSummary(events), nil
}

func (m *CCM) TemplateEnvironment(ctx context.Context) (*templates.Env, error) {
	f, err := m.Facts(ctx)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return &templates.Env{Facts: f, Data: m.data, Environ: m.env, WorkingDir: m.workingDir}, nil
}

func (m *CCM) NoopMode() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.noop
}
