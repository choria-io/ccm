// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package templates

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/CloudyKit/jet/v6"
)

func (e *Env) JetVariables() jet.VarMap {
	return jet.VarMap{
		"facts":   reflect.ValueOf(e.Facts),
		"Facts":   reflect.ValueOf(e.Facts),
		"data":    reflect.ValueOf(e.Data),
		"Data":    reflect.ValueOf(e.Data),
		"environ": reflect.ValueOf(e.Environ),
		"Environ": reflect.ValueOf(e.Environ),
	}
}

func (e *Env) JetFunctions() map[string]jet.Func {
	return map[string]jet.Func{
		"lookup":        e.jetLookup(),
		"readFile":      e.jetReadFile(),
		"file":          e.jetReadFile(),
		"registrations": e.jetRegistrations(),
	}
}

func (e *Env) jet(params ...any) (any, error) {
	lpat := "[["
	rpat := "]]"
	body := ""
	var ok bool

	switch len(params) {
	case 1:
		body, ok = params[0].(string)
		if !ok {
			return "", fmt.Errorf("jet requires a string argument for template body")
		}

	case 3:
		body, ok = params[0].(string)
		if !ok {
			return "", fmt.Errorf("jet requires a string argument for template body")
		}

		lpat, ok = params[1].(string)
		if !ok {
			return "", fmt.Errorf("jet requires a string argument for left delimiter")
		}

		rpat, ok = params[2].(string)
		if !ok {
			return "", fmt.Errorf("jet requires a string argument for right delimiter")
		}
	default:
		return "", fmt.Errorf("jet requires 1 or 3 arguments")
	}

	if strings.HasSuffix(body, ".jet") {
		f, err := e.readFile(body)
		if err != nil {
			return nil, err
		}
		body = f.(string)
	}

	set := jet.NewSet(jet.NewInMemLoader(), jet.WithDelims(lpat, rpat), jet.WithSafeWriter(func(w io.Writer, b []byte) {
		w.Write(b)
	}))
	tpl, err := set.Parse("input", body)
	if err != nil {
		return nil, err
	}

	for k, v := range e.JetFunctions() {
		set.AddGlobalFunc(k, v)
	}

	buff := bytes.NewBuffer([]byte{})
	err = tpl.Execute(buff, e.JetVariables(), e)
	if err != nil {
		return nil, err
	}

	return buff.String(), nil
}

func (e *Env) jetLookup() jet.Func {
	return func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("lookup", 1, 2)

		var key string
		if err := a.ParseInto(&key); err != nil {
			a.Panicf("lookup: first argument must be a string: %v", err)
		}

		var defaultValue any
		if a.NumOfArguments() == 2 {
			defaultValue = a.Get(1).Interface()
		} else {
			defaultValue = nil
		}

		val, err := e.lookup(key, defaultValue)
		if err != nil {
			a.Panicf("lookup: failed: %v", err)
		}

		return reflect.ValueOf(val)
	}
}

func (e *Env) jetReadFile() jet.Func {
	return func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("file", 1, 1)

		var file string
		if err := a.ParseInto(&file); err != nil {
			a.Panicf("file: argument must be a string: %v", err)
		}

		val, err := e.readFile(file)
		if err != nil {
			a.Panicf("file: %v", err)
		}

		return reflect.ValueOf(val)
	}
}

func (e *Env) jetRegistrations() jet.Func {
	return func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("registrations", 4, 4)

		var cluster, protocol, service, ip string
		if err := a.ParseInto(&cluster, &protocol, &service, &ip); err != nil {
			a.Panicf("registrations: arguments must be strings: %v", err)
		}

		val, err := e.registrations(cluster, protocol, service, ip)
		if err != nil {
			a.Panicf("registrations: %v", err)
		}

		return reflect.ValueOf(val)
	}
}
