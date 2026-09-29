//go:build !darwin && !linux

package career

import (
	"context"
	"errors"
	"gorm.io/gorm"
)

var errLifecycleGuardBusy = errors.New("lifecycle execution guard is held")

func acquireLifecycleExecutionGuard(context.Context, *gorm.DB, Scope, string, string) (func(), error) {
	return nil, errors.New("cross-process lifecycle guard unsupported on this platform")
}
