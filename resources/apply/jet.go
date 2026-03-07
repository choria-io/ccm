// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"bytes"
	"fmt"
	"os"

	"github.com/CloudyKit/jet/v6"
	"github.com/goccy/go-yaml"

	"github.com/choria-io/ccm/templates"
)

func jetParseManifestResources(path string, env *templates.Env) (yaml.RawMessage, error) {
	jb, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	set := jet.NewSet(jet.NewInMemLoader(), jet.WithDelims("[[", "]]"))

	for k, v := range env.JetFunctions() {
		set.AddGlobalFunc(k, v)
	}

	tpl, err := set.Parse(path, string(jb))
	if err != nil {
		return nil, err
	}

	buff := bytes.NewBuffer([]byte{})
	err = tpl.Execute(buff, env.JetVariables(), env)
	if err != nil {
		return nil, err
	}

	if buff.Len() == 0 {
		return nil, fmt.Errorf("resources jet template produced no output, check for syntax errors")
	}

	return buff.Bytes(), nil
}
