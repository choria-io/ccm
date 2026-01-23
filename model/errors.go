// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
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
	ErrInvalidRequires        = errors.New("invalid require properties")
	ErrProviderNotFound       = errors.New("provider not found")
	ErrProviderNotManageable  = errors.New("provider is not manageable")
	ErrNoSuitableProvider     = errors.New("no suitable provider found")
	ErrDuplicateProvider      = errors.New("provider already exists")
	ErrUnknownType            = errors.New("unknown resource type")
	ErrDesiredStateFailed     = errors.New("failed to reach desired state")
	ErrInvalidEnsureValue     = errors.New("invalid ensure value")
	ErrInvalidState           = errors.New("invalid state encountered")
)
