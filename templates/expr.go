// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package templates

import (
	"fmt"

	"github.com/expr-lang/expr"
)

func ExprParse(query string, env *Env, opts ...expr.Option) (any, error) {
	o := []expr.Option{
		expr.Env(env),
		expr.Function("lookup", env.lookup),
		expr.Function("readFile", env.readFile),
		expr.Function("file", env.readFile),
		expr.Function("template", env.template),
		expr.Function("jet", env.jet),
		expr.Function("registrations", env.registrations),
	}
	o = append(o, opts...)

	program, err := expr.Compile(query, o...)
	if err != nil {
		return "", fmt.Errorf("expr compile error for '%s': %w", query, err)
	}

	return expr.Run(program, env)
}
