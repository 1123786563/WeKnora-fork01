// T11 受限来源共享同意 — restricted-share consent contracts.
//
// Story 32: sharing a webpage derived from restricted sources must be a
// conscious owner choice. This file holds the pure sharing rules of the
// craft package on top of the T00 frozen DTOs (RestrictedContribution,
// ShareDecision, DecisionStatus): how one version's restricted
// contribution is derived from RECORDED evidence, what a consent binds,
// and the single rule that turns a persisted decision into sharing
// authority. Every rule here is server-side; no request input widens it.
package craft

import (
	"fmt"
	"time"
)

// ShareDecisionTTL bounds how long one approved share decision keeps
// authority. Expiry is enforced on every read: an expired consent grants
// nothing until the current owner consents again against current evidence.
const ShareDecisionTTL = 24 * time.Hour

// ShareState is the externally visible sharing authority state of one
// immutable version. Consent never widens original-source access; it only
// authorizes sharing the derived result.
type ShareState string

const (
	// ShareStatePrivate is the default of a restricted version: no live
	// consent binds it, so sharing authority does not exist yet.
	ShareStatePrivate ShareState = "private"
	// ShareStateConsented means sharing authority currently exists — either
	// the version's contribution is unrestricted, or a live approved owner
	// decision binds this exact version and evidence digest.
	ShareStateConsented ShareState = "consented"
	// ShareStateDeclined means the owner explicitly rejected sharing.
	ShareStateDeclined ShareState = "declined"
)

// Valid reports whether the state is one of the closed vocabulary.
func (s ShareState) Valid() bool {
	return s == ShareStatePrivate || s == ShareStateConsented || s == ShareStateDeclined
}

// RestrictedContributionFrom derives one version's restricted-source
// contribution from RECORDED evidence only: the citation manifest the
// version stores (T06) supplies the stable evidence digest, and the Run's
// immutable knowledge record (T05) supplies the restricted fact. A recorded
// source owned by a DIFFERENT tenant than the Task — an organization-shared
// library original the Run consumed — marks the contribution restricted:
// the derived result is built from material viewers may not individually
// hold permission to open. The manifest's rendered citations do not narrow
// the flag: a shared original that contributed without being cited still
// restricts the version.
func RestrictedContributionFrom(versionID string, manifest WebCitationManifest, record KnowledgeRecord) (RestrictedContribution, error) {
	if versionID == "" {
		return RestrictedContribution{}, fmt.Errorf("%w: restricted contribution requires the version id", ErrInvalidInput)
	}
	digest, err := WebCitationsDigest(manifest)
	if err != nil {
		return RestrictedContribution{}, err
	}
	restricted := false
	for _, source := range record.Sources {
		// Fail-closed on the degenerate tenant 0 as well: a source row that
		// lost its tenant attribution behaves like a cross-tenant source
		// (same direction as export_manifest.go), never like an own-tenant
		// one — the opposite default would grant shared derivation without
		// the owner's consent.
		if source.TenantID != record.Scope.TenantID {
			restricted = true
			break
		}
	}
	contribution := RestrictedContribution{VersionID: versionID, EvidenceDigest: digest, Restricted: restricted}
	if err := contribution.Validate(); err != nil {
		return RestrictedContribution{}, err
	}
	return contribution, nil
}

// DecisionBinds reports whether a share decision binds exactly this
// contribution: the same immutable Version ID and the same evidence digest.
// Changed evidence (a different digest) or another version unbinds a prior
// decision, so it can never grant authority for what the owner did not see.
func DecisionBinds(d ShareDecision, c RestrictedContribution) bool {
	if err := d.Validate(); err != nil {
		return false
	}
	return d.VersionID == c.VersionID && d.EvidenceDigest == c.EvidenceDigest
}

// RecordedShareDecision is the durable share-decision fact: the frozen wire
// decision plus its server-recorded lifecycle. DecidedAt starts the TTL
// window; RevokedAt (zero = still live) ends the decision permanently — a
// revoked consent never grants again, even inside its TTL.
type RecordedShareDecision struct {
	Decision  ShareDecision
	DecidedAt time.Time
	RevokedAt time.Time
}

// Validate enforces the decision's own shape plus a sane lifecycle.
func (r RecordedShareDecision) Validate() error {
	if err := r.Decision.Validate(); err != nil {
		return err
	}
	if r.DecidedAt.IsZero() {
		return fmt.Errorf("%w: share decision carries no decided-at", ErrInvalidInput)
	}
	if !r.RevokedAt.IsZero() && r.RevokedAt.Before(r.DecidedAt) {
		return fmt.Errorf("%w: share decision revoked before it was decided", ErrInvalidInput)
	}
	return nil
}

// GrantsShareAuthority is the single rule that turns a persisted decision
// into sharing authority for one contribution: the decision must be the
// owner's APPROVED decision, bind this exact version and evidence digest,
// sit inside its TTL window, and not be revoked. Everything else — rejected,
// unknown, missing, expired, stale (replayed for other evidence) and
// revoked decisions — creates no authority. now is server time; callers
// never accept a client clock.
func GrantsShareAuthority(c RestrictedContribution, d *RecordedShareDecision, now time.Time) bool {
	if err := c.Validate(); err != nil {
		return false
	}
	if d == nil {
		return false
	}
	if err := d.Validate(); err != nil {
		return false
	}
	if d.Decision.Decision != DecisionApproved {
		return false
	}
	if !DecisionBinds(d.Decision, c) {
		return false
	}
	if now.Before(d.DecidedAt) || now.Sub(d.DecidedAt) > ShareDecisionTTL {
		return false
	}
	if !d.RevokedAt.IsZero() {
		return false
	}
	return true
}

// ShareStateOf reduces one contribution and its latest persisted decision
// (nil = none recorded) to the externally visible sharing state. An
// unrestricted version carries no consent requirement, so its authority
// exists by default; a restricted version is private until a live approved
// decision binds it, and declined only when the owner explicitly rejected.
func ShareStateOf(c RestrictedContribution, d *RecordedShareDecision, now time.Time) ShareState {
	if err := c.Validate(); err != nil {
		return ShareStatePrivate
	}
	if !c.Restricted {
		return ShareStateConsented
	}
	if d == nil || d.Validate() != nil {
		return ShareStatePrivate
	}
	switch {
	case d.Decision.Decision == DecisionRejected && DecisionBinds(d.Decision, c):
		return ShareStateDeclined
	case GrantsShareAuthority(c, d, now):
		return ShareStateConsented
	default:
		return ShareStatePrivate
	}
}
