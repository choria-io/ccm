// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package posix

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

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

func (p *Provider) CreateDirectory(ctx context.Context, dir string, owner string, group string, mode string) error {
	parsedMode, err := parseFileMode(mode)
	if err != nil {
		return err
	}
	parsedMode = iu.DirectoryMode(parsedMode)

	// Check if path is a symlink - if so, remove it first to ensure
	// we create an actual directory rather than following the symlink
	lstat, err := os.Lstat(dir)
	if err == nil && lstat.Mode()&os.ModeSymlink != 0 {
		err := os.Remove(dir)
		if err != nil {
			return fmt.Errorf("could not remove symlink: %w", err)
		}
	}

	err = os.MkdirAll(dir, parsedMode)
	if err != nil {
		return err
	}

	uid, gid, err := iu.LookupOwnerGroup(owner, group)
	if err != nil {
		return err
	}

	err = os.Chmod(dir, parsedMode)
	if err != nil {
		return err
	}

	return os.Chown(dir, uid, gid)
}

func parseFileMode(mode string) (os.FileMode, error) {
	parsedMode, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return 0, err
	}

	return os.FileMode(parsedMode), nil
}

func (p *Provider) Store(ctx context.Context, file string, contents []byte, source string, owner string, group string, mode string) error {
	dir := filepath.Dir(file)
	if !iu.IsDirectory(dir) {
		return fmt.Errorf("%q is not a directory", dir)
	}

	parsedMode, err := parseFileMode(mode)
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
	err = tf.Chmod(parsedMode)
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

	err = iu.ChownFile(tf, owner, group)
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

// SetAttributes updates owner, group and mode on an existing regular file
// without touching its contents. Symlinks are rejected to avoid silently
// mutating the link target.
//
// chown runs before chmod because chown(2) clears setuid/setgid bits on
// Linux; doing chmod last preserves any setuid/setgid bits the caller
// requested in mode.
func (p *Provider) SetAttributes(ctx context.Context, file string, owner string, group string, mode string) error {
	parsedMode, err := parseFileMode(mode)
	if err != nil {
		return err
	}

	lstat, err := os.Lstat(file)
	if err != nil {
		return err
	}
	if lstat.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink; refusing to set attributes through it", file)
	}

	uid, gid, err := iu.LookupOwnerGroup(owner, group)
	if err != nil {
		return err
	}

	err = os.Chown(file, uid, gid)
	if err != nil {
		return err
	}

	return os.Chmod(file, parsedMode)
}

// Remove removes a file or directory.
//
// When force is true, non-empty directories are removed recursively via
// os.RemoveAll. RemoveAll does not follow symlinks during traversal; if the
// target itself is a symlink, only the symlink is removed and its target is
// left intact.
//
// When force is false, os.Remove is used, which errors on non-empty
// directories. In that case the error is wrapped with guidance to set
// force: true.
//
// A path that does not exist is treated as a no-op.
func (p *Provider) Remove(ctx context.Context, file string, force bool) error {
	if force {
		return os.RemoveAll(file)
	}

	err := os.Remove(file)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, os.ErrNotExist):
		return nil
	case errors.Is(err, syscall.ENOTEMPTY):
		return fmt.Errorf("cannot remove %s: directory is not empty, set 'force: true' to remove non-empty directories: %w", file, err)
	default:
		return err
	}
}

// Status returns the current installation status of a file
func (p *Provider) Status(ctx context.Context, file string) (*model.FileState, error) {
	metadata := &model.FileMetadata{
		Name:     file,
		Provider: ProviderName,
		Extended: map[string]any{},
	}

	state := &model.FileState{
		CommonResourceState: model.NewCommonResourceState(model.ResourceStatusFileProtocol, model.FileTypeName, file, model.EnsurePresent),
		Metadata:            metadata,
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

		metadata.Size = stat.Size()
		metadata.MTime = stat.ModTime()

		metadata.Owner, metadata.Group, metadata.Mode, err = iu.GetFileOwner(stat)
		if err != nil {
			p.log.Warn("Failed to get file ownership information: %s", err)
		}

		if stat.IsDir() {
			state.Ensure = model.FileEnsureDirectory
		} else {
			metadata.Checksum, err = iu.Sha256HashFile(file)
			if err != nil {
				p.log.Warn("Failed to calculate checksum: %s", err)
			}

			state.Ensure = model.EnsurePresent
		}
	default:
		p.log.Warn("Failed to get file ownership information: %s", err)
	}

	return state, nil
}

func (p *Provider) Name() string {
	return ProviderName
}
