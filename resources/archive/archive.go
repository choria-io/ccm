// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package archiveresource

import (
	"context"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/archive/http"
)

type ArchiveFactory interface {
	model.ProviderFactory
}

func init() {
	http.Register()
}

type ArchiveProvider interface {
	model.Provider

	Download(ctx context.Context, properties *model.ArchiveResourceProperties, log model.Logger) error
	Extract(ctx context.Context, properties *model.ArchiveResourceProperties, log model.Logger) error
	Status(ctx context.Context, properties *model.ArchiveResourceProperties) (*model.ArchiveState, error)
}
