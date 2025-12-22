// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"sync"
)

// PackageGlobalLock is used to ensure only one package resource is running at a time
// even when multiple managers are applying multiple manifests. This avoids issues
// with concurrent access to databases etc
var PackageGlobalLock = sync.Mutex{}

// ServiceGlobalLock is used to ensure only one service resource is running at a time
// even when multiple managers are applying multiple manifests.
var ServiceGlobalLock = sync.Mutex{}
