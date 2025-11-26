// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"errors"
)

var (
	ErrResourceInvalid        = errors.New("invalid resource")
	ErrResourceNameRequired   = errors.New("name is required")
	ErrResourceEnsureRequired = errors.New("ensure is required")
	ErrProviderNotFound       = errors.New("provider not found")
	ErrProviderNotManageable  = errors.New("provider is not manageable")
	ErrMultipleProviders      = errors.New("multiple providers found")
	ErrNoSuitableProvider     = errors.New("no suitable provider found")
	ErrDuplicateProvider      = errors.New("provider already exists")
	ErrUnknownType            = errors.New("unknown resource type")
)
