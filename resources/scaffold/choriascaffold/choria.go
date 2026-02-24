// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package choriascaffold

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/templates"
	"github.com/choria-io/scaffold"
)

const ProviderName = "choria"

type Provider struct {
	log model.Logger
}

func NewChoriaProvider(log model.Logger) (*Provider, error) {
	return &Provider{log: log}, nil
}

func (p *Provider) Name() string {
	return ProviderName
}

func (p *Provider) Remove(ctx context.Context, prop *model.ScaffoldResourceProperties, state *model.ScaffoldState) error {
	var files []string
	files = append(files, state.Metadata.Changed...)
	files = append(files, state.Metadata.Stable...)

	dirs := map[string]bool{}

	for _, f := range files {
		if !filepath.IsAbs(f) {
			return fmt.Errorf("cannot remove %s, it is not an absolute path", f)
		}

		p.log.Debug("Removing file", "file", f)
		err := os.Remove(f)
		if err != nil && !os.IsNotExist(err) {
			p.log.Error("Failed to remove file", "file", f, "error", err)
		}

		dirs[filepath.Dir(f)] = true
	}

	// remove empty directories deepest-first by iterating until no more
	// directories can be removed
	for {
		removed := false

		for d := range dirs {
			if d == prop.Name {
				continue
			}

			if !iu.IsEmptyDirectory(d) {
				continue
			}

			p.log.Debug("Removing empty directory", "dir", d)
			err := os.Remove(d)
			if err != nil {
				p.log.Error("Failed to remove directory", "dir", d, "error", err)
			}

			delete(dirs, d)
			dirs[filepath.Dir(d)] = true
			removed = true
		}

		if !removed {
			break
		}
	}

	// best-effort removal of the target directory, only succeeds if empty
	err := os.Remove(prop.Name)
	if err != nil {
		p.log.Debug("Target directory not removed", "dir", prop.Name, "error", err)
	}

	return nil
}

func (p *Provider) Scaffold(ctx context.Context, env *templates.Env, prop *model.ScaffoldResourceProperties, noop bool) (*model.ScaffoldState, error) {
	return p.render(ctx, env, prop, noop)
}

func (p *Provider) Status(ctx context.Context, env *templates.Env, prop *model.ScaffoldResourceProperties) (*model.ScaffoldState, error) {
	p.log.Debug("Getting scaffold status")
	state, err := p.render(ctx, env, prop, true)
	if err != nil {
		return nil, err
	}

	// The noop render tells us what would happen if we scaffolded. When removing,
	// we filter the lists to only include files that actually exist on disk so
	// that status reflects reality rather than intent.
	if prop.Ensure == model.EnsureAbsent {
		p.log.Debug("Adjusting state for purge")
		var changed []string
		for _, f := range state.Metadata.Changed {
			if iu.FileExists(f) {
				p.log.Debug("Changed file still exist", "file", f)
				changed = append(changed, f)
			}
		}
		state.Metadata.Changed = changed

		var stable []string
		for _, f := range state.Metadata.Stable {
			if iu.FileExists(f) {
				p.log.Debug("Stable file still exist", "file", f)
				stable = append(stable, f)
			}
		}
		state.Metadata.Stable = stable
	}

	return state, nil
}

func (p *Provider) render(_ context.Context, env *templates.Env, prop *model.ScaffoldResourceProperties, noop bool) (*model.ScaffoldState, error) {
	metadata := &model.ScaffoldMetadata{
		Name:         prop.Name,
		Provider:     ProviderName,
		Engine:       prop.Engine,
		TargetExists: iu.FileExists(prop.Name),
	}

	state := &model.ScaffoldState{
		CommonResourceState: model.NewCommonResourceState(model.ResourceStatusScaffoldProtocol, model.ScaffoldTypeName, prop.Name, model.EnsurePresent),
		Metadata:            metadata,
	}

	var s *scaffold.Scaffold
	var err error

	cfg := scaffold.Config{
		TargetDirectory:      prop.Name,
		SourceDirectory:      prop.Source,
		MergeTargetDirectory: true,
		Post:                 prop.Post,
		SkipEmpty:            prop.SkipEmpty,
		CustomLeftDelimiter:  prop.LeftDelimiter,
		CustomRightDelimiter: prop.RightDelimiter,
	}

	switch prop.Engine {
	case model.ScaffoldEngineGo:
		s, err = scaffold.New(cfg, env.GoFunctions())
	case model.ScaffoldEngineJet:
		s, err = scaffold.NewJet(cfg, env.JetFunctions())
	default:
		return nil, fmt.Errorf("unknown scaffold engine %s", prop.Engine)
	}
	if err != nil {
		return nil, err
	}

	s.Logger(&logger{p.log})

	var result []scaffold.ManagedFile
	if noop {
		result, err = s.RenderNoop(env.JetVariables())
	} else {
		result, err = s.Render(env.JetVariables())
	}
	if err != nil {
		p.log.Error("Failed to render scaffold", "error", err)
		return nil, err
	}

	for _, f := range result {
		switch f.Action {
		case scaffold.FileActionEqual:
			metadata.Stable = append(metadata.Stable, filepath.Join(prop.Name, f.Path))
		case scaffold.FileActionAdd, scaffold.FileActionUpdate:
			metadata.Changed = append(metadata.Changed, filepath.Join(prop.Name, f.Path))
		case scaffold.FileActionRemove:
			metadata.Purged = append(metadata.Purged, filepath.Join(prop.Name, f.Path))
			if prop.Purge {
				if noop {
					p.log.Info("Would have removed file", "file", filepath.Join(prop.Name, f.Path))
					continue
				}

				err = os.Remove(filepath.Join(prop.Name, f.Path))
				if err != nil {
					return nil, fmt.Errorf("failed to remove %v: %w", filepath.Join(prop.Name, f.Path), err)
				}
				p.log.Info("Removed file", "file", filepath.Join(prop.Name, f.Path))
			}
		}
	}
	return state, nil
}
