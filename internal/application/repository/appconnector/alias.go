// Pass A compatibility alias for internal/modules/appconnector/repository/appconnector — zero logic. Deleted by Pass B task B-appconnector.
package appconnector

import appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"

// Constants forwarded to the moved package.

// Type aliases to the moved package.
type ActionRow = appconnectorrepo.ActionRow
type ActionStore = appconnectorrepo.ActionStore
type AppVersion = appconnectorrepo.AppVersion
type ApprovalRow = appconnectorrepo.ApprovalRow
type ConnectionRow = appconnectorrepo.ConnectionRow
type InstallationRow = appconnectorrepo.InstallationRow
type InstallationStore = appconnectorrepo.InstallationStore
type OCAuthorizationAttemptRow = appconnectorrepo.OCAuthorizationAttemptRow
type OCBindingRow = appconnectorrepo.OCBindingRow
type OCDefinitionRow = appconnectorrepo.OCDefinitionRow
type OCDispatchAlignment = appconnectorrepo.OCDispatchAlignment
type OCDispatchLeaseRow = appconnectorrepo.OCDispatchLeaseRow
type OCDispatchRecordRow = appconnectorrepo.OCDispatchRecordRow
type OCOperationsOutboxRow = appconnectorrepo.OCOperationsOutboxRow
type OCRuntimeRow = appconnectorrepo.OCRuntimeRow
type OCStore = appconnectorrepo.OCStore
type PreAuthorizationRow = appconnectorrepo.PreAuthorizationRow

// Variable and function forwarding to the moved package.
var ErrActionNotFound = appconnectorrepo.ErrActionNotFound
var ErrActionState = appconnectorrepo.ErrActionState
var ErrApprovalExhausted = appconnectorrepo.ErrApprovalExhausted
var ErrInstallationConflict = appconnectorrepo.ErrInstallationConflict
var ErrOCAttemptConflict = appconnectorrepo.ErrOCAttemptConflict
var ErrOCAttemptInvalid = appconnectorrepo.ErrOCAttemptInvalid
var ErrOCBindingConflict = appconnectorrepo.ErrOCBindingConflict
var ErrOCBindingInvalid = appconnectorrepo.ErrOCBindingInvalid
var ErrOCDefinitionConflict = appconnectorrepo.ErrOCDefinitionConflict
var ErrOCDefinitionInvalid = appconnectorrepo.ErrOCDefinitionInvalid
var ErrOCDispatchClaimed = appconnectorrepo.ErrOCDispatchClaimed
var ErrOCDispatchConflict = appconnectorrepo.ErrOCDispatchConflict
var ErrOCDispatchInvalid = appconnectorrepo.ErrOCDispatchInvalid
var ErrOCLeaseBusy = appconnectorrepo.ErrOCLeaseBusy
var ErrOCLeaseInvalid = appconnectorrepo.ErrOCLeaseInvalid
var ErrOCOperationInvalid = appconnectorrepo.ErrOCOperationInvalid
var ErrOCOutboxInvalid = appconnectorrepo.ErrOCOutboxInvalid
var ErrOCRevokeConflict = appconnectorrepo.ErrOCRevokeConflict
var ErrOCRevokeInvalid = appconnectorrepo.ErrOCRevokeInvalid
var ErrOCRuntimeUnavailable = appconnectorrepo.ErrOCRuntimeUnavailable
var ErrReauthorizationRequired = appconnectorrepo.ErrReauthorizationRequired
var NewActionStore = appconnectorrepo.NewActionStore
var NewInstallationStore = appconnectorrepo.NewInstallationStore
var NewOCKey = appconnectorrepo.NewOCKey
var NewOCStore = appconnectorrepo.NewOCStore
