// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"time"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/manager"
	"github.com/choria-io/fisk"
)

type sessionCmd struct {
	sessionStore string
}

func registerSessionCommand(app *fisk.Application) {
	cmd := &sessionCmd{}

	sess := app.Command("session", "Manage session stores")

	newAction := sess.Command("new", "Creates a new session store").Alias("start").Action(cmd.newAction)
	newAction.Flag("directory", "Directory to store the session in").StringVar(&cmd.sessionStore)

	reportAction := sess.Command("report", "Report on the active session").Action(cmd.reportAction)
	reportAction.Flag("session", "Session store to use").Envar("CCM_SESSION_STORE").StringVar(&cmd.sessionStore)

}

func (c *sessionCmd) reportAction(_ *fisk.ParseContext) error {
	if c.sessionStore == "" {
		return fmt.Errorf("no session store specified")
	}

	mgr, err := manager.NewManager(newLogger(), newOutputLogger(), manager.WithSessionDirectory(c.sessionStore))
	if err != nil {
		return err
	}

	summary, err := mgr.SessionSummary()
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Session Summary")
	fmt.Println()
	if summary.TotalDuration > 0 {
		fmt.Printf("             Run Time: %v\n", summary.TotalDuration.Round(time.Millisecond))
	}
	fmt.Printf("      Total Resources: %d\n", summary.TotalResources)
	fmt.Printf("     Unique Resources: %d\n", summary.UniqueResources)
	fmt.Printf("     Stable Resources: %d\n", summary.StableResources)
	fmt.Printf("    Changed Resources: %d\n", summary.ChangedResources)
	fmt.Printf("     Failed Resources: %d\n", summary.FailedResources)
	fmt.Printf("    Skipped Resources: %d\n", summary.SkippedResources)
	fmt.Printf("  Refreshed Resources: %d\n", summary.RefreshedCount)
	fmt.Printf("         Total Errors: %d\n", summary.TotalErrors)

	return nil
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
