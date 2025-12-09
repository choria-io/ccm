// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/fisk"
)

type sessionCmd struct {
	sessionStore string
}

func registerSessionCommand(app *fisk.Application) {
	cmd := &sessionCmd{}

	sess := app.Command("session", "Manage session stores")

	newAction := sess.Command("new", "Creates a new session store").Action(cmd.newAction)
	newAction.Flag("directory", "Directory to store the session in").StringVar(&cmd.sessionStore)
}

func (c *sessionCmd) newAction(_ *fisk.ParseContext) error {
	var err error

	if c.sessionStore == "" {
		c.sessionStore, err = os.MkdirTemp("", "ccm-session-*")
		if err != nil {
			return err
		}
	} else {
		if iu.IsDirectory(c.sessionStore) {
			return fmt.Errorf("session store %s already exists", c.sessionStore)
		}
	}

	fmt.Printf("export CCM_SESSION_STORE=%v\n", c.sessionStore)

	return nil
}
