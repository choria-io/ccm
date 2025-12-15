// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"fmt"
	"maps"
	"sync"

	"github.com/choria-io/ccm/model"
)

type entry struct {
	factory model.ProviderFactory
}

var (
	entries = make(map[string]map[string]*entry)
	mu      sync.Mutex
)

// Clear removes all registered providers
func Clear() {
	mu.Lock()
	defer mu.Unlock()

	entries = make(map[string]map[string]*entry)
}

// MustRegister registers a provider factory and panics if registration fails
func MustRegister(p model.ProviderFactory) {
	err := Register(p)
	if err != nil {
		panic(err)
	}
}

// Register registers a provider factory for its type and returns an error if a provider with the same name already exists
func Register(p model.ProviderFactory) error {
	mu.Lock()
	defer mu.Unlock()

	tn := p.TypeName()
	pn := p.Name()

	_, ok := entries[tn]
	if !ok {
		entries[tn] = make(map[string]*entry)
	}

	_, ok = entries[tn][pn]
	if ok {
		return model.ErrDuplicateProvider
	}

	entries[tn][pn] = &entry{factory: p}

	return nil
}

// SelectProviders returns a list of providers that can manage the node given facts
func SelectProviders(typeName string, facts map[string]any, log model.Logger) ([]model.ProviderFactory, error) {
	mu.Lock()
	defer mu.Unlock()

	var result []model.ProviderFactory

	typeEntries, ok := entries[typeName]
	if !ok {
		return result, nil
	}

	for _, v := range typeEntries {
		ok, err := v.factory.IsManageable(facts)
		if err != nil {
			log.Warn("Could not check if provider is manageable", "provider", v.factory.Name(), "err", err)
			continue
		}

		if ok {
			result = append(result, v.factory)
		}
	}

	return result, nil
}

// SelectProvider finds a provider matching name and checks it's manageable before returning it
func SelectProvider(typeName string, providerName string, facts map[string]any) (model.ProviderFactory, error) {
	mu.Lock()
	defer mu.Unlock()

	typeEntries, ok := entries[typeName]
	if !ok {
		return nil, nil
	}

	p, ok := typeEntries[providerName]
	if !ok {
		return nil, model.ErrProviderNotFound
	}

	ok, err := p.factory.IsManageable(facts)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", model.ErrProviderNotManageable, err)
	}

	if !ok {
		return nil, fmt.Errorf("%w: %s", model.ErrProviderNotManageable, "not applicable to instance")
	}

	return p.factory, nil
}

// Types returns a list of all registered resource type names
func Types() []string {
	mu.Lock()
	defer mu.Unlock()

	var res []string
	for k := range maps.Keys(entries) {
		res = append(res, k)
	}

	return res
}

func FindSuitableProvider(typeName string, provider string, facts map[string]any, log model.Logger, runner model.CommandRunner) (model.Provider, error) {
	var selected model.ProviderFactory

	if provider == "" {
		provs, err := SelectProviders(typeName, facts, log)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", model.ErrProviderNotFound, err)
		}

		if len(provs) == 0 {
			return nil, model.ErrNoSuitableProvider
		}

		if len(provs) != 1 {
			return nil, model.ErrMultipleProviders
		}

		selected = provs[0]
	} else {
		prov, err := SelectProvider(typeName, provider, facts)
		if err != nil && prov == nil {
			return nil, fmt.Errorf("%w: %w", model.ErrResourceInvalid, err)
		}

		selected = prov
	}

	if selected == nil {
		return nil, model.ErrNoSuitableProvider
	}

	return selected.New(log, runner)
}
