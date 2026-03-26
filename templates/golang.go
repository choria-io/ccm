// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package templates

import (
	"text/template"
)

func (e *Env) GoFunctions() template.FuncMap {
	return map[string]interface{}{
		"lookup":        e.lookup,
		"readFile":      e.goReadFile,
		"file":          e.goReadFile,
		"registrations": e.registrations,
	}
}

func (e *Env) goReadFile(file string) (string, error) {
	res, err := e.readFile(file)
	if err != nil {
		return "", err
	}

	return res.(string), nil
}
