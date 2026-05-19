// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package templates

import (
	"fmt"
	"text/template"
)

func (e *Env) GoFunctions() template.FuncMap {
	return map[string]interface{}{
		"lookup":        e.lookup,
		"readFile":      e.goReadFile,
		"file":          e.goReadFile,
		"registrations": e.registrations,
		"jet":           e.goJet,
	}
}

func (e *Env) goReadFile(file string) (string, error) {
	res, err := e.readFile(file)
	if err != nil {
		return "", err
	}

	return res.(string), nil
}

func (e *Env) goJet(params ...any) (string, error) {
	res, err := e.jet(params...)
	if err != nil {
		return "", err
	}

	s, ok := res.(string)
	if !ok {
		return "", fmt.Errorf("jet did not return a string")
	}

	return s, nil
}
