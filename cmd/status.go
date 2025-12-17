// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/choria-io/fisk"
)

type statusCommand struct {
	json bool
}

func registerStatusCommand(ccm *fisk.Application) {
	cmd := &statusCommand{}

	status := ccm.Command("status", "Get resource status")
	status.Flag("json", "Output status in JSON format").UnNegatableBoolVar(&cmd.json)

	registerStatusFileCommand(status, cmd)
	registerStatusPackageCommand(status, cmd)
	registerStatusServiceCommand(status, cmd)
}
