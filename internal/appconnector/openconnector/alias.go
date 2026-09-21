// Pass A compatibility alias for internal/modules/appconnector/openconnector — zero logic. Deleted by Pass B task B-appconnector.
package openconnector

import "github.com/Tencent/WeKnora/internal/modules/appconnector/openconnector"

// Constants forwarded to the moved package.
const DefaultTimeout = openconnector.DefaultTimeout
const MaxResponseBytes = openconnector.MaxResponseBytes

// Type aliases to the moved package.
type Call = openconnector.Call
type Client = openconnector.Client
type Executor = openconnector.Executor
type Grant = openconnector.Grant
type Result = openconnector.Result

// Variable and function forwarding to the moved package.
var ErrAliasRequired = openconnector.ErrAliasRequired
var ErrEmptyGrant = openconnector.ErrEmptyGrant
var ErrInvalidActionGrant = openconnector.ErrInvalidActionGrant
var ErrInvalidActionID = openconnector.ErrInvalidActionID
var ErrInvalidBaseURL = openconnector.ErrInvalidBaseURL
var ErrInvalidInput = openconnector.ErrInvalidInput
var ErrInvalidKey = openconnector.ErrInvalidKey
var ErrMalformedEnvelope = openconnector.ErrMalformedEnvelope
var ErrResponseTooLarge = openconnector.ErrResponseTooLarge
var ErrTokenRequired = openconnector.ErrTokenRequired
var NewClient = openconnector.NewClient
var NewGrant = openconnector.NewGrant
