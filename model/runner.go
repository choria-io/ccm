// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
)

type CommandRunner interface {
	Execute(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error)
}
