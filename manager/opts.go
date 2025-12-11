// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"fmt"

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

// WithExtraData sets extra data to be added to data set by user
func WithExtraData(key string, facts map[string]any) Option {
	return func(c *CCM) error {
		if key == "" {
			return fmt.Errorf("extra data key is required")
		}

		if facts == nil {
			facts = make(map[string]any)
		}

		c.extraData = facts
		c.extraDataKey = key

		return nil
	}
}
