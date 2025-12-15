// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package posix

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
)

const ProviderName = "posix"

type Provider struct {
	log model.Logger
}

func NewPosixProvider(log model.Logger) (*Provider, error) {
	return &Provider{log: log}, nil
}

func (p *Provider) Store(ctx context.Context, file string, contents []byte, source string, owner string, group string, mode string) error {
	dir := filepath.Dir(file)
	if !iu.IsDirectory(dir) {
		return fmt.Errorf("%q is not a directory", dir)
	}

	var parsedMode uint64
	parsedMode, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return err
	}

	var sf *os.File

	if source != "" {
		sf, err = os.Open(source)
		if err != nil {
			return err
		}
		defer sf.Close()
	}

	tf, err := os.CreateTemp(dir, fmt.Sprintf("%s.*", filepath.Base(file)))
	if err != nil {
		return err
	}
	defer tf.Close()
	defer os.Remove(tf.Name())
	err = tf.Chmod(os.FileMode(parsedMode))
	if err != nil {
		return err
	}

	if sf != nil {
		_, err = io.Copy(tf, sf)
	} else {
		_, err = tf.Write(contents)
	}
	if err != nil {
		return err
	}

	usrIDString, err := user.Lookup(owner)
	if err != nil {
		return fmt.Errorf("could not lookup user %q: %w", owner, err)
	}
	uid, err := strconv.Atoi(usrIDString.Uid)
	if err != nil {
		return fmt.Errorf("could not convert user id %s to integer: %w", usrIDString.Uid, err)
	}

	grpIDString, err := user.LookupGroup(group)
	if err != nil {
		return fmt.Errorf("could not lookup group %q: %w", group, err)
	}
	gid, err := strconv.Atoi(grpIDString.Gid)
	if err != nil {
		return fmt.Errorf("could not convert group id %s to integer: %w", grpIDString.Gid, err)
	}

	err = tf.Chown(uid, gid)
	if err != nil {
		return err
	}

	err = tf.Close()
	if err != nil {
		return fmt.Errorf("could not close temporary file: %w", err)
	}

	err = os.Rename(tf.Name(), file)
	if err != nil {
		return fmt.Errorf("could not rename temporary file: %w", err)
	}

	return nil

}

// Status returns the current installation status of a file
func (p *Provider) Status(ctx context.Context, file string) (*model.FileState, error) {
	state := &model.FileState{
		CommonResourceState: model.NewCommonResourceState(model.ResourceStatusFileProtocol, model.FileTypeName, file, model.EnsurePresent),
		Metadata: &model.FileMetadata{
			Name:     file,
			Provider: ProviderName,
			Extended: map[string]any{},
		},
	}

	stat, err := os.Stat(file)
	switch {
	case os.IsNotExist(err):
		state.Ensure = model.EnsureAbsent
	case os.IsPermission(err):
		p.log.Warn("Permission denied for file %v: %v", file, err)
		state.Ensure = model.EnsureAbsent
	case err == nil:
		var err error
		state.Metadata.Owner, state.Metadata.Group, state.Metadata.Mode, err = getFileOwner(stat)
		if err != nil {
			p.log.Warn("Failed to get file ownership information: %s", err)
		}

		state.Metadata.Checksum, err = iu.Sha256HashFile(file)
		if err != nil {
			p.log.Warn("Failed to calculate checksum: %s", err)
		}

	default:
		p.log.Warn("Failed to get file ownership information: %s", err)
	}

	return state, nil
}

func (p *Provider) Name() string {
	return ProviderName
}
