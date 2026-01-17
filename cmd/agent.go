// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"sync"

	"github.com/choria-io/ccm/agent"
	"github.com/choria-io/fisk"
)

type agentCommand struct {
	cfg         string
	natsContext string
}

func registerAgentCommand(ccm *fisk.Application) {
	cmd := &agentCommand{}

	ag := ccm.Command("agent", "Continuous manifest runner").Action(cmd.runAction)
	ag.Flag("config", "Configuration file to use").Required().ExistingFileVar(&cmd.cfg)
	ag.Flag("context", "NATS Context to connect with").Envar("NATS_CONTEXT").Default("CCM").StringVar(&cmd.natsContext)
}

func (a *agentCommand) runAction(_ *fisk.ParseContext) error {
	cfb, err := os.ReadFile(a.cfg)
	if err != nil {
		return err
	}

	cfg, err := agent.ParseConfig(cfb)
	if err != nil {
		return err
	}

	if a.natsContext != "" {
		cfg.NatsContext = a.natsContext
	}
	switch {
	case debug:
		cfg.LogLevel = "debug"
	case info:
		cfg.LogLevel = "info"
	}

	ag, err := agent.New(cfg)
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	err = ag.Run(ctx, &wg)
	if err != nil {
		return err
	}

	wg.Wait()

	return nil
}
