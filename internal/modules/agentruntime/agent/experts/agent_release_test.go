package experts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/types"
)

// The frozen snapshot read model: types.AgentVersionSnapshot is the same
// type Task 1 exposes as interfaces.AgentVersionSnapshot (a type alias), so
// this test constructs the exact value the service layer will pass in.

// releaseForbiddenMarkers are values AND JSON keys that must never appear in
// any serialized form of a Release bundle: KB bindings, model IDs,
// credential-shaped strings, the Sandbox config binding, Memory flags,
// Task/session transcript markers and the tenant-local config surface at
// large. Each one is populated in the source snapshot below, so the
// byte-level assertions prove the allow-list projection (not an accident of
// fixture construction) keeps them out.
var releaseForbiddenMarkers = []string{
	// Knowledge-base bindings.
	"kb-release-77", "kb_selection_mode", "knowledge_bases",
	// Model IDs of every kind.
	"model-do-not-leak", "rerank-do-not-leak", "vlm-do-not-leak",
	"asr-do-not-leak", "qu-do-not-leak",
	"model_id", "rerank_model_id", "vlm_model_id", "asr_model_id",
	"query_understand_model_id",
	// Credential-shaped strings.
	"sk-live-do-not-leak-9f1e", "AKIAIOSFODNN7EXAMPLE", "bearer-do-not-leak-token",
	// Sandbox binding.
	"sbx-do-not-leak", "sandbox_config_id",
	// Memory flags.
	"memory_enabled",
	// Task/session transcript markers.
	"TASK-TRANSCRIPT-8837", "SESSION-LOG-42",
	// Other tenant-local bindings and workspace-scoped config.
	"mcp-do-not-leak", "mcp_services", "web_search_provider_id",
	"image_storage_provider", "context_template", "fallback_response",
	"intent_prompts", "tenant_id", "created_by", "creator_name",
}

// releasePortableMarkers are the portable fields that must SURVIVE into the
// serialized bundle bytes.
var releasePortableMarkers = []string{
	"You review contracts and flag risks.",
	"INTJ",
	"Be terse and direct.",
	"web_search", "data_schema",
	"pdf-extract", "doc-render",
	"web-researcher",
	"Review this NDA",
}

// releaseTestSnapshot builds one frozen AgentVersionSnapshot whose config
// carries every portable field AND every forbidden marker class. Task/session
// and credential markers ride non-portable prompt/config fields they would
// leak through if the whole CustomAgent were ever serialized.
func releaseTestSnapshot() types.AgentVersionSnapshot {
	memory := true
	return types.AgentVersionSnapshot{
		AgentVersionView: types.AgentVersionView{
			ID:            "b1e0c3d2-1111-4222-8333-444455556666",
			AgentID:       "0b9f6a1e-6c96-4a8e-b7a5-3f2d1c0a9b8d",
			VersionNumber: 2,
			SourceSHA256:  strings.Repeat("c3", 32),
			FrozenBy:      "user-7",
		},
		Agent: &types.CustomAgent{
			ID:       "0b9f6a1e-6c96-4a8e-b7a5-3f2d1c0a9b8d",
			TenantID: 7,
			Name:     "合同审查助手",
			Config: types.CustomAgentConfig{
				// ===== Portable allow-list =====
				AgentMode:           types.AgentModeSmartReasoning,
				SystemPrompt:        "You review contracts and flag risks.",
				PersonaMBTI:         "INTJ",
				PersonaStyle:        "Be terse and direct.",
				AllowedTools:        []string{"web_search", "data_schema"},
				SkillsSelectionMode: "selected",
				SelectedSkills:      []string{"pdf-extract", "doc-render"},
				Subagents:           []string{"web-researcher"},
				QuestionSuggestions: &types.QuestionSuggestionConfig{
					Starters: types.StarterSuggestionConfig{
						Enabled: true,
						Mode:    types.SuggestionModeCurated,
						Items:   []string{"Review this NDA"},
					},
				},

				// ===== Forbidden: must never reach the bundle =====
				KBSelectionMode:        "selected",
				KnowledgeBases:         []string{"kb-release-77"},
				ModelID:                "model-do-not-leak",
				RerankModelID:          "rerank-do-not-leak",
				VLMModelID:             "vlm-do-not-leak",
				ASRModelID:             "asr-do-not-leak",
				QueryUnderstandModelID: "qu-do-not-leak",
				SandboxConfigID:        "sbx-do-not-leak",
				MemoryEnabled:          &memory,
				MCPServices:            []string{"mcp-do-not-leak"},
				WebSearchProviderID:    "wsp-do-not-leak",
				ImageStorageProvider:   "minio-do-not-leak",
				ContextTemplate:        "Context for TASK-TRANSCRIPT-8837 with key sk-live-do-not-leak-9f1e",
				FallbackResponse:       "Fallback mentioning SESSION-LOG-42",
				RewritePromptSystem:    "Rewrite with bearer-do-not-leak-token",
				IntentPrompts:          map[string]string{"chitchat": "Secret AKIAIOSFODNN7EXAMPLE"},
			},
		},
	}
}

func releaseMetadata() types.ReleaseMetadata {
	return types.ReleaseMetadata{
		SemanticVersion:          "1.2.0",
		DisplayName:              "Contract Reviewer",
		Summary:                  "Reviews contracts and flags clause risks.",
		SupportedLanguages:       []string{"zh", "en"},
		UseCases:                 []string{"NDA review", "Clause risk scanning"},
		NonUseCases:              []string{"Binding legal advice"},
		CapabilityRequirements:   []string{"skill:pdf-extract", "tool:web_search"},
		DataCategories:           []string{"user_documents"},
		ExternalSideEffects:      []string{"web_search_queries"},
		MinimumWeKnoraCapability: "tenant-marketplace/2026-09",
		LicenseID:                "MIT",
		ChangeNotes:              "First tenant catalog release.",
	}
}

func releaseDependencyLock() types.DependencyLock {
	return types.DependencyLock{Dependencies: []types.AgentReleaseDependency{
		// Deliberately unsorted: the canonical lock must order entries.
		{Type: "subagent", ID: "web-researcher", Version: "1.4.2", Digest: strings.Repeat("b2", 32), LicenseID: "Apache-2.0"},
		{Type: "skill", ID: "pdf-extract", Version: "2.0.1", Digest: strings.Repeat("a1", 32), LicenseID: "MIT"},
	}}
}

// serializedBundleForms returns every serialized representation of the bundle:
// the canonical envelope bytes plus each document re-marshaled on its own, so
// the forbidden-marker assertion covers ALL serialized bundle bytes, not just
// the envelope.
func serializedBundleForms(t *testing.T, bundle types.AgentReleaseBundle) [][]byte {
	t.Helper()
	forms := [][]byte{bundle.Bytes}
	for _, doc := range []any{bundle.Manifest, bundle.Lock, bundle.Payload} {
		raw, err := json.Marshal(doc)
		require.NoError(t, err)
		forms = append(forms, raw)
	}
	return forms
}

func TestBuildAgentReleaseBundleSanitizesPortableProjection(t *testing.T) {
	bundle, err := BuildAgentReleaseBundle(releaseTestSnapshot(), releaseMetadata(), releaseDependencyLock())
	require.NoError(t, err)

	// The portable projection survives, typed first.
	require.Equal(t, types.AgentModeSmartReasoning, bundle.Payload.AgentMode)
	require.Equal(t, "You review contracts and flag risks.", bundle.Payload.SystemPrompt)
	require.Equal(t, "INTJ", bundle.Payload.PersonaMBTI)
	require.Equal(t, "Be terse and direct.", bundle.Payload.PersonaStyle)
	require.Equal(t, []string{"web_search", "data_schema"}, bundle.Payload.AllowedTools)
	require.Equal(t, []string{"pdf-extract", "doc-render"}, bundle.Payload.Skills)
	require.Equal(t, []string{"web-researcher"}, bundle.Payload.Subagents)
	require.Equal(t, []string{"Review this NDA"}, bundle.Payload.StarterPrompts)

	// The Manifest carries the author metadata verbatim plus the immutable
	// source-version binding.
	snapshot := releaseTestSnapshot()
	require.Equal(t, "1.2.0", bundle.Manifest.SemanticVersion)
	require.Equal(t, "Contract Reviewer", bundle.Manifest.DisplayName)
	require.Equal(t, "Reviews contracts and flags clause risks.", bundle.Manifest.Summary)
	require.Equal(t, []string{"zh", "en"}, bundle.Manifest.SupportedLanguages)
	require.Equal(t, []string{"NDA review", "Clause risk scanning"}, bundle.Manifest.UseCases)
	require.Equal(t, []string{"Binding legal advice"}, bundle.Manifest.NonUseCases)
	require.Equal(t, []string{"skill:pdf-extract", "tool:web_search"}, bundle.Manifest.CapabilityRequirements)
	require.Equal(t, []string{"user_documents"}, bundle.Manifest.DataCategories)
	require.Equal(t, []string{"web_search_queries"}, bundle.Manifest.ExternalSideEffects)
	require.Equal(t, "tenant-marketplace/2026-09", bundle.Manifest.MinimumWeKnoraCapability)
	require.Equal(t, "MIT", bundle.Manifest.LicenseID)
	require.Equal(t, "First tenant catalog release.", bundle.Manifest.ChangeNotes)
	require.Equal(t, snapshot.ID, bundle.Manifest.Source.AgentVersionID)
	require.Equal(t, snapshot.VersionNumber, bundle.Manifest.Source.VersionNumber)
	require.Equal(t, snapshot.SourceSHA256, bundle.Manifest.Source.SourceSHA256)

	// The lock is normalized canonically: sorted by (type, id).
	require.Equal(t, "skill", bundle.Lock.Dependencies[0].Type)
	require.Equal(t, "pdf-extract", bundle.Lock.Dependencies[0].ID)
	require.Equal(t, "subagent", bundle.Lock.Dependencies[1].Type)
	require.Equal(t, "web-researcher", bundle.Lock.Dependencies[1].ID)

	// Portable markers are present in the canonical bytes...
	for _, marker := range releasePortableMarkers {
		require.Contains(t, string(bundle.Bytes), marker, "portable field must survive serialization")
	}
	// ...and every forbidden marker is absent from EVERY serialized form.
	for _, form := range serializedBundleForms(t, bundle) {
		for _, marker := range releaseForbiddenMarkers {
			require.NotContains(t, string(form), marker,
				"forbidden tenant-local marker must never reach serialized bundle bytes")
		}
	}

	// The digest is the SHA-256 over exactly the canonical bytes.
	sum := sha256.Sum256(bundle.Bytes)
	require.Equal(t, hex.EncodeToString(sum[:]), bundle.SHA256)
	require.Len(t, bundle.SHA256, 64)
}

func TestBuildAgentReleaseBundleValidationRejections(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*types.ReleaseMetadata, *types.DependencyLock, *types.AgentVersionSnapshot)
		wantErr string
	}{
		{
			name: "missing license",
			mutate: func(m *types.ReleaseMetadata, _ *types.DependencyLock, _ *types.AgentVersionSnapshot) {
				m.LicenseID = ""
			},
			wantErr: "license",
		},
		{
			name: "unsupported language",
			mutate: func(m *types.ReleaseMetadata, _ *types.DependencyLock, _ *types.AgentVersionSnapshot) {
				m.SupportedLanguages = []string{"zh", "fr"}
			},
			wantErr: "language",
		},
		{
			name: "no supported language at all",
			mutate: func(m *types.ReleaseMetadata, _ *types.DependencyLock, _ *types.AgentVersionSnapshot) {
				m.SupportedLanguages = nil
			},
			wantErr: "language",
		},
		{
			name: "empty use cases",
			mutate: func(m *types.ReleaseMetadata, _ *types.DependencyLock, _ *types.AgentVersionSnapshot) {
				m.UseCases = nil
			},
			wantErr: "use case",
		},
		{
			name: "blank use cases only",
			mutate: func(m *types.ReleaseMetadata, _ *types.DependencyLock, _ *types.AgentVersionSnapshot) {
				m.UseCases = []string{"  ", ""}
			},
			wantErr: "use case",
		},
		{
			name: "absent minimum capability",
			mutate: func(m *types.ReleaseMetadata, _ *types.DependencyLock, _ *types.AgentVersionSnapshot) {
				m.MinimumWeKnoraCapability = "   "
			},
			wantErr: "capability",
		},
		{
			name: "dependency without stable version",
			mutate: func(_ *types.ReleaseMetadata, l *types.DependencyLock, _ *types.AgentVersionSnapshot) {
				l.Dependencies[0].Version = ""
			},
			wantErr: "version",
		},
		{
			name: "dependency with moving version",
			mutate: func(_ *types.ReleaseMetadata, l *types.DependencyLock, _ *types.AgentVersionSnapshot) {
				l.Dependencies[0].Version = "latest"
			},
			wantErr: "version",
		},
		{
			name: "dependency without digest",
			mutate: func(_ *types.ReleaseMetadata, l *types.DependencyLock, _ *types.AgentVersionSnapshot) {
				l.Dependencies[0].Digest = ""
			},
			wantErr: "digest",
		},
		{
			name: "dependency with malformed digest",
			mutate: func(_ *types.ReleaseMetadata, l *types.DependencyLock, _ *types.AgentVersionSnapshot) {
				l.Dependencies[0].Digest = "not-a-sha256-digest"
			},
			wantErr: "digest",
		},
		{
			name: "dependency without license",
			mutate: func(_ *types.ReleaseMetadata, l *types.DependencyLock, _ *types.AgentVersionSnapshot) {
				l.Dependencies[0].LicenseID = ""
			},
			wantErr: "license",
		},
		{
			name: "nil agent snapshot",
			mutate: func(_ *types.ReleaseMetadata, _ *types.DependencyLock, s *types.AgentVersionSnapshot) {
				s.Agent = nil
			},
			wantErr: "agent",
		},
		{
			name: "unsafe skill reference in the snapshot",
			mutate: func(_ *types.ReleaseMetadata, _ *types.DependencyLock, s *types.AgentVersionSnapshot) {
				s.Agent.Config.SelectedSkills = []string{"../evil"}
			},
			wantErr: "skill",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := releaseTestSnapshot()
			metadata := releaseMetadata()
			lock := releaseDependencyLock()
			tc.mutate(&metadata, &lock, &snapshot)

			bundle, err := BuildAgentReleaseBundle(snapshot, metadata, lock)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
			require.Empty(t, bundle.Bytes, "a rejected build must not produce canonical bytes")
			require.Empty(t, bundle.SHA256)
		})
	}
}

func TestBuildAgentReleaseBundleDigestStability(t *testing.T) {
	first, err := BuildAgentReleaseBundle(releaseTestSnapshot(), releaseMetadata(), releaseDependencyLock())
	require.NoError(t, err)

	// Identical snapshot + metadata + lock must produce byte-identical output.
	second, err := BuildAgentReleaseBundle(releaseTestSnapshot(), releaseMetadata(), releaseDependencyLock())
	require.NoError(t, err)
	require.Equal(t, first.Bytes, second.Bytes, "identical inputs must serialize to identical bytes")
	require.Equal(t, first.SHA256, second.SHA256)

	// Canonical ordering: lock entry order never changes the digest.
	reordered := types.DependencyLock{Dependencies: []types.AgentReleaseDependency{
		first.Lock.Dependencies[1],
		first.Lock.Dependencies[0],
	}}
	shuffled, err := BuildAgentReleaseBundle(releaseTestSnapshot(), releaseMetadata(), reordered)
	require.NoError(t, err)
	require.Equal(t, first.SHA256, shuffled.SHA256, "a reordered lock is the same lock")

	// Freeze bookkeeping (frozen-by, frozen-at) is not Release content.
	viewOnly := releaseTestSnapshot()
	viewOnly.FrozenBy = "someone-else"
	viewOnly.CreatedAt = viewOnly.CreatedAt.Add(72 * 1e9)
	viewOnlyBytes, err := BuildAgentReleaseBundle(viewOnly, releaseMetadata(), releaseDependencyLock())
	require.NoError(t, err)
	require.Equal(t, first.SHA256, viewOnlyBytes.SHA256, "digest covers Release content, not freeze bookkeeping")

	// Any change to portable behavior, lock, Manifest or license must alter
	// the digest.
	changedBehavior := releaseTestSnapshot()
	changedBehavior.Agent.Config.SystemPrompt = "You draft contracts from scratch."
	behavior, err := BuildAgentReleaseBundle(changedBehavior, releaseMetadata(), releaseDependencyLock())
	require.NoError(t, err)
	require.NotEqual(t, first.SHA256, behavior.SHA256, "portable behavior change must alter the digest")

	changedLock := releaseDependencyLock()
	changedLock.Dependencies[0].Version = "2.0.2"
	lockChanged, err := BuildAgentReleaseBundle(releaseTestSnapshot(), releaseMetadata(), changedLock)
	require.NoError(t, err)
	require.NotEqual(t, first.SHA256, lockChanged.SHA256, "lock change must alter the digest")

	changedManifest := releaseMetadata()
	changedManifest.ChangeNotes = "Second release notes."
	manifestChanged, err := BuildAgentReleaseBundle(releaseTestSnapshot(), changedManifest, releaseDependencyLock())
	require.NoError(t, err)
	require.NotEqual(t, first.SHA256, manifestChanged.SHA256, "Manifest change must alter the digest")

	changedLicense := releaseMetadata()
	changedLicense.LicenseID = "Apache-2.0"
	licenseChanged, err := BuildAgentReleaseBundle(releaseTestSnapshot(), changedLicense, releaseDependencyLock())
	require.NoError(t, err)
	require.NotEqual(t, first.SHA256, licenseChanged.SHA256, "license change must alter the digest")
}
