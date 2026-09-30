//go:build darwin

package container

import "golang.org/x/sys/unix"

func renameRunViewInputNoReplace(dirFD int, tempName, targetName string) error {
	return unix.RenameatxNp(dirFD, tempName, dirFD, targetName, unix.RENAME_EXCL)
}
