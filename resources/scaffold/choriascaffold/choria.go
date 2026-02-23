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
	files = append(files, state.Metadata.Purged...)
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

	return nil
}

func (p *Provider) Scaffold(ctx context.Context, env *templates.Env, prop *model.ScaffoldResourceProperties, noop bool) (*model.ScaffoldState, error) {
	return p.render(ctx, env, prop, noop)
}

func (p *Provider) Status(ctx context.Context, env *templates.Env, prop *model.ScaffoldResourceProperties) (*model.ScaffoldState, error) {
	return p.render(ctx, env, prop, true)
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
		s, err = scaffold.New(cfg, nil)
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
		}
	}
	return state, nil
}
