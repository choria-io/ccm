// Copyright (c) 2025-2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnf

import (
	"context"
	"fmt"
	"regexp"

	"github.com/choria-io/ccm/model"
)

const (
	dnfNevraQueryFormat = `%{NAME} %|EPOCH?{%{EPOCH}}:{0}| %{VERSION} %{RELEASE} %{ARCH}`
	dnfNevraRegex       = `^(\S+) (\S+) (\S+) (\S+) (\S+)$`
)

var (
	dnfNevraRe = regexp.MustCompile(dnfNevraRegex)
)

// DnfProvider manages packages using the DNF package manager
type DnfProvider struct {
	log    model.Logger
	runner model.CommandRunner
}

// NewDnfProvider creates a new DNF package provider
func NewDnfProvider(log model.Logger, runner model.CommandRunner) (*DnfProvider, error) {
	return &DnfProvider{log: log, runner: runner}, nil
}

// Install installs a package using DNF
func (p *DnfProvider) Install(ctx context.Context, pkg string, version string) error {
	var err error

	pkgVersion := ""

	switch version {
	case "latest", "present":
		pkgVersion = pkg
	default:
		pkgVersion = fmt.Sprintf("%s-%s", pkg, version)
	}

	_, _, exitcode, err := p.runner.Execute(ctx, "dnf", "install", "-y", pkgVersion)
	if err != nil {
		return err
	}

	if exitcode != 0 {
		return fmt.Errorf("failed to Install package %q", pkg)
	}

	return nil
}

// Upgrade upgrades a package to a specific version or latest using DNF
func (p *DnfProvider) Upgrade(ctx context.Context, pkg string, version string) error {
	return p.Install(ctx, pkg, version)
}

// Downgrade downgrades a package to a specific version using DNF
func (p *DnfProvider) Downgrade(ctx context.Context, pkg string, version string) error {
	_, _, exitcode, err := p.runner.Execute(ctx, "dnf", "downgrade", "-y", fmt.Sprintf("%s-%s", pkg, version))
	if err != nil {
		return err
	}

	if exitcode != 0 {
		return fmt.Errorf("failed to Downgrade %s", pkg)
	}

	return nil

}

// Uninstall removes a package using DNF
func (p *DnfProvider) Uninstall(ctx context.Context, pkg string) error {
	_, _, exitcode, err := p.runner.Execute(ctx, "dnf", "remove", "-y", pkg)
	if err != nil {
		return err
	}

	if exitcode != 0 {
		return fmt.Errorf("failed to Uninstall %s", pkg)
	}

	return nil
}

// Status returns the current installation status of a package
func (p *DnfProvider) Status(ctx context.Context, pkg string) (*model.PackageState, error) {
	stdout, _, exitcode, err := p.runner.Execute(ctx, "rpm", "-q", pkg, "--queryformat", dnfNevraQueryFormat)
	if err != nil {
		return nil, err
	}

	switch {
	case exitcode == 0:
		matches := dnfNevraRe.FindStringSubmatch(string(stdout))

		state := &model.PackageState{
			CommonResourceState: model.NewCommonResourceState(model.ResourceStatusPackageProtocol, "package", pkg, matches[3]),
			Metadata: &model.PackageMetadata{
				Name:     matches[1],
				Version:  fmt.Sprintf("%s-%s", matches[3], matches[4]),
				Arch:     matches[5],
				Provider: "dnf",
				Extended: map[string]any{
					"epoch":   matches[2],
					"release": matches[4],
				},
			},
		}

		return state, nil
	case exitcode > 0:
		return &model.PackageState{
			CommonResourceState: model.NewCommonResourceState(model.ResourceStatusPackageProtocol, "package", pkg, model.EnsureAbsent),
			Metadata: &model.PackageMetadata{
				Name:     pkg,
				Provider: "dnf",
				Version:  "absent",
				Extended: map[string]any{},
			},
		}, nil
	}

	return nil, nil
}
