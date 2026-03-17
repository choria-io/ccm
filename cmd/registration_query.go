// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/goccy/go-yaml"

	"github.com/choria-io/ccm/registration"
	"github.com/choria-io/fisk"
)

type registrationQueryCommand struct {
	cluster  string
	protocol string
	service  string
	ip       string
	json     bool
	yaml     bool
	parent   *registrationCommand
}

func registerRegistrationQueryCommand(reg *fisk.CmdClause, parent *registrationCommand) {
	cmd := &registrationQueryCommand{parent: parent}

	q := reg.Command("query", "Query the registration store").Alias("q").Action(cmd.queryAction)
	q.Arg("cluster", "The cluster to query").Default("*").StringVar(&cmd.cluster)
	q.Arg("protocol", "the protocol to query").Default("*").StringVar(&cmd.protocol)
	q.Arg("service", "The service to query").Default("*").StringVar(&cmd.service)
	q.Arg("ip", "The ip or host to query").Default("*").StringVar(&cmd.ip)
	q.Flag("json", "Render results in JSON format").Default("false").UnNegatableBoolVar(&cmd.json)
	q.Flag("yaml", "Render results in YAML format").Default("false").UnNegatableBoolVar(&cmd.yaml)
	q.Flag("registration", "The NATS Stream holding registration data").Default("REGISTRATION").Short('R').StringVar(&cmd.parent.registrationStream)
}

func (c *registrationQueryCommand) queryAction(_ *fisk.ParseContext) error {
	mgr, _, err := newManager("", "", c.parent.natsContext, false, false, c.parent.registrationStream, nil)
	if err != nil {
		return err
	}

	res, err := registration.JetStreamLookup(context.Background(), mgr, c.cluster, c.protocol, c.service, c.ip)
	if err != nil {
		return err
	}

	if c.json {
		j, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(j))

		return nil
	}

	if c.yaml {
		y, err := yaml.Marshal(res)
		if err != nil {
			return err
		}

		fmt.Println(string(y))

		return nil
	}

	if len(res) == 0 {
		fmt.Println("No results found")
		return nil
	}

	sort.Slice(res, func(i, j int) bool {
		if res[i].Cluster != res[j].Cluster {
			return res[i].Cluster < res[j].Cluster
		}
		if res[i].Protocol != res[j].Protocol {
			return res[i].Protocol < res[j].Protocol
		}
		return res[i].Service < res[j].Service
	})

	var cluster, service string

	for _, r := range res {
		if cluster != r.Cluster || service != r.Service {
			fmt.Printf("Service: %s @ %s\n\n", r.Service, r.Cluster)
		}

		fmt.Printf("       Cluster: %s\n", r.Cluster)
		fmt.Printf("      Protocol: %s\n", r.Protocol)
		fmt.Printf("       Service: %s\n", r.Service)
		fmt.Printf("            IP: %s\n", r.IP)
		fmt.Printf("          Port: %v\n", r.Port)
		fmt.Printf("      Priority: %d\n", r.Priority)
		fmt.Printf("   Annotations:\n")
		for k, v := range r.Annotations {
			fmt.Printf("                %s: %s\n", k, v)
		}
		fmt.Println()
	}

	return nil
}
