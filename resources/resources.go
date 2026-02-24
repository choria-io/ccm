// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/choria-io/ccm/model"
	archiveresource "github.com/choria-io/ccm/resources/archive"
	execresource "github.com/choria-io/ccm/resources/exec"
	fileresource "github.com/choria-io/ccm/resources/file"
	packageresource "github.com/choria-io/ccm/resources/package"
	scaffoldresource "github.com/choria-io/ccm/resources/scaffold"
	serviceresource "github.com/choria-io/ccm/resources/service"
)

// NewResourceFromProperties creates a new resource from a properties struct
func NewResourceFromProperties(ctx context.Context, mgr model.Manager, props model.ResourceProperties) (model.Resource, error) {
	switch rprop := props.(type) {
	case *model.PackageResourceProperties:
		return packageresource.New(ctx, mgr, *rprop)
	case *model.ServiceResourceProperties:
		return serviceresource.New(ctx, mgr, *rprop)
	case *model.FileResourceProperties:
		return fileresource.New(ctx, mgr, *rprop)
	case *model.ExecResourceProperties:
		return execresource.New(ctx, mgr, *rprop)
	case *model.ArchiveResourceProperties:
		return archiveresource.New(ctx, mgr, *rprop)
	case *model.ScaffoldResourceProperties:
		return scaffoldresource.New(ctx, mgr, *rprop)
	default:
		return nil, fmt.Errorf("unsupported resource property type %T", rprop)
	}
}
