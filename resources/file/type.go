// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package fileresource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/choria-io/ccm/healthcheck"
	"github.com/choria-io/ccm/internal/registry"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/file/posix"
)

type Type struct {
	prop     *model.FileResourceProperties
	mgr      model.Manager
	log      model.Logger
	provider model.Provider
	facts    map[string]any
	data     map[string]any

	mu sync.Mutex
}

var _ model.Resource = (*Type)(nil)
var _ FileProvider = (*posix.Provider)(nil)

// New creates a new file resource with the given properties
func New(ctx context.Context, mgr model.Manager, properties model.FileResourceProperties) (*Type, error) {
	env, err := mgr.TemplateEnvironment(ctx)
	if err != nil {
		return nil, err
	}

	err = properties.ResolveTemplates(env)
	if err != nil {
		return nil, err
	}

	err = properties.Validate()
	if err != nil {
		return nil, err
	}

	logger, err := mgr.Logger("type", model.FileTypeName, "name", properties.Name)
	if err != nil {
		return nil, err
	}

	t := &Type{
		prop:  &properties,
		mgr:   mgr,
		log:   logger,
		facts: env.Facts,
		data:  env.Data,
	}

	err = t.validate()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", t.String(), model.ErrResourceInvalid, err)
	}

	t.log.Debug("Created resource instance")

	return t, nil
}

func (t *Type) Apply(ctx context.Context) (*model.TransactionEvent, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	event := t.newTransactionEvent()
	start := time.Now()

	state, err := t.apply(ctx)
	event.Duration = time.Since(start)
	if err != nil {
		event.Failed = true
		event.Error = err.Error()
	}

	if t.prop.HealthCheck != nil {
		res, err := healthcheck.Execute(ctx, t.mgr, t.prop.HealthCheck, t.log)
		event.HealthCheck = res
		if err != nil {
			event.Failed = true
			event.Error = err.Error()
		} else {
			if res.Status != model.HealthCheckOK {
				event.Failed = true
				event.Error = fmt.Sprintf("health check status %q", res.Status.String())
			}
		}
	}

	if state != nil {
		event.Status = *state
		event.Changed = state.Changed
		event.ActualEnsure = state.Ensure
		event.Noop = state.Noop
		event.NoopMessage = state.NoopMessage
	}
	event.Provider = t.providerUnlocked()

	return event, nil
}

func (t *Type) apply(ctx context.Context) (*model.FileState, error) {
	err := t.selectProviderUnlocked()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.stringUnlocked(), err)
	}

	var (
		initialStatus *model.FileState
		finalStatus   *model.FileState
		refreshState  bool
		p             = t.provider.(FileProvider)
		properties    = t.prop
		noop          = t.mgr.NoopMode()
		noopMessage   string
	)

	initialStatus, err = p.Status(ctx, t.prop.Name)
	if err != nil {
		return nil, err
	}

	isStable, _, err := t.isDesiredState(properties, initialStatus)
	if err != nil {
		return nil, err
	}

	switch {
	case isStable:
	// nothing to do
	case properties.Ensure == model.FileEnsureDirectory:
		if !noop {
			t.log.Info("Creating directory")
			err = p.CreateDirectory(ctx, properties.Name, properties.Owner, properties.Group, properties.Mode)
			if err != nil {
				return nil, err
			}
		} else {
			t.log.Info("Skipping create directory as noop")
			noopMessage = "Would have created directory"
		}
		refreshState = true
	case properties.Ensure == model.EnsureAbsent && initialStatus.Ensure != model.EnsureAbsent:
		// remove
		if !noop {
			t.log.Info("Removing file due to ensure=absent")
			err = os.Remove(properties.Name)
			if err != nil {
				return nil, err
			}
		} else {
			t.log.Info("Skipping remove as noop")
			noopMessage = "Would have removed the file"
		}
		refreshState = true
	default:
		// create
		if !noop {
			t.log.Info("Creating file")
			err = p.Store(ctx, properties.Name, []byte(properties.Contents), properties.Source, properties.Owner, properties.Group, properties.Mode)
			if err != nil {
				t.log.Error("Could not store new file", "error", err)
				return nil, err
			}
		} else {
			t.log.Info("Skipping create as noop")
			noopMessage = "Would have created the file"
		}
		refreshState = true
	}

	if refreshState && !noop {
		finalStatus, err = p.Status(ctx, properties.Name)
		if err != nil {
			return nil, err
		}
	} else {
		finalStatus = initialStatus
	}

	if !noop {
		isStable, _, _ = t.isDesiredState(properties, finalStatus)
		if !isStable {
			return nil, fmt.Errorf("failed to reach desired state %s", properties.Ensure)
		}
	}

	finalStatus.Noop = noop
	finalStatus.NoopMessage = noopMessage
	finalStatus.Changed = refreshState // we mark it changed even in noop mode since noop guesses what would have happened

	return finalStatus, nil
}

func (t *Type) isDesiredState(properties *model.FileResourceProperties, state *model.FileState) (bool, string, error) {
	if properties.Ensure == model.EnsureAbsent {
		t.log.Debug("Checking if file is absent due to ensure=absent", "ensure", state.Ensure)
		return state.Ensure == model.EnsureAbsent, "", nil
	}

	var contentChecksum string
	var err error

	if properties.Ensure != state.Ensure {
		t.log.Debug("Ensure does not match", "requested", properties.Ensure, "state", state.Ensure)
		return false, "", nil
	}

	if properties.Ensure != model.FileEnsureDirectory {
		if properties.Source != "" {
			path, err := filepath.Abs(properties.Source)
			if err != nil {
				return false, "", fmt.Errorf("failed to find absolute path for source: %w", err)
			}

			contentChecksum, err = iu.Sha256HashFile(path)
			if err != nil {
				return false, "", err
			}
		} else {
			contentChecksum, err = iu.Sha256HashBytes([]byte(properties.Contents))
			if err != nil {
				return false, "", err
			}
		}

		if contentChecksum != state.Metadata.Checksum {
			t.log.Debug("Content does not match", "requested", contentChecksum, "state", state.Metadata.Checksum)
			return false, contentChecksum, nil
		}
	}

	if state.Metadata.Owner != properties.Owner {
		t.log.Debug("Owner does not match", "state", state.Metadata.Owner, "requested", properties.Owner)
		return false, contentChecksum, nil
	}

	if state.Metadata.Group != properties.Group {
		t.log.Debug("Group does not match", "state", state.Metadata.Group, "requested", properties.Group)
		return false, contentChecksum, nil
	}

	if state.Metadata.Mode != properties.Mode {
		t.log.Debug("Mode does not match", "state", state.Metadata.Mode, "requested", properties.Mode)
		return false, contentChecksum, nil
	}

	return true, contentChecksum, nil
}

func (t *Type) newTransactionEvent() *model.TransactionEvent {
	event := model.NewTransactionEvent(model.FileTypeName, t.prop.Name)
	if t.prop != nil {
		event.Properties = t.prop
		event.Name = t.prop.Name
		event.Ensure = t.prop.Ensure
	}

	return event
}

func (t *Type) Info(ctx context.Context) (any, error) {
	err := t.selectProvider()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.String(), err)
	}

	return t.provider.(FileProvider).Status(ctx, t.prop.Name)
}

func (t *Type) validate() error {
	mode := t.prop.Mode

	// Strip common octal prefixes (0o, 0O)
	mode = strings.TrimPrefix(mode, "0o")
	mode = strings.TrimPrefix(mode, "0O")

	// Parse as octal number
	parsed, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return fmt.Errorf("mode %q is not a valid octal number: %w", t.prop.Mode, err)
	}

	// Validate it's within valid Unix permission range (0-0777)
	if parsed > 0o777 {
		return fmt.Errorf("mode %q exceeds maximum value 0777", t.prop.Mode)
	}

	return t.prop.Validate()
}

func (t *Type) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.stringUnlocked()
}

func (t *Type) stringUnlocked() string {
	return fmt.Sprintf("%s#%s", model.FileTypeName, t.prop.Name)
}

func (t *Type) Type() string {
	return model.FileTypeName
}

func (t *Type) Name() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.prop.Name
}

func (t *Type) providerUnlocked() string {
	if t.provider == nil {
		return ""
	}

	return t.provider.Name()
}
func (t *Type) Provider() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.providerUnlocked()
}

func (t *Type) Properties() any {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.prop
}

func (t *Type) selectProviderUnlocked() error {
	if t.provider != nil {
		return nil
	}

	selected, err := registry.FindSuitableProvider(model.FileTypeName, t.prop.Provider, t.facts, t.log, nil)
	if err != nil {
		return err
	}

	if selected == nil {
		return model.ErrNoSuitableProvider
	}

	t.log.Debug("Selected provider", "provider", selected.Name())
	t.provider = selected

	return nil
}

func (t *Type) selectProvider() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.selectProviderUnlocked()
}
