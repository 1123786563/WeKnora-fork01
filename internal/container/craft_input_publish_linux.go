//go:build linux

package container

import "golang.org/x/sys/unix"

func renameRunViewInputNoReplace(dirFD int, tempName, targetName string) error {
	return unix.Renameat2(dirFD, tempName, dirFD, targetName, unix.RENAME_NOREPLACE)
}
