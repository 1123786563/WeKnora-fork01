// Pass A compatibility alias for internal/modules/appconnector/connectorcontrol — zero logic. Deleted by Pass B task B-appconnector.
package connectorcontrol

import "github.com/Tencent/WeKnora/internal/modules/appconnector/connectorcontrol"

// Constants forwarded to the moved package.
const AdminDefaultTimeout = connectorcontrol.AdminDefaultTimeout
const AdminMaxResponseBytes = connectorcontrol.AdminMaxResponseBytes
const KindAuthorize = connectorcontrol.KindAuthorize
const KindConfirm = connectorcontrol.KindConfirm
const KindCreateToken = connectorcontrol.KindCreateToken
const KindDeleteConnection = connectorcontrol.KindDeleteConnection
const KindDeleteToken = connectorcontrol.KindDeleteToken

// Type aliases to the moved package.
type AdminClientConfig = connectorcontrol.AdminClientConfig
type AdminError = connectorcontrol.AdminError
type AdminSecret = connectorcontrol.AdminSecret
type AuthorizationStart = connectorcontrol.AuthorizationStart
type ConnectionRequestStatus = connectorcontrol.ConnectionRequestStatus
type ControlOperation = connectorcontrol.ControlOperation
type ControlStore = connectorcontrol.ControlStore
type ControlWorker = connectorcontrol.ControlWorker
type CreateTokenRequest = connectorcontrol.CreateTokenRequest
type EncryptedFileSecretSink = connectorcontrol.EncryptedFileSecretSink
type FileSecretSink = connectorcontrol.FileSecretSink
type OutboxStore = connectorcontrol.OutboxStore
type RuntimeAdmin = connectorcontrol.RuntimeAdmin
type RuntimeAdminClient = connectorcontrol.RuntimeAdminClient
type RuntimeConnection = connectorcontrol.RuntimeConnection
type RuntimeCorrelator = connectorcontrol.RuntimeCorrelator
type RuntimeTokenCreated = connectorcontrol.RuntimeTokenCreated
type SecretKeySource = connectorcontrol.SecretKeySource
type SecretSink = connectorcontrol.SecretSink
type WorkerConfig = connectorcontrol.WorkerConfig

// Variable and function forwarding to the moved package.
var DefaultInternalAllowlist = connectorcontrol.DefaultInternalAllowlist
var DefaultWorkerConfig = connectorcontrol.DefaultWorkerConfig
var ErrAdminResponseTooLarge = connectorcontrol.ErrAdminResponseTooLarge
var ErrAdminSecretRequired = connectorcontrol.ErrAdminSecretRequired
var ErrAdminSecretUnavailable = connectorcontrol.ErrAdminSecretUnavailable
var ErrCorrelationUnavailable = connectorcontrol.ErrCorrelationUnavailable
var ErrInvalidControlOperation = connectorcontrol.ErrInvalidControlOperation
var ErrInvalidRuntimeAddress = connectorcontrol.ErrInvalidRuntimeAddress
var ErrKindNotWiredYet = connectorcontrol.ErrKindNotWiredYet
var ErrMalformedAdminResponse = connectorcontrol.ErrMalformedAdminResponse
var ErrRuntimeAddressNotAllowed = connectorcontrol.ErrRuntimeAddressNotAllowed
var ErrSecretKeyUnavailable = connectorcontrol.ErrSecretKeyUnavailable
var ErrSecretNotEncrypted = connectorcontrol.ErrSecretNotEncrypted
var ErrUnpinnedEndpoint = connectorcontrol.ErrUnpinnedEndpoint
var ErrUnsupportedControlKind = connectorcontrol.ErrUnsupportedControlKind
var FileAdminSecretSource = connectorcontrol.FileAdminSecretSource
var FileSecretKeySource = connectorcontrol.FileSecretKeySource
var IsNotFound = connectorcontrol.IsNotFound
var NewControlWorker = connectorcontrol.NewControlWorker
var NewEncryptedFileSecretSink = connectorcontrol.NewEncryptedFileSecretSink
var NewFileSecretSink = connectorcontrol.NewFileSecretSink
var NewRuntimeAdminClient = connectorcontrol.NewRuntimeAdminClient
var OperationFromRow = connectorcontrol.OperationFromRow
var ParseAllowlist = connectorcontrol.ParseAllowlist
var ValidateControlOperation = connectorcontrol.ValidateControlOperation
