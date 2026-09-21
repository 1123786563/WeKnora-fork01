// W34 managed-workbench capability gate: the single switch evaluation
// consulted by AdmissionCoordinator.Start before identity, budget
// reservation or any durable write. Closing a lane rejects NEW work in that
// lane only; already-admitted runs finish and read/cleanup paths stay
// available (one switch never cuts query and cleanup at the same time).
package workbench

import (
	"errors"

	"github.com/Tencent/WeKnora/internal/config"
)

var (
	// ErrAdmissionDraining: worker drain is active — every admission lane is
	// closed to NEW work while existing runs drain to completion.
	ErrAdmissionDraining = errors.New("workbench worker drain refuses new admissions")
	// ErrPlatformAdmissionClosed: the platform lane's rollback switch
	// (workbench.platform_admission) is off; other lanes stay open.
	ErrPlatformAdmissionClosed = errors.New("workbench platform admission is closed")
)

// NewWorkbenchCapabilityGate builds the admission gate from the W34
// workbench capability switches. A nil or unset config keeps every lane
// open (safe-on defaults). The gate is target-scoped: drain closes all
// targets, the platform switch closes only the platform target.
//
// Remote (Paseo) targets have no production admission entrypoint on this
// branch yet (W22+ lands the remote submit path); when that entrypoint
// arrives it must install the same gate so paseo_admission /
// container.WorkbenchPaseoAdmissionEnabled take effect there.
func NewWorkbenchCapabilityGate(cfg *config.Config) func(targetID string) error {
	return func(targetID string) error {
		if cfg.IsWorkbenchWorkerDraining() {
			return ErrAdmissionDraining
		}
		if targetID == "platform" && !cfg.IsWorkbenchPlatformAdmissionEnabled() {
			return ErrPlatformAdmissionClosed
		}
		return nil
	}
}
