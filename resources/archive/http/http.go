// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
)

const ProviderName = "http"

type Provider struct {
	log    model.Logger
	runner model.CommandRunner
}

func NewHttpProvider(log model.Logger, runner model.CommandRunner) (*Provider, error) {
	return &Provider{log: log, runner: runner}, nil
}

func (p *Provider) Download(ctx context.Context, properties *model.ArchiveResourceProperties, log model.Logger) error {
	uri, err := url.Parse(properties.Url)
	if err != nil {
		return err
	}

	if properties.Username != "" && properties.Password != "" {
		uri.User = url.UserPassword(properties.Username, properties.Password)
	}

	p.log.Info("Downloading", "url", iu.RedactUrlCredentials(uri))

	hdr := http.Header{}
	if properties.Headers != nil {
		for k, v := range properties.Headers {
			hdr.Add(k, v)
		}
	}

	resp, cancel, err := iu.HttpGetResponse(ctx, uri.String(), 0, hdr)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	defer cancel()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	parent := filepath.Dir(properties.Name)
	archiveName := filepath.Base(uri.Path)

	tf, err := os.CreateTemp(parent, fmt.Sprintf("%s-*", archiveName))
	if err != nil {
		return err
	}
	defer os.Remove(tf.Name())

	p.log.Info("Saving archive", "dest", properties.Name, "tf", tf.Name())

	// validate checks this is always set
	err = iu.ChownFile(tf, properties.Owner, properties.Group)
	if err != nil {
		tf.Close()
		return err
	}

	copied, err := io.Copy(tf, resp.Body)
	if err != nil {
		tf.Close()
		return fmt.Errorf("could not copy file: %w", err)
	}
	log.Info("Archive downloaded", "bytes", copied)

	err = tf.Close()
	if err != nil {
		return err
	}

	if properties.Checksum != "" {
		sum, err := iu.Sha256HashFile(tf.Name())
		if err != nil {
			return fmt.Errorf("could not checksum archive: %w", err)
		}
		if sum != properties.Checksum {
			return fmt.Errorf("checksum mismatch, expected %q got %q", properties.Checksum, sum)
		}
	}

	return os.Rename(tf.Name(), properties.Name)
}

func (p *Provider) Extract(ctx context.Context, properties *model.ArchiveResourceProperties, log model.Logger) error {
	// TODO: realistically this probably belong to the type else we end up with a ton of duplication, or perhaps a utility class or something

	if properties.ExtractParent == "" {
		return fmt.Errorf("extract parent not set")
	}

	if !iu.FileExists(properties.ExtractParent) {
		log.Info("Creating extract parent", "path", properties.ExtractParent)
		err := os.MkdirAll(properties.ExtractParent, 0755)
		if err != nil {
			return err
		}
	}

	switch {
	case iu.FileHasSuffix(properties.Name, ".tar.gz", ".tgz"):
		return p.extractTarGz(ctx, properties, log)
	case iu.FileHasSuffix(properties.Name, ".tar"):
		return p.extractTar(ctx, properties, log)
	case iu.FileHasSuffix(properties.Name, ".zip"):
		return p.extractZip(ctx, properties, log)
	default:
		return fmt.Errorf("archive type not supported")
	}
}

func toolForFileName(name string) string {
	switch {
	case iu.FileHasSuffix(name, ".zip"):
		return "unzip"
	case iu.FileHasSuffix(name, ".tar.gz", ".tgz", ".tar"):
		return "tar"
	default:
		return ""
	}
}

func (p *Provider) extractTarGz(ctx context.Context, properties *model.ArchiveResourceProperties, log model.Logger) error {
	_, stderr, exitCode, err := p.runner.ExecuteWithOptions(ctx, model.ExtendedExecOptions{
		Command: "tar",
		Args:    []string{"-xzf", properties.Name, "-C", properties.ExtractParent},
		Cwd:     properties.ExtractParent,
		Timeout: time.Minute,
	})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("tar exited with code %d: %s", exitCode, stderr)
	}

	return nil
}

func (p *Provider) extractTar(ctx context.Context, properties *model.ArchiveResourceProperties, log model.Logger) error {
	_, stderr, exitCode, err := p.runner.ExecuteWithOptions(ctx, model.ExtendedExecOptions{
		Command: "tar",
		Args:    []string{"-xf", properties.Name, "-C", properties.ExtractParent},
		Cwd:     properties.ExtractParent,
		Timeout: time.Minute,
	})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("tar exited with code %d: %s", exitCode, stderr)
	}

	return nil
}
func (p *Provider) extractZip(ctx context.Context, properties *model.ArchiveResourceProperties, log model.Logger) error {
	_, stderr, exitCode, err := p.runner.ExecuteWithOptions(ctx, model.ExtendedExecOptions{
		Command: "unzip",
		Args:    []string{"-d", properties.ExtractParent, properties.Name},
		Cwd:     properties.ExtractParent,
		Timeout: time.Minute,
	})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("unzip exited with code %d: %s", exitCode, stderr)
	}

	return nil
}

func (p *Provider) Status(ctx context.Context, properties *model.ArchiveResourceProperties) (*model.ArchiveState, error) {
	metadata := &model.ArchiveMetadata{
		Name:     properties.Name,
		Provider: ProviderName,
	}

	state := &model.ArchiveState{
		CommonResourceState: model.NewCommonResourceState(model.ResourceStatusArchiveProtocol, model.ArchiveTypeName, properties.Name, model.EnsureAbsent),
		Metadata:            metadata,
	}

	// Check if archive file exists
	stat, err := os.Stat(properties.Name)
	if err == nil {
		metadata.ArchiveExists = true
		metadata.Size = stat.Size()
		metadata.MTime = stat.ModTime()
		state.Ensure = model.EnsurePresent

		// Get owner and group
		owner, group, _, err := iu.GetFileOwner(stat)
		if err == nil {
			metadata.Owner = owner
			metadata.Group = group
		}

		// Calculate checksum
		checksum, err := iu.Sha256HashFile(properties.Name)
		if err == nil {
			metadata.Checksum = checksum
		}
	} else if errors.Is(err, os.ErrNotExist) {
		metadata.ArchiveExists = false
	}

	// Check if creates file exists
	if properties.Creates != "" {
		_, err := os.Stat(properties.Creates)
		if err == nil {
			metadata.CreatesExists = true
		}
	}

	return state, nil
}

func (p *Provider) Name() string {
	return ProviderName
}
