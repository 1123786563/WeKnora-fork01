package publish

// ProviderProfile is the ONLY place a provider's plan-level identity
// lives (AC1: 飞书差异只存在于 Adapter). Everything the service layer
// cannot decide generically — the reviewed app id, the publication-row
// provider brand, the action version, the conflict-result prefix, the
// block projection, the approved-snapshot byte shapes, the plan-time
// version pre-read and the receipt projection — is a profile field
// backed by adapter-domain functions.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
)

type ProviderProfile struct {
	AppID         string // reviewed installation app id ("notion" / "feishu")
	Provider      string // app_publications.provider brand
	ActionVersion string // prepared action version tag
	// ConflictResultPrefix is the ProviderResult prefix the provider's
	// bridge records for a definitive version conflict; the handler maps
	// it onto 409.
	ConflictResultPrefix string
	// BlocksOf derives the provider's content blocks from artifact text.
	BlocksOf func(text string) ([]json.RawMessage, error)
	// CreateArgs / UpdateArgs marshal the approved snapshot bytes — the
	// field names ARE the provider's snapshot contract (approve-then-
	// rewrite protection parses exactly these).
	CreateArgs func(parent, title string, blocks []json.RawMessage) ([]byte, error)
	UpdateArgs func(destination, expectedVersion, title string, blocks []json.RawMessage) ([]byte, error)
	// ReadRemoteVersion is the plan-formation pre-read (AC1 baseline).
	ReadRemoteVersion func(ctx context.Context, connectionID, destination string) (string, error)
	// ParseReceipt projects the adapter's output evidence onto
	// (externalID, externalVersion) for the publication settle.
	ParseReceipt func(providerResult string) (externalID, externalVersion string, err error)
}

// NotionProfile is #48's profile, extracted verbatim.
func NotionProfile(remote NotionRemoteReader) ProviderProfile {
	return ProviderProfile{
		AppID: "notion", Provider: "notion", ActionVersion: "notion/v1",
		ConflictResultPrefix: PublishVersionConflictResult,
		BlocksOf:             NotionParagraphBlocks,
		CreateArgs: func(parent, title string, blocks []json.RawMessage) ([]byte, error) {
			return json.Marshal(map[string]any{"parent": parent, "title": title, "blocks": blocks})
		},
		UpdateArgs: func(destination, expectedVersion, title string, blocks []json.RawMessage) ([]byte, error) {
			return json.Marshal(map[string]any{"page_id": destination, "expected_version": expectedVersion, "title": title, "blocks": blocks})
		},
		ReadRemoteVersion: remote.ReadPageVersion,
		ParseReceipt: func(providerResult string) (string, string, error) {
			rcpt, err := appconn.ParseNotionPageReceipt([]byte(providerResult))
			return rcpt.ExternalID, rcpt.ExternalVersion, err
		},
	}
}

// FeishuProfile is #49's profile. The snapshot field names bind the
// FE-PUB-01 contract parsed by appconn.ParseFeishuDocCreate/UpdateSnapshot:
// create = exactly {parent_folder, title, blocks}; update = exactly
// {document_id, expected_revision, title, blocks}.
func FeishuProfile(read NotionRemoteReader) ProviderProfile {
	return ProviderProfile{
		AppID: "feishu", Provider: "feishu", ActionVersion: "feishu/v1",
		ConflictResultPrefix: FeishuVersionConflictResult,
		// B5-F78/F57: the adapter-domain block projection's sentinels are
		// mapped onto the NEUTRAL publish-package errors — the family the
		// handler's failPublish matches. Returning the adapter instances
		// verbatim left the handler's 400/413 branches unreachable
		// (errors.Is over different instances) and surfaced 500s.
		BlocksOf: func(text string) ([]json.RawMessage, error) {
			blocks, err := appconn.FeishuTextBlocks(text)
			if err != nil {
				if errors.Is(err, appconn.ErrFeishuPublishEmptyContent) {
					return nil, fmt.Errorf("%w: %v", ErrPublishEmptyContent, err)
				}
				if errors.Is(err, appconn.ErrFeishuPublishContentTooLarge) {
					return nil, fmt.Errorf("%w: %v", ErrPublishContentTooLarge, err)
				}
				return nil, err
			}
			return blocks, nil
		},
		CreateArgs: func(parent, title string, blocks []json.RawMessage) ([]byte, error) {
			return json.Marshal(map[string]any{"parent_folder": parent, "title": title, "blocks": blocks})
		},
		UpdateArgs: func(destination, expectedVersion, title string, blocks []json.RawMessage) ([]byte, error) {
			return json.Marshal(map[string]any{"document_id": destination, "expected_revision": expectedVersion, "title": title, "blocks": blocks})
		},
		ReadRemoteVersion: read.ReadPageVersion,
		ParseReceipt: func(providerResult string) (string, string, error) {
			rcpt, err := appconn.ParseFeishuDocReceipt([]byte(providerResult))
			return rcpt.ExternalID, rcpt.ExternalVersion, err
		},
	}
}

// errConfluenceVersionEmpty preserves the confluence plan face's fail-closed
// pre-read: an empty version string is a failure, never a recorded baseline
// (the shared create path tolerates empty baselines for providers whose
// destinations have no revision — feishu folders; a confluence page read
// must never record one).
var errConfluenceVersionEmpty = errors.New("confluence page version empty")

// ConfluenceProfile is #50's profile (#49 feishu precedent): the storage
// body projection, the snapshot byte shapes, the plan-time version pre-read
// and the receipt parser. The storage XHTML rides BlocksOf's slice as its
// single RawMessage — the shared service never inspects the carrier; only
// this profile's Create/UpdateArgs unwrap it into the "storage" field, so
// the approved snapshot bytes stay identical to the pre-collapse wire
// (digest compatibility).
func ConfluenceProfile(remote ConfluenceRemoteReader) ProviderProfile {
	return ProviderProfile{
		AppID: appIDConfluence, Provider: "confluence", ActionVersion: "confluence/v1",
		ConflictResultPrefix: ConfluenceVersionConflictResult,
		// ConfluenceStorageBody already speaks the neutral publish
		// sentinels (ErrPublishEmptyContent / ErrPublishContentTooLarge).
		BlocksOf: func(text string) ([]json.RawMessage, error) {
			storage, err := ConfluenceStorageBody(text)
			if err != nil {
				return nil, err
			}
			return []json.RawMessage{json.RawMessage(storage)}, nil
		},
		CreateArgs: func(parent, title string, blocks []json.RawMessage) ([]byte, error) {
			if len(blocks) == 0 {
				return nil, ErrPublishEmptyContent
			}
			return json.Marshal(map[string]any{"parent": parent, "title": title, "storage": string(blocks[0])})
		},
		UpdateArgs: func(destination, expectedVersion, title string, blocks []json.RawMessage) ([]byte, error) {
			if len(blocks) == 0 {
				return nil, ErrPublishEmptyContent
			}
			return json.Marshal(map[string]any{"page_id": destination, "expected_version": expectedVersion, "title": title, "storage": string(blocks[0])})
		},
		ReadRemoteVersion: func(ctx context.Context, connectionID, destination string) (string, error) {
			v, err := remote.ReadConfluencePageVersion(ctx, connectionID, destination)
			if err == nil && v == "" {
				return "", errConfluenceVersionEmpty
			}
			return v, err
		},
		ParseReceipt: func(providerResult string) (string, string, error) {
			rcpt, err := appconn.ParseConfluencePageReceipt([]byte(providerResult))
			return rcpt.ExternalID, rcpt.ExternalVersion, err
		},
	}
}
