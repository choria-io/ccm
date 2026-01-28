// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"fmt"
	"maps"
	"sort"
	"sync"

	"github.com/choria-io/ccm/model"
)

type providerEntry struct {
	factory model.ProviderFactory
}

var (
	providers = make(map[string]map[string]*providerEntry)
	mu        sync.Mutex
)

// Clear removes all registered providers
func Clear() {
	mu.Lock()
	defer mu.Unlock()

	providers = make(map[string]map[string]*providerEntry)
}

// Register registers a plugin
func Register(p any) error {
	switch tp := p.(type) {
	case model.ProviderFactory:
		return registerProvider(tp)
	default:
		return fmt.Errorf("cannot register provider of type %T", p)
	}
}

// MustRegister registers a plugin and panics if registration fails
func MustRegister(p any) {
	err := Register(p)
	if err != nil {
		panic(err)
	}
}

// mustRegisterProvider registers a provider factory and panics if registration fails
func mustRegisterProvider(p model.ProviderFactory) {
	err := registerProvider(p)
	if err != nil {
		panic(err)
	}
}

// registerProvider registers a provider factory for its type and returns an error if a provider with the same name already exists
func registerProvider(p model.ProviderFactory) error {
	mu.Lock()
	defer mu.Unlock()

	tn := p.TypeName()
	pn := p.Name()

	_, ok := providers[tn]
	if !ok {
		providers[tn] = make(map[string]*providerEntry)
	}

	_, ok = providers[tn][pn]
	if ok {
		return model.ErrDuplicateProvider
	}

	providers[tn][pn] = &providerEntry{factory: p}

	return nil
}

// selectProviders returns a list of providers that can manage the node given facts
func selectProviders(typeName string, facts map[string]any, properties model.ResourceProperties, log model.Logger) ([]model.ProviderFactory, error) {
	mu.Lock()
	defer mu.Unlock()

	var result []model.ProviderFactory

	typeEntries, ok := providers[typeName]
	if !ok {
		return result, nil
	}

	type matched struct {
		prio int
		prov model.ProviderFactory
	}

	var found []*matched

	for _, v := range typeEntries {
		ok, priority, err := v.factory.IsManageable(facts, properties)
		if err != nil {
			log.Warn("Could not check if provider is manageable", "provider", v.factory.Name(), "err", err)
			continue
		}

		if ok {
			found = append(found, &matched{priority, v.factory})
		}
	}

	sort.Slice(found, func(i, j int) bool { return found[i].prio < found[j].prio })
	for _, v := range found {
		result = append(result, v.prov)
	}

	return result, nil
}

// selectProvider finds a provider matching name and checks it's manageable before returning it
func selectProvider(typeName string, providerName string, facts map[string]any, properties model.ResourceProperties, log model.Logger) (model.ProviderFactory, error) {
	mu.Lock()
	defer mu.Unlock()

	typeEntries, ok := providers[typeName]
	if !ok {
		log.Debug("No providers registered", "type", typeName)
		return nil, nil
	}

	p, ok := typeEntries[providerName]
	if !ok {
		log.Debug("No providers found", "type", typeName, "provider", providerName)
		return nil, model.ErrProviderNotFound
	}

	ok, _, err := p.factory.IsManageable(facts, properties)
	if err != nil {
		log.Debug("Provider detection failed", "provider", p.factory.Name(), "err", err)
		return nil, fmt.Errorf("%w: %w", model.ErrProviderNotManageable, err)
	}

	if !ok {
		log.Debug("Provider cannot be used", "provider", p.factory.Name())
		return nil, fmt.Errorf("%w: %s", model.ErrProviderNotManageable, "not applicable to instance")
	}

	return p.factory, nil
}

// Types returns a list of all registered resource type names
func Types() []string {
	mu.Lock()
	defer mu.Unlock()

	var res []string
	for k := range maps.Keys(providers) {
		res = append(res, k)
	}

	sort.Strings(res)

	return res
}

// FindSuitableProvider searches all registered providers for a suitable provider capable of working on the node
func FindSuitableProvider(typeName string, provider string, facts map[string]any, properties model.ResourceProperties, log model.Logger, runner model.CommandRunner) (model.Provider, error) {
	var selected model.ProviderFactory

	if provider == "" {
		provs, err := selectProviders(typeName, facts, properties, log)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", model.ErrProviderNotFound, err)
		}

		if len(provs) == 0 {
			return nil, model.ErrNoSuitableProvider
		}

		selected = provs[0]
	} else {
		prov, err := selectProvider(typeName, provider, facts, properties, log)
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
