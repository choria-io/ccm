// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"fmt"
	"net/url"

	"github.com/choria-io/ccm/internal/registry"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
)

func Register() {
	registry.MustRegister(&factory{})
}

type factory struct{}

func (p *factory) TypeName() string { return model.ArchiveTypeName }
func (p *factory) Name() string     { return ProviderName }
func (p *factory) New(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
	return NewHttpProvider(log, runner)
}
func (p *factory) IsManageable(_ map[string]any, prop model.ResourceProperties) (bool, int, error) {
	ap, ok := prop.(*model.ArchiveResourceProperties)
	if !ok {
		return false, 0, fmt.Errorf("invalid properties %T", prop)
	}

	uri, err := url.Parse(ap.Url)
	if err != nil {
		return false, 0, fmt.Errorf("invalid URL %q: %w", ap.Url, err)
	}

	if uri.Scheme != "http" && uri.Scheme != "https" {
		return false, 0, nil
	}

	tool := toolForFileName(ap.Name)
	if tool == "" {
		return false, 0, nil
	}

	_, found, err := iu.ExecutableInPath(tool)
	if err != nil {
		return false, 0, err
	}
	if !found {
		return false, 0, nil
	}

	return true, 1, nil
}
