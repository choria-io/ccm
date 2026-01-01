// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package posix

import (
	"fmt"
	"os"
)

func getFileOwner(stat os.FileInfo) (owner string, group string, mode string, err error) {
	return "", "", "", fmt.Errorf("not supported on windows")
}
