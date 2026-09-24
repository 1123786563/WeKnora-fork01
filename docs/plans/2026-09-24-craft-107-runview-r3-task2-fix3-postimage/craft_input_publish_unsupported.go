//go:build !linux && !darwin

package container

import "errors"

func renameRunViewInputNoReplace(_ int, _, _ string) error {
	return errors.New("RunView input no-replace publication is unsupported on this platform")
}
