// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/choria-io/fisk"
)

type registrationCommand struct {
	registrationStream string
	natsContext        string
}

func registerRegistrationCommand(ccm *fisk.Application) {
	cmd := &registrationCommand{}

	reg := ccm.Command("registration", "Node Registration system").Alias("reg").Alias("r")

	registerRegistrationInitCommand(reg, cmd)
	registerRegistrationQueryCommand(reg, cmd)
	registerRegistrationWatchCommand(reg, cmd)

	reg.Flag("context", "NATS Context to connect with").Envar("NATS_CONTEXT").Default("CCM").StringVar(&cmd.natsContext)
}
