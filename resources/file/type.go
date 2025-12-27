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

	"github.com/choria-io/ccm/internal/registry"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/base"
	"github.com/choria-io/ccm/resources/file/posix"
)

type Type struct {
	*base.Base

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

	loggerArgs := []any{"type", model.FileTypeName, "name", properties.Name, "working_dir", mgr.WorkingDirectory()}
	logger, err := mgr.Logger(loggerArgs...)
	if err != nil {
		return nil, err
	}

	properties.CommonResourceProperties.Type = model.FileTypeName

	t := &Type{
		prop:  &properties,
		mgr:   mgr,
		log:   logger,
		facts: env.Facts,
		data:  env.Data,
	}
	t.Base = &base.Base{
		Resource:           t,
		ResourceProperties: &properties,
		CommonProperties:   properties.CommonResourceProperties,
		Log:                logger,
		UserLogger:         mgr.UserLogger().With(loggerArgs...),
		Manager:            mgr,
	}

	err = t.validate()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", t.String(), model.ErrResourceInvalid, err)
	}

	t.log.Debug("Created resource instance")

	return t, nil
}

func (t *Type) ApplyResource(ctx context.Context) (model.ResourceState, error) {
	var (
		initialStatus *model.FileState
		finalStatus   *model.FileState
		refreshState  bool
		p             = t.provider.(FileProvider)
		properties    = t.prop
		noop          = t.mgr.NoopMode()
		noopMessage   string
		err           error
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
		t.log.Debug("Creating file", "source", properties.Source, "working_dir", t.mgr.WorkingDirectory())

		if !noop {
			source := t.adjustedSource(properties)
			err = p.Store(ctx, properties.Name, []byte(properties.Contents), source, properties.Owner, properties.Group, properties.Mode)
			if err != nil {
				t.log.Error(fmt.Sprintf("Could not store new file %v", err))
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

	t.FinalizeState(finalStatus, noop, noopMessage, refreshState, isStable, false)

	return finalStatus, nil
}

func (t *Type) isDesiredState(properties *model.FileResourceProperties, state *model.FileState) (bool, string, error) {
	if properties.Ensure == model.EnsureAbsent {
		t.log.Debug("Checking if file is absent due to ensure=absent", "ensure", state.Ensure)
		return state.Ensure == model.EnsureAbsent, "", nil
	}

	var contentChecksum string
	var err error
	meta := state.Metadata.(*model.FileMetadata)

	if properties.Ensure != state.Ensure {
		t.log.Debug("Ensure does not match", "requested", properties.Ensure, "state", state.Ensure)
		return false, "", nil
	}

	if properties.Ensure != model.FileEnsureDirectory {
		if properties.Source != "" {
			path := t.adjustedSource(properties)
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

		if contentChecksum != meta.Checksum {
			t.log.Debug("Content does not match", "requested", contentChecksum, "state", meta.Checksum)
			return false, contentChecksum, nil
		}
	}

	if meta.Owner != properties.Owner {
		t.log.Debug("Owner does not match", "state", meta.Owner, "requested", properties.Owner)
		return false, contentChecksum, nil
	}

	if meta.Group != properties.Group {
		t.log.Debug("Group does not match", "state", meta.Group, "requested", properties.Group)
		return false, contentChecksum, nil
	}

	if meta.Mode != properties.Mode {
		t.log.Debug("Mode does not match", "state", meta.Mode, "requested", properties.Mode)
		return false, contentChecksum, nil
	}

	return true, contentChecksum, nil
}

func (t *Type) Info(ctx context.Context) (any, error) {
	_, err := t.SelectProvider()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.String(), err)
	}

	return t.provider.(FileProvider).Status(ctx, t.prop.Name)
}

func (t *Type) validate() error {
	if t.prop.SkipValidate {
		return nil
	}

	err := t.Base.Validate()
	if err != nil {
		return err
	}

	mode := t.prop.Mode

	// Strip common octal prefixes (0o, 0O)
	mode = strings.TrimPrefix(mode, "0o")
	mode = strings.TrimPrefix(mode, "0O")

	// Parse as octal number
	parsed, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return fmt.Errorf("mode %q is not a valid octal number: %w", t.prop.Mode, err)
	}

	// Validate it's within the valid Unix permission range (0-0777)
	if parsed > 0o777 {
		return fmt.Errorf("mode %q exceeds maximum value 0777", t.prop.Mode)
	}

	return t.prop.Validate()
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

func (t *Type) selectProviderUnlocked() error {
	// TODO: move to base

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

func (t *Type) SelectProvider() (string, error) {
	// TODO: move to base

	t.mu.Lock()
	defer t.mu.Unlock()

	err := t.selectProviderUnlocked()
	if err != nil {
		return "", err
	}

	return t.providerUnlocked(), nil
}

func (t *Type) adjustedSource(properties *model.FileResourceProperties) string {
	source := properties.Source
	if properties.Source != "" && t.mgr.WorkingDirectory() != "" {
		source = filepath.Join(t.mgr.WorkingDirectory(), properties.Source)
	}

	return source
}
