package datasource

import (
	"errors"
	"fmt"
	"strings"
)

// Error definitions for datasource operations
var (
	// Connector errors
	ErrConnectorNil       = errors.New("connector is nil")
	ErrConnectorTypeEmpty = errors.New("connector type is empty")
	ErrConnectorNotFound  = errors.New("connector type not found in registry")

	// DataSource errors
	ErrDataSourceNotFound  = errors.New("data source not found")
	ErrDataSourceInvalid   = errors.New("data source configuration is invalid")
	ErrDataSourceNotActive = errors.New("data source is not active")

	// Configuration errors
	ErrInvalidConfig      = errors.New("invalid configuration")
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrCredentialRefreshRejected is returned by the machine credential
	// write-back channel (SP2-b §6.2) when its anti-overwrite guard trips:
	// no usable credentials are stored (nothing configured, or the stored
	// blob no longer decrypts under the current SYSTEM_AES_KEY). Writing a
	// single refreshed key in that state would permanently overwrite the
	// surviving ciphertext with blank values, so the channel refuses.
	ErrCredentialRefreshRejected = errors.New("credential write-back rejected: no usable stored credentials")

	// Sync errors
	ErrSyncFailed       = errors.New("sync operation failed")
	ErrSyncCanceled     = errors.New("sync operation was canceled")
	ErrFetchFailed      = errors.New("failed to fetch items from source")
	ErrResourceNotFound = errors.New("resource not found in source system")

	// ErrItemNotFound is returned by TargetedFetcher.FetchByExternalID when the
	// requested external id no longer exists at the source (deleted, moved out
	// of the configured resources, or unfetchable), so the scoped-reindex path
	// can record a recognizable per-item failure instead of a generic one.
	ErrItemNotFound = errors.New("item not found in source system")

	// Knowledge base errors
	ErrKnowledgeBaseNotFound = errors.New("knowledge base not found")

	// Sync log errors
	ErrSyncLogNotFound = errors.New("sync log not found")
)

// PartialFetchError indicates that some resources were fetched successfully but
// others failed. The caller should process Items (if any), persist an updated
// cursor when provided, and surface Details to the user as a partial sync.
type PartialFetchError struct {
	Details []string
}

func (e *PartialFetchError) Error() string {
	if e == nil || len(e.Details) == 0 {
		return "partial fetch: some resources failed"
	}
	return fmt.Sprintf("partial fetch: %s", strings.Join(e.Details, "; "))
}
