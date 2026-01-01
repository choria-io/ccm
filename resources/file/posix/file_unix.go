// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package posix

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"
)

func getFileOwner(stat os.FileInfo) (owner string, group string, mode string, err error) {
	sys := stat.Sys()
	ssys, ok := sys.(*syscall.Stat_t)
	if !ok {
		return "", "", "", fmt.Errorf("could not get platform stat information")
	}

	group = strconv.Itoa(int(ssys.Gid))
	owner = strconv.Itoa(int(ssys.Uid))

	grp, err := user.LookupGroupId(group)
	if err == nil {
		group = grp.Name
	}

	usr, err := user.LookupId(owner)
	if err == nil {
		owner = usr.Username
	}

	mode = fmt.Sprintf("%04o", stat.Mode()&os.ModePerm)

	return owner, group, mode, nil
}
