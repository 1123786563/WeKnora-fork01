// Pass A compatibility alias for internal/modules/policy/access — zero logic. Deleted by Pass B task B-policy.
package access

import "github.com/Tencent/WeKnora/internal/modules/policy/access"

// Type aliases to the moved package.
type AgentShareLookup = access.AgentShareLookup
type FileAccess = access.FileAccess
type FileCatalog = access.FileCatalog
type KBAccess = access.KBAccess
type KBPermissions = access.KBPermissions
type KBRequest = access.KBRequest
type KBShareLookup = access.KBShareLookup
type KBSharePermissionGuard = access.KBSharePermissionGuard
type KBSharePermissions = access.KBSharePermissions
type KBTenantLookup = access.KBTenantLookup
type KBTransferOperation = access.KBTransferOperation
type KnowledgeOwnerLookup = access.KnowledgeOwnerLookup
type MessageFileLookup = access.MessageFileLookup
type MessageKBShareAuthorizer = access.MessageKBShareAuthorizer
type OwnershipDecision = access.OwnershipDecision
type OwnershipRequest = access.OwnershipRequest
type SharedAgentFileLookup = access.SharedAgentFileLookup
type SharedAgentLookup = access.SharedAgentLookup

// Constants forwarded to the moved package.
const KBTransferClone = access.KBTransferClone
const KBTransferMove = access.KBTransferMove

// Variables and functions forwarded to the moved package.
var AuthorizeMessageFile = access.AuthorizeMessageFile
var CheckOwnershipOrRole = access.CheckOwnershipOrRole
var CloneDestination = access.CloneDestination
var HasKBGrant = access.HasKBGrant
var MessageReferencesFile = access.MessageReferencesFile
var NewKBPermissions = access.NewKBPermissions
var NewKBSharePermissions = access.NewKBSharePermissions
var RejectMovingKnowledge = access.RejectMovingKnowledge
var RequireKBTransfer = access.RequireKBTransfer
var RequireKBWrite = access.RequireKBWrite
var ResolveKB = access.ResolveKB
var ResolveKBFile = access.ResolveKBFile
var ResolveMessageArtifact = access.ResolveMessageArtifact
var ResolveMessageFile = access.ResolveMessageFile
var TransferTaskID = access.TransferTaskID
var ValidateKBTransferCompatibility = access.ValidateKBTransferCompatibility
var WithKBTaskWrite = access.WithKBTaskWrite
var WithKBTransfer = access.WithKBTransfer
var WithKBTransferTask = access.WithKBTransferTask
var WithSharedAgent = access.WithSharedAgent
var ErrForbidden = access.ErrForbidden
var ErrInvalidAgentSource = access.ErrInvalidAgentSource
var ErrNotFound = access.ErrNotFound
var ErrOwnershipForbidden = access.ErrOwnershipForbidden
var ErrResourceNotFound = access.ErrResourceNotFound
var ErrUnauthorized = access.ErrUnauthorized
