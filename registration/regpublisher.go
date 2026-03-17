// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"fmt"

	"github.com/choria-io/ccm/model"
)

func New(mgr model.Manager, destination model.RegistrationDestination) (model.RegistrationPublisher, error) {
	logger, err := mgr.Logger("registration", destination)
	if err != nil {
		return nil, err
	}

	switch destination {
	case model.NatsRegistrationDestination:
		nc, err := mgr.NatsConnection()
		if err != nil {
			return nil, err
		}

		return newNatsPublisher(nc, logger)

	case model.JetStreamRegistrationDestination:
		nc, err := mgr.NatsConnection()
		if err != nil {
			return nil, err
		}

		return newJetStreamPublisher(nc, mgr.RegistrationStream(), logger)

	default:
		return nil, fmt.Errorf("unknown registration destination %q", destination)
	}
}
