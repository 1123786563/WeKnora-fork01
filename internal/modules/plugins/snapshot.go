package plugins

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

// BuildVerifiedSnapshot cross-checks the manifest's declared tool directory
// against the LIVE ListTools result and derives the authoritative snapshot.
//
// Verification is bidirectional and exhaustive — every disagreement is a
// rejection, never a silent acceptance:
//   - a declared tool missing from the live endpoint;
//   - a declared tool whose live schema digest differs from the declaration;
//   - a live tool the manifest never declared;
//   - a live tool name failing identifier hygiene (oversized or carrying
//     control/format characters) — live names are untrusted remote data and
//     are vetted with the same validateName rules as manifest names BEFORE
//     any of them is quoted into the rejection message;
//   - a live directory larger than maxLiveTools; a manifest declaring the
//     same tool name twice, declaring no tools at all, or declaring a
//     malformed (non-64-hex) input_schema_digest — all re-checked here
//     (ValidateManifest already rejects each of them, but this function is
//     exported and must not silently rely on the caller having validated
//     first; an unchecked empty digest would ""==""-match ToolSchemaDigest's
//     "" for an unparseable live schema and mint a snapshot whose digest is
//     not 64-hex); a declaration whose name fails the identifier hygiene
//     rules or whose scopes fail the RFC 6749 checks — re-checked for the
//     same reason: unchecked scopes would flow verbatim into the persisted
//     snapshot and the admin/member authorization surfaces.
//
// A name the endpoint serves several times (declared or not) is ONE
// discrepancy and is reported once — duplicate and undeclared echoes are
// both deduplicated.
//
// The returned error names EVERY differing tool — up to
// maxVerificationProblems entries, with the remainder collapsed into a
// counter, so a hostile endpoint cannot balloon the single joined error.
// Snapshot fields come from the manifest declaration (read_only /
// requires_personal_auth / scopes) except description and
// input_schema_digest, which are recomputed from the live endpoint — the
// live data is the authority.
func BuildVerifiedSnapshot(manifest *types.PluginManifest, live []*types.MCPTool) ([]types.PluginToolSnapshot, string, error) {
	if manifest == nil {
		return nil, "", fmt.Errorf("manifest is required")
	}
	// Re-check here instead of trusting the caller to have run
	// ValidateManifest: an empty declaration list would otherwise produce an
	// empty snapshot plus the well-formed digest of "[]" — silently breaking
	// the "a verified snapshot always carries at least the declared tools"
	// contract downstream installations rely on (OCR T01-R1-F4).
	if len(manifest.Tools) == 0 {
		return nil, "", fmt.Errorf("manifest must declare at least one tool")
	}
	// The live directory has no transport-level size bound (only the manifest
	// download is capped at 1MiB) — refuse to process an oversized directory
	// before any O(n) map/digest/snapshot work.
	if len(live) > maxLiveTools {
		return nil, "", fmt.Errorf("live endpoint returned %d tools, exceeding the maximum of %d", len(live), maxLiveTools)
	}
	var problems []string
	liveByName := make(map[string]*types.MCPTool, len(live))
	unvettedName := make(map[int]bool, len(live))
	// One duplicated live name is ONE contradiction — report it once, not
	// (N-1) times (OCR T01-R3-F3).
	reportedDuplicate := make(map[string]bool)
	// An unvetted name served N times is likewise ONE discrepancy: dedupe by
	// the name even though the per-position message text differs (the where
	// prefix carries the index) — the "reported once" contract at the top of
	// this function covers unvetted names too (OCR round-1 R12 F04).
	reportedUnvetted := make(map[string]bool)
	for i, tool := range live {
		if tool == nil {
			continue
		}
		// Vet the name up front, keyed by position (never by the untrusted
		// name itself): everything that reaches an error message below has
		// passed the same hygiene the manifest validator enforces.
		if err := validateToolName(fmt.Sprintf("live tools[%d].name", i), tool.Name, maxToolNameLen); err != nil {
			if !reportedUnvetted[tool.Name] {
				reportedUnvetted[tool.Name] = true
				problems = append(problems, err.Error())
			}
			unvettedName[i] = true
			continue
		}
		if _, dup := liveByName[tool.Name]; dup {
			if reportedDuplicate[tool.Name] {
				continue
			}
			reportedDuplicate[tool.Name] = true
			// A directory naming one tool twice is self-contradictory; letting
			// the last entry win would silently mask the other's schema.
			problems = append(problems, fmt.Sprintf("live endpoint returned duplicate tool name %q", tool.Name))
			continue
		}
		liveByName[tool.Name] = tool
	}
	declared := make(map[string]bool, len(manifest.Tools))
	for i, decl := range manifest.Tools {
		// Re-check the declaration's name hygiene and scopes as well: this
		// function must not silently rely on the caller having validated
		// first — unchecked scopes would flow verbatim into the persisted
		// snapshot and the admin/member authorization surfaces, and an
		// unhygienic name would otherwise get a misleading "missing from
		// live endpoint" (it can never match a vetted live name). The
		// `where` uses the position index, never the untrusted name itself
		// (OCR T01-R3-F4).
		if err := validateToolName(fmt.Sprintf("manifest tools[%d].name", i), decl.Name, maxToolNameLen); err != nil {
			return nil, "", err
		}
		if err := validateScopes(decl.Scopes, fmt.Sprintf("manifest tools[%d]", i)); err != nil {
			return nil, "", err
		}
		// Re-check here instead of trusting the caller to have run
		// ValidateManifest: a duplicate declaration would otherwise append
		// the same tool to the snapshot twice, silently. The name echoed
		// here is UNVETTED at this point (this function must not rely on
		// the caller having validated first) — truncate it via echoQuoted
		// so a hostile manifest cannot balloon the error (整分支 OCR 一轮 F3).
		if declared[decl.Name] {
			return nil, "", fmt.Errorf("manifest declares duplicate tool name %s", echoQuoted(decl.Name))
		}
		// Re-check the digest format too: ToolSchemaDigest returns "" for an
		// unparseable live schema, so an unchecked empty declared digest would
		// ""=="" match it and mint an authoritative snapshot whose digest is
		// not 64-hex — the invariant later install verification and drift
		// detection compute against (OCR T01-R1-F4). The malformed digest
		// value is not echoed: it is unbounded untrusted input.
		if !schemaDigestPattern.MatchString(decl.InputSchemaDigest) {
			return nil, "", fmt.Errorf("manifest tool %s input_schema_digest must be 64 lowercase hex chars (canonical-JSON SHA-256)", echoQuoted(decl.Name))
		}
		declared[decl.Name] = true
	}

	snapshot := make([]types.PluginToolSnapshot, 0, len(manifest.Tools))
	for _, decl := range manifest.Tools {
		// All three rejection messages below echo MANIFEST-side fields, which
		// are unvetted at this point (this function must not rely on the
		// caller having validated first) — truncate them via echoQuoted so a
		// hostile manifest cannot balloon the joined error (整分支 OCR 二轮
		// F1; a legitimate 64-hex digest is exactly maxEchoRunes and passes
		// untruncated; liveDigest is computed locally and always bounded).
		actual, ok := liveByName[decl.Name]
		if !ok {
			problems = append(problems, fmt.Sprintf("manifest tool %s missing from live endpoint", echoQuoted(decl.Name)))
			continue
		}
		liveDigest := ToolSchemaDigest(actual.InputSchema)
		if liveDigest != decl.InputSchemaDigest {
			problems = append(problems, fmt.Sprintf(
				"manifest tool %s schema digest mismatch (declared %s, live %s)",
				echoQuoted(decl.Name), echoQuoted(decl.InputSchemaDigest), liveDigest,
			))
			continue
		}
		if err := validateDescription(fmt.Sprintf("live description of tool %s", echoQuoted(decl.Name)), actual.Description); err != nil {
			problems = append(problems, err.Error())
			continue
		}
		// Defensive copy: the snapshot is the tenant-facing authority and must
		// not share backing arrays with the untrusted manifest document.
		// make+copy keeps undeclared scopes as [] instead of nil so the field
		// serializes with one shape (跨任务转交 T01-R1-F1).
		scopes := make([]string, len(decl.Scopes))
		copy(scopes, decl.Scopes)
		snapshot = append(snapshot, types.PluginToolSnapshot{
			Name:                 decl.Name,
			Description:          actual.Description,
			InputSchemaDigest:    liveDigest,
			ReadOnly:             decl.ReadOnly,
			RequiresPersonalAuth: decl.RequiresPersonalAuth,
			Scopes:               scopes,
		})
	}
	// One undeclared name is ONE discrepancy — report it once even when the
	// endpoint served it N times (OCR T01-R3-F3).
	reportedUndeclared := make(map[string]bool)
	for i, tool := range live {
		if tool == nil || unvettedName[i] || declared[tool.Name] {
			continue
		}
		if reportedUndeclared[tool.Name] {
			continue
		}
		reportedUndeclared[tool.Name] = true
		problems = append(problems, fmt.Sprintf("undeclared tool %q present on endpoint", tool.Name))
	}
	if len(problems) > maxVerificationProblems {
		problems = append(problems[:maxVerificationProblems],
			fmt.Sprintf("...and %d more discrepancies", len(problems)-maxVerificationProblems))
	}
	if len(problems) > 0 {
		return nil, "", fmt.Errorf("manifest and live endpoint disagree: %s", strings.Join(problems, "; "))
	}
	return snapshot, SnapshotDigest(snapshot), nil
}

// DiffSnapshots computes the five-dimension capability diff between the
// ACCEPTED tool snapshot and a CANDIDATE snapshot (T14): added tools, removed
// tools, per-tool changes (schema digest / scope set / read-write class /
// personal-auth requirement — four independent flags on one change row), and
// the execution endpoint. It is a PURE function over untrusted-but-verified
// data: no network, no clock, no persistence — PreviewUpgrade composes it
// with the re-fetch, which is why the wire never sees an unverified candidate.
//
// Semantics:
//   - identity key is the tool NAME (snapshot names passed BuildVerifiedSnapshot's
//     hygiene, so they are stable identifiers);
//   - scope comparison is SET equality (order-insensitive): the manifest
//     validator rejects duplicates, so a pure set comparison is exact;
//   - description-only differences are NOT changes — free text sits outside
//     the five review dimensions;
//   - output rows are sorted by tool name for deterministic serialization
//     (idempotent previews return byte-identical diffs);
//   - snapshot slices are defensively copied (scopes included) — the diff is
//     tenant-facing review material and must not alias the caller's rows
//     (跨任务转交 T01-R1-F1 convention; empty scope lists stay [], never nil);
//   - version fields (PluginID/CurrentVersion/CandidateVersion/IsDowngrade)
//     stay zeroed — versions are not inputs here; PreviewUpgrade fills them
//     from the installation row and the candidate manifest.
func DiffSnapshots(current, candidate []types.PluginToolSnapshot, currentEndpoint, candidateEndpoint string) *types.PluginVersionDiff {
	currentByName := make(map[string]types.PluginToolSnapshot, len(current))
	for _, tool := range current {
		currentByName[tool.Name] = tool
	}
	candidateByName := make(map[string]types.PluginToolSnapshot, len(candidate))
	for _, tool := range candidate {
		candidateByName[tool.Name] = tool
	}

	added := make([]types.PluginToolSnapshot, 0, len(candidate))
	removed := make([]types.PluginToolSnapshot, 0, len(current))
	changed := make([]types.PluginToolChange, 0, len(current))
	// Single pass over the candidate side (added + changed), then the current
	// side for removed — each name visits exactly once per side.
	for name, cand := range candidateByName {
		cur, ok := currentByName[name]
		if !ok {
			added = append(added, cloneSnapshot(cand))
			continue
		}
		change := types.PluginToolChange{
			Name:      name,
			Current:   cloneSnapshot(cur),
			Candidate: cloneSnapshot(cand),
		}
		change.SchemaChanged = cur.InputSchemaDigest != cand.InputSchemaDigest
		change.ScopeChanged = !sameScopeSet(cur.Scopes, cand.Scopes)
		change.ReadWriteClassChanged = cur.ReadOnly != cand.ReadOnly
		change.PersonalAuthChanged = cur.RequiresPersonalAuth != cand.RequiresPersonalAuth
		if change.SchemaChanged || change.ScopeChanged || change.ReadWriteClassChanged || change.PersonalAuthChanged {
			changed = append(changed, change)
		}
	}
	for name := range currentByName {
		if _, ok := candidateByName[name]; !ok {
			removed = append(removed, cloneSnapshot(currentByName[name]))
		}
	}
	sortSnapshotDiff(added, removed, changed)

	return &types.PluginVersionDiff{
		EndpointChanged:   currentEndpoint != candidateEndpoint,
		CurrentEndpoint:   currentEndpoint,
		CandidateEndpoint: candidateEndpoint,
		AddedTools:        added,
		RemovedTools:      removed,
		ChangedTools:      changed,
	}
}

// cloneSnapshot deep-copies one snapshot (scopes included) and normalizes an
// absent scope list to [] — one wire shape end to end.
func cloneSnapshot(tool types.PluginToolSnapshot) types.PluginToolSnapshot {
	cp := tool
	cp.Scopes = make([]string, len(tool.Scopes))
	copy(cp.Scopes, tool.Scopes)
	return cp
}

// sameScopeSet reports set equality of two scope lists (order-insensitive;
// duplicates are impossible past validateScopes, so multiset semantics are
// unnecessary).
func sameScopeSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, s := range a {
		seen[s]++
	}
	for _, s := range b {
		seen[s]--
		if seen[s] < 0 {
			return false
		}
	}
	return true
}

// sortSnapshotDiff orders every diff row by tool name for deterministic
// output (map iteration order must never leak into the admin review surface
// or an idempotency assertion).
func sortSnapshotDiff(added, removed []types.PluginToolSnapshot, changed []types.PluginToolChange) {
	sort.Slice(added, func(i, j int) bool { return added[i].Name < added[j].Name })
	sort.Slice(removed, func(i, j int) bool { return removed[i].Name < removed[j].Name })
	sort.Slice(changed, func(i, j int) bool { return changed[i].Name < changed[j].Name })
}

// DiffLiveAgainstSnapshot computes how a plugin installation's LIVE endpoint
// directory deviates from its ACCEPTED tools snapshot (T17): added tools
// (live-only names), removed tools (snapshot-only names), schema changes and
// description changes (same-name tools whose live values disagree with the
// accepted ones). It is a PURE function — no network, no clock beyond the
// CheckedAt stamp, no persistence; CheckDrift and ResolveDrift compose it
// with the EndpointLister fetch (the accepted endpoint is the drift
// authority, never the manifest — 总索引「安装后远端真相的统一口径」).
//
// Semantics (mirrors the T09 runtime guard, tools.FilterToolsBySnapshot, so
// the persisted verdict never disagrees with what blocks member calls):
//   - identity key is the tool NAME (live names are untrusted remote data but
//     only feed string comparisons and the name-only detail — never schema
//     text, per the plan's drift_detail hygiene constraint);
//   - schema comparison is digest-based (ToolSchemaDigest over the live raw
//     schema), the same convention the snapshot itself was minted with;
//   - description comparison is exact — the runtime guard fail-closes on a
//     post-install rewrite, so the drift record must carry it too
//     (DescriptionChanged; without it a description-only drift would leave
//     member conversations blocked while CheckDrift says "none");
//   - every output list is sorted by name for deterministic serialization;
//   - a nil detail is never returned on success — a no-drift comparison
//     yields empty lists (HasDrift()==false), one shape end to end;
//   - the UNTRUSTED live directory is vetted first via the same
//     ValidateLiveDirectoryForRebase gates (size cap, duplicate-name
//     rejection, identifier hygiene — OCR R1 F15): drift's premise is that
//     the endpoint may have turned hostile, so an oversized/duplicated/
//     unhygienic directory yields (nil, err) and the caller treats it as
//     "cannot produce a drift verdict" instead of persisting unvetted names
//     into drift_detail.
func DiffLiveAgainstSnapshot(live []*types.MCPTool, snap []types.PluginToolSnapshot) (*types.PluginDriftDetail, error) {
	if err := ValidateLiveDirectoryForRebase(live); err != nil {
		return nil, err
	}
	accepted := make(map[string]types.PluginToolSnapshot, len(snap))
	for _, tool := range snap {
		accepted[tool.Name] = tool
	}
	liveNames := make(map[string]*types.MCPTool, len(live))
	for _, tool := range live {
		if tool == nil {
			continue
		}
		liveNames[tool.Name] = tool
	}

	detail := &types.PluginDriftDetail{
		Added:              []string{},
		Removed:            []string{},
		SchemaChanged:      []string{},
		DescriptionChanged: []string{},
		CheckedAt:          time.Now(),
	}
	for name, liveTool := range liveNames {
		acceptedTool, ok := accepted[name]
		if !ok {
			// The tenant never accepted this capability — the drift record
			// names it for review; the runtime guard drops it from Agent
			// discovery.
			detail.Added = append(detail.Added, name)
			continue
		}
		if ToolSchemaDigest(liveTool.InputSchema) != acceptedTool.InputSchemaDigest {
			detail.SchemaChanged = append(detail.SchemaChanged, name)
		}
		if liveTool.Description != acceptedTool.Description {
			detail.DescriptionChanged = append(detail.DescriptionChanged, name)
		}
	}
	for name := range accepted {
		if _, ok := liveNames[name]; !ok {
			detail.Removed = append(detail.Removed, name)
		}
	}
	sort.Strings(detail.Added)
	sort.Strings(detail.Removed)
	sort.Strings(detail.SchemaChanged)
	sort.Strings(detail.DescriptionChanged)
	return detail, nil
}

// ValidateLiveDirectoryForRebase vets an UNTRUSTED live directory before
// ResolveDrift mints it into the tenant's accepted snapshot (T17-OCR1-F6):
// the install/upgrade path runs the same directory through
// BuildVerifiedSnapshot's hygiene gates (size cap, duplicate-name rejection,
// per-name identifier hygiene) — and drift's PREMISE is that the endpoint may
// have turned hostile, so the rebase path must not trust what the install
// path vetted. Semantics mirror BuildVerifiedSnapshot exactly:
//
//   - more than maxLiveTools tools → rejection (a hostile endpoint cannot
//     balloon the snapshot, the policy rows and the member-visible directory,
//     nor stretch the serialized DB writes inside the resolve's lock);
//   - one duplicated name → rejection (letting either entry win would mint a
//     self-contradictory baseline — the same verdict the install path gives);
//   - a name failing the identifier hygiene rules → rejection (unvetted
//     names must not reach the snapshot, the per-tool policy rows or the
//     member-visible directory). The `where` echoes the position index,
//     never the untrusted name itself.
//
// A nil/empty clean directory passes (removing every tool is a reviewable
// drift form, not an attack). The caller rejects the resolve with ZERO
// writes on any error.
func ValidateLiveDirectoryForRebase(live []*types.MCPTool) error {
	if len(live) > maxLiveTools {
		return fmt.Errorf("live endpoint returned %d tools, exceeding the maximum of %d", len(live), maxLiveTools)
	}
	seen := make(map[string]bool, len(live))
	for i, tool := range live {
		if tool == nil {
			continue
		}
		// Vet up front, keyed by position (never echo the untrusted name):
		// everything that reaches the duplicate echo below has passed the
		// same hygiene the install path enforces.
		if err := validateToolName(fmt.Sprintf("live tools[%d].name", i), tool.Name, maxToolNameLen); err != nil {
			return err
		}
		if seen[tool.Name] {
			return fmt.Errorf("live endpoint returned duplicate tool name %q", tool.Name)
		}
		seen[tool.Name] = true
	}
	return nil
}

// ComparePluginVersions compares two weknora.plugin/1 version strings
// (MAJOR.MINOR.PATCH) by three-segment numeric value: -1 when a < b, 0 when
// equal, 1 when a > b (T14's IsDowngrade basis). Versions reaching the plugin
// domain already passed pluginVersionPattern (no leading zeros, each
// component at most 9 digits — ValidateManifest on the manifest side, the
// installation row's copy of a validated manifest on the accepted side), so
// unparsable input is a defensive path only and compares as equal (0): a
// verdict of "not a downgrade" on malformed input keeps the flag
// conservative, and no error surface is invented for an unreachable case.
func ComparePluginVersions(a, b string) int {
	aMajor, aMinor, aPatch, aOK := parseVersionTriple(a)
	bMajor, bMinor, bPatch, bOK := parseVersionTriple(b)
	if !aOK || !bOK {
		return 0
	}
	for _, pair := range [][2]uint64{
		{aMajor, bMajor}, {aMinor, bMinor}, {aPatch, bPatch},
	} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	return 0
}

// parseVersionTriple splits an MAJOR.MINOR.PATCH string into its numeric
// triple. ok is false for any non-conforming input.
func parseVersionTriple(v string) (major, minor, patch uint64, ok bool) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	nums := make([]uint64, 3)
	for i, part := range parts {
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return 0, 0, 0, false
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], true
}
