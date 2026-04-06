// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/tidwall/gjson"

	"github.com/choria-io/ccm/hiera"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/manager"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/fisk"
)

type hieraCommand struct {
	input       string
	factsInput  map[string]string
	factsFile   string
	envFacts    bool
	yamlOutput  bool
	envOutput   bool
	envPrefix   string
	dataInput   map[string]string
	dataFile    string
	query       string
	natsContext string
}

func registerHieraCommand(ccm *fisk.Application) {
	cmd := &hieraCommand{
		factsInput: make(map[string]string),
		dataInput:  make(map[string]string),
	}

	hiera := ccm.Command("hiera", "Hierarchical data resolver")

	parse := hiera.Command("parse", "Parses a YAML or JSON file and prints the result as JSON").Alias("resolve").Action(cmd.parseAction)
	parse.Arg("input", "Input JSON or YAML file to resolve").Envar("HIERA_INPUT").Required().StringVar(&cmd.input)
	parse.Flag("data", "Override data values (always strings)").Short('D').StringMapVar(&cmd.dataInput)
	parse.Flag("data-file", "JSON or YAML file containing override data values").PlaceHolder("FILE").ExistingFileVar(&cmd.dataFile)
	parse.Arg("fact", "Facts about the node").StringMapVar(&cmd.factsInput)
	parse.Flag("facts", "JSON or YAML file containing facts").ExistingFileVar(&cmd.factsFile)
	parse.Flag("env-facts", "Provide facts from the process environment").Short('E').UnNegatableBoolVar(&cmd.envFacts)
	parse.Flag("query", "Performs a gjson query on the result").StringVar(&cmd.query)
	parse.Flag("context", "NATS Context to connect with").Envar("NATS_CONTEXT").Default("CCM").StringVar(&cmd.natsContext)
	parse.Flag("yaml", "Output YAML instead of JSON").UnNegatableBoolVar(&cmd.yamlOutput)
	parse.Flag("env", "Output environment variables").UnNegatableBoolVar(&cmd.envOutput)
	parse.Flag("env-prefix", "Prefix for environment variable names").Default("HIERA").StringVar(&cmd.envPrefix)

	facts := hiera.Command("facts", "Shows resolved facts").Action(cmd.showFactsAction)
	facts.Arg("fact", "Facts about the node").StringMapVar(&cmd.factsInput)
	facts.Flag("facts", "JSON or YAML file containing facts").ExistingFileVar(&cmd.factsFile)
	facts.Flag("env-facts", "Provide facts from the process environment").Short('E').UnNegatableBoolVar(&cmd.envFacts)
	facts.Flag("query", "Performs a gjson query on the facts").StringVar(&cmd.query)
}

func (cmd *hieraCommand) showFactsAction(_ *fisk.ParseContext) error {
	facts, err := cmd.resolveFacts()
	if err != nil {
		return err
	}

	j, err := json.MarshalIndent(facts, "", "  ")
	if err != nil {
		return err
	}

	if cmd.query != "" {
		val := gjson.GetBytes(j, cmd.query)
		fmt.Println(val.String())
		return nil
	}

	fmt.Println(string(j))

	return nil
}
func (cmd *hieraCommand) parseAction(_ *fisk.ParseContext) error {
	facts, err := cmd.resolveFacts()
	if err != nil {
		return err
	}

	var res any
	var logger model.Logger

	if debug {
		logger = manager.NewSlogLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	} else {
		logger = manager.NewSlogLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	}

	mgr, _, err := newManager("", "", cmd.natsContext, false, true, "", nil)
	if err != nil {
		return err
	}

	hieraOpts := hiera.DefaultOptions

	if cmd.dataFile != "" {
		raw, err := os.ReadFile(cmd.dataFile)
		if err != nil {
			return fmt.Errorf("failed to read data file: %w", err)
		}

		fileData := map[string]any{}
		if iu.IsJsonObject(raw) {
			err = json.Unmarshal(raw, &fileData)
		} else {
			err = yaml.Unmarshal(raw, &fileData)
		}
		if err != nil {
			return fmt.Errorf("failed to parse data file: %w", err)
		}

		hieraOpts.DataOverrides = fileData
	}

	if len(cmd.dataInput) > 0 {
		if hieraOpts.DataOverrides == nil {
			hieraOpts.DataOverrides = iu.MapStringsToMapStringAny(cmd.dataInput)
		} else {
			for k, v := range cmd.dataInput {
				hieraOpts.DataOverrides[k] = v
			}
		}
	}

	hieraResult, err := hiera.ResolveUrl(ctx, cmd.input, mgr, facts, hieraOpts, logger)
	if err != nil {
		return err
	}

	res = hieraResult.Data

	if cmd.query != "" {
		jout, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return err
		}

		res = gjson.GetBytes(jout, cmd.query).Value()
	}

	var out []byte
	switch {
	case cmd.yamlOutput:
		out, err = yaml.Marshal(res)
	case cmd.envOutput:
		buff := bytes.NewBuffer([]byte{})
		err = cmd.renderEnvOutput(buff, res)
		out = buff.Bytes()
	default:
		out, err = json.MarshalIndent(res, "", "  ")
	}
	if err != nil {
		return err
	}

	fmt.Println(strings.TrimSpace(string(out)))

	return nil
}

func (cmd *hieraCommand) renderEnvOutput(w io.Writer, res any) error {
	switch val := res.(type) {
	case map[string]any:
		for k, v := range val {
			key := fmt.Sprintf("%s_%s", cmd.envPrefix, strings.ToUpper(k))

			switch typed := v.(type) {
			case string:
				fmt.Fprintf(w, "%s=%s\n", key, typed)
			case int8, int16, int32, int64, int:
				fmt.Fprintf(w, "%s=%v\n", key, typed)
			case float32, float64:
				fmt.Fprintf(w, "%s=%f\n", key, typed)
			default:
				j, err := json.Marshal(typed)
				if err != nil {
					return err
				}
				fmt.Fprintf(w, "%s=%s\n", key, string(j))
			}
		}
	case gjson.Result:
		j, _ := json.Marshal(val.Value())
		fmt.Fprintf(w, "%s_VALUE=%v\n", cmd.envPrefix, string(j))
	default:
		j, _ := json.Marshal(val)
		fmt.Fprintf(w, "%s_VALUE=%v\n", cmd.envPrefix, string(j))
	}

	return nil
}

func (cmd *hieraCommand) resolveFacts() (map[string]any, error) {
	facts := make(map[string]any)

	mgr, _, err := newManager("", "", "", false, true, "", nil)
	if err != nil {
		return nil, err
	}

	sf, err := mgr.Facts(ctx)
	if err != nil {
		return nil, err
	}
	for k, v := range sf {
		facts[k] = v
	}

	if cmd.envFacts {
		for _, v := range os.Environ() {
			kv := strings.Split(v, "=")
			facts[kv[0]] = kv[1]
		}
	}

	if cmd.factsFile != "" {
		fc, err := os.ReadFile(cmd.factsFile)
		if err != nil {
			return nil, err
		}

		if iu.IsJsonObject(fc) {
			err = json.Unmarshal(fc, &facts)
		} else {
			err = yaml.Unmarshal(fc, &facts)
		}
		if err != nil {
			return nil, err
		}
	}

	for k, v := range cmd.factsInput {
		facts[k] = v
	}

	return facts, nil
}
