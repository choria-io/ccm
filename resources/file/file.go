// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package fileresource

import (
	"context"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/file/posix"
)

func init() {
	posix.Register()
}

type FileProvider interface {
	model.Provider

	CreateDirectory(ctx context.Context, dir string, owner string, group string, mode string) error
	Store(ctx context.Context, file string, contents []byte, source string, owner string, group string, mode string) error
	Status(ctx context.Context, file string) (*model.FileState, error)
}
