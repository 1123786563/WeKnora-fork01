package craft

// T13 (#133): restricted derived-export consent — the pure rules on top of
// the T00 frozen DTOs (ExportManifest, ExportDecision, DecisionStatus).
// Before a bundle includes data derived from restricted sources, the server
// classifies every derived file from its RECORDED origins, shows the owner
// the exact manifest, and records a decision bound to the immutable
// Version ID + Export Manifest digest. Every rule here is server-side; no
// request input widens it, and the classification never looks at the
// mutable Workspace or the live Run record — only at the immutable
// manifest facts the export projection (T12) already recorded.

import "fmt"

// ExportConsentState is the externally visible consent state of one
// immutable version's export manifest. Restricted originals never enter a
// version manifest; consent governs only the DERIVED members.
type ExportConsentState string

const (
	// ExportConsentNone: no derived member carries a restricted origin —
	// nothing to consent to, downloads need no decision.
	ExportConsentNone ExportConsentState = "none"
	// ExportConsentAwaiting: restricted derived members exist and no live
	// decision of the current owner binds this exact manifest.
	ExportConsentAwaiting ExportConsentState = "awaiting"
	// ExportConsentConsented: the current owner's approved decision binds
	// exactly this Version ID and manifest digest.
	ExportConsentConsented ExportConsentState = "consented"
	// ExportConsentDeclined: the current owner explicitly rejected
	// exporting the restricted derived members.
	ExportConsentDeclined ExportConsentState = "declined"
)

// Valid reports whether the state is one of the closed vocabulary.
func (s ExportConsentState) Valid() bool {
	return s == ExportConsentNone || s == ExportConsentAwaiting ||
		s == ExportConsentConsented || s == ExportConsentDeclined
}

// DerivedFileRestricted classifies ONE derived file from its recorded
// origins: a member is restricted derived iff ANY origin that contributed
// to it is restricted (a cross-tenant shared-library original). The
// classification is per file — one restricted member never restricts its
// siblings — and reads the ORIGIN facts, never the file's own Restricted
// flag (that flag marks restricted ORIGINALS, which by construction never
// enter a version manifest).
func DerivedFileRestricted(f ExportFile) bool {
	for _, origin := range f.Origins {
		if origin.Restricted {
			return true
		}
	}
	return false
}

// RestrictedDerivedPaths lists the manifest's restricted derived members in
// manifest order. The paths are exactly what a safe bundle withholds and
// what the owner is shown before deciding.
func RestrictedDerivedPaths(m ExportManifest) []string {
	paths := make([]string, 0, len(m.Files))
	for _, f := range m.Files {
		if DerivedFileRestricted(f) {
			paths = append(paths, f.Path)
		}
	}
	return paths
}

// SafeExportManifest projects the safe fallback bundle: exactly the
// members that are NOT restricted derived, with their recorded facts
// verbatim, and its own freshly derived canonical digest. The safe bundle
// is a different bundle honestly labelled as one — its digest differs from
// the full manifest's, so a consent recorded for the full digest can never
// be replayed as authority for the safe projection or vice versa. An
// all-restricted manifest degrades to an empty (still validated) safe
// manifest: without consent no restricted derived byte is returned at all.
func SafeExportManifest(m ExportManifest) (ExportManifest, error) {
	files := make([]ExportFile, 0, len(m.Files))
	for _, f := range m.Files {
		if !DerivedFileRestricted(f) {
			files = append(files, f)
		}
	}
	safe := ExportManifest{VersionID: m.VersionID, Files: files}
	digest, err := ExportManifestDigest(safe)
	if err != nil {
		return ExportManifest{}, fmt.Errorf("%w: safe export manifest: %v", ErrInvalidInput, err)
	}
	safe.ManifestDigest = digest
	return safe, nil
}

// ExportDecisionBinds reports whether an export decision binds exactly one
// manifest identity: the same immutable Version ID AND the same canonical
// Export Manifest digest. A changed manifest (or another version) unbinds
// a prior decision, so it can never grant authority for what the owner did
// not see; replay cannot broaden scope.
func ExportDecisionBinds(d ExportDecision, versionID, manifestDigest string) bool {
	if err := d.Validate(); err != nil {
		return false
	}
	return d.VersionID == versionID && d.ManifestDigest == manifestDigest
}

// GrantsExportAuthority is the single rule that turns a persisted decision
// into export authority for one manifest: the decision must be the CURRENT
// owner's APPROVED decision binding exactly this Version ID and manifest
// digest. Everything else — rejected, unknown, missing, stale (replayed
// for another manifest or version) and former-owner decisions — creates no
// authority, so the restricted derived members stay withheld.
func GrantsExportAuthority(d *ExportDecision, versionID, manifestDigest, currentOwner string) bool {
	if d == nil {
		return false
	}
	if err := d.Validate(); err != nil {
		return false
	}
	if d.Decision != DecisionApproved {
		return false
	}
	if !ExportDecisionBinds(*d, versionID, manifestDigest) {
		return false
	}
	if currentOwner == "" || d.OwnerID != currentOwner {
		return false
	}
	return true
}

// ExportConsentStateOf reduces one manifest's restricted derived members
// and its latest persisted decision (nil = none recorded) to the
// externally visible consent state. currentOwner is the task's CURRENT
// owner resolved server-side: a decision of a former owner is history, so
// after an ownership move the state awaits the new owner's decision again.
func ExportConsentStateOf(restrictedDerived []string, d *ExportDecision, versionID, manifestDigest, currentOwner string) ExportConsentState {
	if len(restrictedDerived) == 0 {
		return ExportConsentNone
	}
	if d == nil || !ExportDecisionBinds(*d, versionID, manifestDigest) {
		return ExportConsentAwaiting
	}
	if d.Decision == DecisionRejected {
		// Symmetric with the approved path below: a decision of a FORMER
		// owner is history — after an ownership move the state awaits the
		// new owner's decision again, whichever way the old decision went.
		// (A former owner's rejection surfacing as declined would attribute
		// the current ownership with a decision it never made.)
		if currentOwner == "" || d.OwnerID != currentOwner {
			return ExportConsentAwaiting
		}
		return ExportConsentDeclined
	}
	if GrantsExportAuthority(d, versionID, manifestDigest, currentOwner) {
		return ExportConsentConsented
	}
	return ExportConsentAwaiting
}
