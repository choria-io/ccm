// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"github.com/choria-io/ccm/session"
)

// Option is a functional option for configuring CCM
type Option func(*CCM) error

// WithSessionDirectory sets the session store to use
func WithSessionDirectory(path string) Option {
	return func(c *CCM) error {
		log, err := c.Logger("session", "directory", "path", path)
		if err != nil {
			return err
		}

		sess, err := session.NewDirectorySessionStore(path, log, c.userLogger)
		if err != nil {
			return err
		}

		c.session = sess

		return nil
	}
}

// WithEnvironmentData sets environment data
func WithEnvironmentData(data map[string]string) Option {
	return func(c *CCM) error {
		if data == nil {
			data = make(map[string]string)
		}

		c.SetEnviron(data)

		return nil
	}
}
