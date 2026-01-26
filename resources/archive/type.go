// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package archiveresource

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/base"
)

type Type struct {
	*base.Base

	prop     *model.ArchiveResourceProperties
	mgr      model.Manager
	log      model.Logger
	provider model.Provider
	facts    map[string]any
	data     map[string]any

	mu sync.Mutex
}

var _ model.Resource = (*Type)(nil)

func New(ctx context.Context, mgr model.Manager, properties model.ArchiveResourceProperties) (*Type, error) {
	env, err := mgr.TemplateEnvironment(ctx)
	if err != nil {
		return nil, err
	}

	err = properties.ResolveTemplates(env)
	if err != nil {
		return nil, err
	}

	loggerArgs := []any{"type", model.ArchiveTypeName, "name", properties.Name}
	logger, err := mgr.Logger(loggerArgs...)
	if err != nil {
		return nil, err
	}

	properties.CommonResourceProperties.Type = model.ArchiveTypeName

	t := &Type{
		prop:  &properties,
		mgr:   mgr,
		log:   logger,
		facts: env.Facts,
		data:  env.Data,
	}
	t.Base = &base.Base{
		Resource:           t,
		CommonProperties:   properties.CommonResourceProperties,
		ResourceProperties: &properties,
		Log:                logger,
		UserLogger:         mgr.UserLogger().With(loggerArgs...),
		Manager:            mgr,
	}

	err = t.Base.Validate()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", t.String(), model.ErrResourceInvalid, err)
	}

	t.log.Debug("Created resource instance")

	return t, nil
}

func (t *Type) ApplyResource(ctx context.Context) (model.ResourceState, error) {
	var (
		initialStatus *model.ArchiveState
		finalStatus   *model.ArchiveState
		refreshState  bool
		p             = t.provider.(ArchiveProvider)
		properties    = t.prop
		noop          = t.mgr.NoopMode()
		noopMessage   []string
		err           error
	)

	initialStatus, err = p.Status(ctx, t.prop)
	if err != nil {
		return nil, err
	}

	isStable, _, err := t.isDesiredState(properties, initialStatus)
	if err != nil {
		return nil, err
	}

	// Early exit for stable state
	if isStable {
		t.FinalizeState(initialStatus, noop, "", false, true, false)
		return initialStatus, nil
	}

	// TODO: these nested if blocks are terrible
	refreshState = true

	if properties.Ensure == model.EnsureAbsent {
		// remove the archive
		// TODO: decide if we should also remove the extracted target
		if !noop {
			t.log.Info("Removing archive")
			err = os.Remove(properties.Name)
			if err != nil {
				return nil, err
			}
		} else {
			t.log.Info("Skipping removal as noop")
			noopMessage = append(noopMessage, "Would have removed")
		}
	} else {
		checksumMatches := initialStatus.Metadata.Checksum != "" && initialStatus.Metadata.Checksum == properties.Checksum
		exists := initialStatus.Metadata.ArchiveExists

		var forceExtract bool
		if !(checksumMatches && exists) {
			forceExtract = true

			if !noop {
				t.log.Info("Downloading archive")
				err = p.Download(ctx, properties, t.log)
				if err != nil {
					return nil, fmt.Errorf("download failed: %w", err)
				}
			} else {
				t.log.Info("Skipping download as noop")
				noopMessage = append(noopMessage, "Would have downloaded")
			}
		}

		if properties.ExtractParent != "" && (forceExtract || !initialStatus.Metadata.CreatesExists) {
			if !noop {
				t.log.Info("Extracting archive")
				// if we have to extract it, then extract it if needed like when the archive changed even if the dest exists already
				err = p.Extract(ctx, properties, t.log)
				if err != nil {
					return nil, err
				}
			} else {
				t.log.Info("Skipping extraction as noop")
				noopMessage = append(noopMessage, "Would have extracted")
			}
		}

		if properties.Cleanup {
			if !noop {
				t.log.Info("Cleaning up archive")
				// cleanup the archive
				err = os.Remove(properties.Name)
				if err != nil {
					return nil, err
				}
			} else {
				t.log.Info("Skipping cleanup as noop")
				noopMessage = append(noopMessage, "Would have cleaned up")
			}
		}
	}

	finalStatus = initialStatus
	if !noop {
		finalStatus, err = p.Status(ctx, properties)
		if err != nil {
			return nil, err
		}

		isStable, _, _ = t.isDesiredState(properties, finalStatus)
		if !isStable {
			return nil, fmt.Errorf("%w: %s", model.ErrDesiredStateFailed, properties.Ensure)
		}
	}

	t.FinalizeState(finalStatus, noop, strings.Join(noopMessage, ". "), refreshState, isStable, false)

	return finalStatus, nil
}

func (t *Type) isDesiredState(properties *model.ArchiveResourceProperties, state *model.ArchiveState) (bool, string, error) {
	if properties.Ensure == model.EnsureAbsent {
		return state.Ensure == model.EnsureAbsent, "", nil
	}

	meta := state.Metadata

	// If Creates is specified, the extraction marker file must exist
	if properties.Creates != "" && !meta.CreatesExists {
		t.log.Debug("Creates file does not exist", "creates", properties.Creates)
		return false, "", nil
	}

	// If cleanup is true and creates is not set the archive file must not exist
	if properties.Cleanup && properties.Creates == "" && meta.ArchiveExists {
		t.log.Debug("Archive file exists and cleanup is true")
		return false, "", nil
	}

	// If Cleanup is false, the archive file must exist
	if !properties.Cleanup && !meta.ArchiveExists {
		t.log.Debug("Archive file does not exist and cleanup is false")
		return false, "", nil
	}

	// If archive exists, verify owner, group, and checksum
	if meta.ArchiveExists {
		if meta.Owner != properties.Owner {
			t.log.Debug("Owner does not match", "state", meta.Owner, "requested", properties.Owner)
			return false, "", nil
		}

		if meta.Group != properties.Group {
			t.log.Debug("Group does not match", "state", meta.Group, "requested", properties.Group)
			return false, "", nil
		}

		if properties.Checksum != "" && meta.Checksum != properties.Checksum {
			t.log.Debug("Checksum does not match", "state", meta.Checksum, "requested", properties.Checksum)
			return false, "", nil
		}
	}

	return true, "", nil
}

func (t *Type) Info(ctx context.Context) (any, error) {
	_, err := t.SelectProvider()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.String(), err)
	}

	return t.provider.(ArchiveProvider).Status(ctx, t.prop)
}

func (t *Type) selectProviderUnlocked() error {
	// TODO: move to base

	if t.provider != nil {
		return nil
	}

	runner, err := t.mgr.NewRunner()
	if err != nil {
		return err
	}

	t.log.Debug("Trying to find providers")
	selected, err := registry.FindSuitableProvider(model.ArchiveTypeName, t.prop.Provider, t.facts, t.prop, t.log, runner)
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
