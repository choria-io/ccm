// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"bytes"
	"os"
	"reflect"

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
	tpl, err := set.Parse(path, string(jb))
	if err != nil {
		return nil, err
	}

	variables := jet.VarMap{
		"facts":   reflect.ValueOf(env.Facts),
		"Facts":   reflect.ValueOf(env.Facts),
		"data":    reflect.ValueOf(env.Data),
		"Data":    reflect.ValueOf(env.Data),
		"environ": reflect.ValueOf(env.Environ),
		"Environ": reflect.ValueOf(env.Environ),
	}

	buff := bytes.NewBuffer([]byte{})
	err = tpl.Execute(buff, variables, env)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}
