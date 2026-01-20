// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnf

import (
	"context"
	"fmt"
	"regexp"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
)

const (
	dnfNevraQueryFormat = `%{NAME} %|EPOCH?{%{EPOCH}}:{0}| %{VERSION} %{RELEASE} %{ARCH}`
	dnfNevraRegex       = `^(\S+) (\S+) (\S+) (\S+) (\S+)$`
)

var (
	dnfNevraRe = regexp.MustCompile(dnfNevraRegex)
)

const ProviderName = "dnf"

// Provider manages packages using the DNF package manager
type Provider struct {
	log    model.Logger
	runner model.CommandRunner
}

// NewDnfProvider creates a new DNF package provider
func NewDnfProvider(log model.Logger, runner model.CommandRunner) (*Provider, error) {
	return &Provider{log: log, runner: runner}, nil
}

// We ensure that any user of this provider in the same process will not call dnf multiple times
func (p *Provider) execute(ctx context.Context, cmd string, args ...string) (stdout []byte, stderr []byte, exitCode int, err error) {
	model.PackageGlobalLock.Lock()
	defer model.PackageGlobalLock.Unlock()

	return p.runner.Execute(ctx, cmd, args...)
}

// Name returns the provider name
func (p *Provider) Name() string {
	return ProviderName
}

// Install installs a package using DNF
func (p *Provider) Install(ctx context.Context, pkg string, version string) error {
	var err error

	pkgVersion := ""

	switch version {
	case model.PackageEnsureLatest, model.EnsurePresent:
		pkgVersion = pkg
	default:
		pkgVersion = fmt.Sprintf("%s-%s", pkg, version)
	}

	_, _, exitcode, err := p.execute(ctx, "dnf", "install", "-y", pkgVersion)
	if err != nil {
		return err
	}

	if exitcode != 0 {
		return fmt.Errorf("failed to Install package %q, dnf exited %d", pkg, exitcode)
	}

	return nil
}

// Upgrade upgrades a package to a specific version or latest using DNF
func (p *Provider) Upgrade(ctx context.Context, pkg string, version string) error {
	return p.Install(ctx, pkg, version)
}

// Downgrade downgrades a package to a specific version using DNF
func (p *Provider) Downgrade(ctx context.Context, pkg string, version string) error {
	_, _, exitcode, err := p.execute(ctx, "dnf", "downgrade", "-y", fmt.Sprintf("%s-%s", pkg, version))
	if err != nil {
		return err
	}

	if exitcode != 0 {
		return fmt.Errorf("failed to Downgrade %s, dnf exited %d", pkg, exitcode)
	}

	return nil

}

// Uninstall removes a package using DNF
func (p *Provider) Uninstall(ctx context.Context, pkg string) error {
	_, _, exitcode, err := p.execute(ctx, "dnf", "remove", "-y", pkg)
	if err != nil {
		return err
	}

	if exitcode != 0 {
		return fmt.Errorf("failed to Uninstall %s, dnf exited %d", pkg, exitcode)
	}

	return nil
}

// Status returns the current installation status of a package
func (p *Provider) Status(ctx context.Context, pkg string) (*model.PackageState, error) {
	stdout, _, exitcode, err := p.execute(ctx, "rpm", "-q", pkg, "--queryformat", dnfNevraQueryFormat)
	if err != nil {
		return nil, err
	}

	if exitcode != 0 {
		return &model.PackageState{
			CommonResourceState: model.NewCommonResourceState(model.ResourceStatusPackageProtocol, model.PackageTypeName, pkg, model.EnsureAbsent),
			Metadata: &model.PackageMetadata{
				Name:     pkg,
				Provider: ProviderName,
				Version:  "absent",
				Extended: map[string]any{},
			},
		}, nil
	}

	matches := dnfNevraRe.FindStringSubmatch(string(stdout))
	if len(matches) != 6 {
		return nil, fmt.Errorf("failed to parse rpm -q output for %s", pkg)
	}

	state := &model.PackageState{
		CommonResourceState: model.NewCommonResourceState(model.ResourceStatusPackageProtocol, "package", pkg, matches[3]),
		Metadata: &model.PackageMetadata{
			Name:     matches[1],
			Version:  fmt.Sprintf("%s-%s", matches[3], matches[4]),
			Arch:     matches[5],
			Provider: ProviderName,
			Extended: map[string]any{
				"epoch":   matches[2],
				"release": matches[4],
			},
		},
	}

	return state, nil
}

func (p *Provider) VersionCmp(versionA, versionB string, ignoreTrailingZeroes bool) (int, error) {
	return iu.VersionCmp(versionA, versionB, ignoreTrailingZeroes), nil
}
