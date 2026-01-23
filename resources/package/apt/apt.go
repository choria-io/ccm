// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package apt

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/choria-io/ccm/model"
)

const ProviderName = "apt"

// Provider manages packages using the APT package manager
type Provider struct {
	log    model.Logger
	runner model.CommandRunner
}

// NewAptProvider creates a new APT package provider
func NewAptProvider(log model.Logger, runner model.CommandRunner) (*Provider, error) {
	return &Provider{log: log, runner: runner}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return ProviderName
}

// We ensure that any user of this provider in the same process will not call apt multiple times
func (p *Provider) execute(ctx context.Context, cmd string, args ...string) (stdout []byte, stderr []byte, exitCode int, err error) {
	model.PackageGlobalLock.Lock()
	defer model.PackageGlobalLock.Unlock()

	return p.runner.ExecuteWithOptions(ctx, model.ExtendedExecOptions{
		Command: cmd,
		Args:    args,
		Environment: []string{
			"DEBIAN_FRONTEND=noninteractive",
			"APT_LISTBUGS_FRONTEND=none",
			"APT_LISTCHANGES_FRONTEND=none",
		},
	})
}

func (p *Provider) Install(ctx context.Context, pkg string, version string) error {
	var err error
	pkgVersion := ""
	args := []string{"install", "-y", "-q", "-o", "DPkg::Options::=--force-confold"}

	switch version {
	case model.PackageEnsureLatest:
		version, err := p.latestAvailable(ctx, pkg)
		if err != nil {
			return err
		}
		p.log.Debug("Found latest candidate", "candidate", version)
		pkgVersion = fmt.Sprintf("%s=%s", pkg, version)

	case model.EnsurePresent:
		pkgVersion = pkg
	default:
		pkgVersion = fmt.Sprintf("%s=%s", pkg, version)
		args = append(args, "--allow-downgrades")
	}
	args = append(args, pkgVersion)

	_, _, exitcode, err := p.execute(ctx, "apt-get", args...)
	if err != nil {
		return err
	}

	if exitcode != 0 {
		return fmt.Errorf("failed to install package %q, apt-get exited %d", pkg, exitcode)
	}

	return nil
}

func (p *Provider) Upgrade(ctx context.Context, pkg string, version string) error {
	return p.Install(ctx, pkg, version)
}

func (p *Provider) Downgrade(ctx context.Context, pkg string, version string) error {
	return p.Install(ctx, pkg, version)
}

func (p *Provider) Uninstall(ctx context.Context, pkg string) error {
	_, stderr, exitcode, err := p.execute(ctx, "apt-get", "-q", "-y", "remove", pkg)
	if err != nil {
		return fmt.Errorf("failed to uninstall %s: %w", pkg, err)
	}

	if exitcode != 0 {
		return fmt.Errorf("failed to uninstall %s: %s", pkg, stderr)
	}

	return nil
}

func (p *Provider) Status(ctx context.Context, pkg string) (*model.PackageState, error) {
	stdout, _, exitcode, err := p.execute(ctx, "dpkg-query", "-W", "-f=${Package} ${Version} ${Architecture} ${db:Status-Status}", pkg)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(strings.TrimSpace(string(stdout)), " ")
	installed := false
	status := "unknown"
	if len(parts) == 4 {
		status = parts[3]
		installed = status == "installed"
	}

	if exitcode != 0 || !installed {
		return &model.PackageState{
			CommonResourceState: model.NewCommonResourceState(model.ResourceStatusPackageProtocol, model.PackageTypeName, pkg, model.EnsureAbsent),
			Metadata: &model.PackageMetadata{
				Name:     pkg,
				Provider: ProviderName,
				Version:  "absent",
				Extended: map[string]any{
					"status": status,
				},
			},
		}, nil
	}

	if len(parts) != 4 {
		return nil, fmt.Errorf("failed to parse dpkg-query output for %s", pkg)
	}

	state := &model.PackageState{
		CommonResourceState: model.NewCommonResourceState(model.ResourceStatusPackageProtocol, model.PackageTypeName, pkg, parts[1]),
		Metadata: &model.PackageMetadata{
			Name:     parts[0],
			Version:  parts[1],
			Arch:     parts[2],
			Provider: ProviderName,
			Extended: map[string]any{
				"status": status,
			},
		},
	}

	return state, nil
}

func (p *Provider) VersionCmp(versionA, versionB string, ignoreTrailingZeroes bool) (int, error) {
	return CompareVersionStrings(versionA, versionB)
}

func (p *Provider) latestAvailable(ctx context.Context, pkg string) (string, error) {
	stdout, _, _, err := p.execute(ctx, "apt-cache", "policy", pkg)
	if err != nil {
		return "", err
	}

	return parseLatestAvailable(string(stdout), pkg)
}

func parseLatestAvailable(output string, pkg string) (string, error) {
	s := bufio.NewScanner(strings.NewReader(output))
	for s.Scan() {
		t := strings.TrimSpace(s.Text())
		version, found := strings.CutPrefix(t, "Candidate:")
		if found {
			return strings.TrimSpace(version), nil
		}
	}

	return "", fmt.Errorf("could not find Candidate: line in apt-cache policy output for %s", pkg)
}
