// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package util

import (
	"fmt"
	"os"
)

// GetFileOwner returns the owner, group, and mode of a file from its FileInfo.
// This is not supported on Windows.
func GetFileOwner(stat os.FileInfo) (owner string, group string, mode string, err error) {
	return "", "", "", fmt.Errorf("not supported on windows")
}
