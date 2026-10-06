package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/experts"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Agent Fork lineage + re-distribution gate (T32 #62, spec §5/§6/§9).
//
// The lineage is a SERVER-side fact: it is resolved from the adoption
// variant ledger via the frozen version's source Agent alone — the request
// body never carries lineage fields (strict decoding rejects them). The
// fork verdict compares the submitted portable core against the source
// Release's publish-pipeline projection; the redistribution gate consults
// the deployment license registry live and fails closed.

var (
	// ErrReleaseRedistributionForbidden rejects a derived Submission whose
	// source license does not (or does not yet) permit redistribution.
	ErrReleaseRedistributionForbidden = errors.New("release redistribution is forbidden by the source lineage license")
	// ErrReleaseLineageUnavailable marks unresolvable/corrupt lineage data;
	// the submission fails closed instead of passing as non-fork.
	ErrReleaseLineageUnavailable = errors.New("release derivation lineage could not be resolved")
	// ErrAgentLicenseInvalid rejects malformed license registry writes.
	ErrAgentLicenseInvalid = errors.New("invalid agent license request")
)

// resolveSubmissionLineage derives the submission's lineage and fork
// verdict, gate included. Returns (nil, "", nil) for original content:
// no lineage columns, no license lookup, a lineage-free manifest.
func (s *AgentMarketplaceService) resolveSubmissionLineage(ctx context.Context, tenantID uint64, version types.AgentVersionSnapshot) (*types.AgentReleaseLineage, string, error) {
	derivation, err := s.repo.FindDerivation(ctx, tenantID, version.AgentID)
	if err != nil {
		return nil, "", err
	}
	if derivation == nil {
		return nil, "", nil
	}
	release := derivation.Release
	if release == nil {
		return nil, "", fmt.Errorf("%w: derivation variant %s pins a missing release", ErrReleaseLineageUnavailable, derivation.Variant.ID)
	}
	var sourceManifest types.AgentReleaseManifest
	if err := json.Unmarshal([]byte(release.ManifestJSON), &sourceManifest); err != nil {
		return nil, "", fmt.Errorf("%w: decode the source release manifest: %v", ErrReleaseLineageUnavailable, err)
	}
	if err := s.requireRedistributable(ctx, sourceManifest.LicenseID); err != nil {
		return nil, "", err
	}
	baseline, err := derivationBaselinePayload(derivation)
	if err != nil {
		return nil, "", err
	}
	submitted, err := experts.PortablePayloadOf(version.Agent)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrReleaseLineageUnavailable, err)
	}
	isFork, err := forkVerdict(submitted, baseline)
	if err != nil {
		return nil, "", err
	}
	return &types.AgentReleaseLineage{
		SourceListingID: derivation.ListingID,
		SourceReleaseID: release.ID,
		IsFork:          isFork,
	}, strings.TrimSpace(sourceManifest.LicenseID), nil
}

// requireRedistributable fails closed unless the deployment license
// registry records the source license as redistribution-permitted
// (spec §9「许可证允许时，Fork 可经 Tenant Review 或 Verified Publisher 加
// Platform Review 再发布」). An unregistered license is NOT permission.
func (s *AgentMarketplaceService) requireRedistributable(ctx context.Context, licenseID string) error {
	licenseID = strings.TrimSpace(licenseID)
	license, err := s.repo.GetLicense(ctx, licenseID)
	if err != nil {
		return err
	}
	if license == nil {
		return fmt.Errorf("%w: source license %q is not registered in the license registry", ErrReleaseRedistributionForbidden, licenseID)
	}
	if !license.AllowsRedistribution {
		return fmt.Errorf("%w: source license %q does not permit redistribution", ErrReleaseRedistributionForbidden, licenseID)
	}
	return nil
}

// derivationBaselinePayload recomputes the portable payload the publish
// pipeline would produce from the source Release (buildLocalAgent ->
// PortablePayloadOf). Comparing against THIS baseline — not the raw source
// payload — keeps pure capability-mapping work from reading as a
// portable-core change despite the publish projection's round-trip shape
// (spec §9「只修改本地资源和策略属于 Mapping」; plan-t62 差异记录 #3).
func derivationBaselinePayload(derivation *types.AgentForkDerivation) (types.AgentReleasePayload, error) {
	var envelope struct {
		Payload types.AgentReleasePayload `json:"payload"`
	}
	if err := json.Unmarshal(derivation.Release.Bundle, &envelope); err != nil {
		return types.AgentReleasePayload{}, fmt.Errorf("%w: decode the source release bundle: %v", ErrReleaseLineageUnavailable, err)
	}
	var manifest types.AgentReleaseManifest
	// buildLocalAgent reads only the Summary; the manifest was already
	// strictly decoded for the license gate above, so a tolerated decode
	// here cannot bypass the gate.
	_ = json.Unmarshal([]byte(derivation.Release.ManifestJSON), &manifest)
	agent := buildLocalAgent(&derivation.Variant, envelope.Payload, manifest, nil)
	baseline, err := experts.PortablePayloadOf(agent)
	if err != nil {
		return types.AgentReleasePayload{}, fmt.Errorf("%w: project the source baseline: %v", ErrReleaseLineageUnavailable, err)
	}
	return baseline, nil
}

// forkVerdict decides whether a lineage-derived submission is a Fork
// (CONTEXT.md「Agent Fork」): only a portable-core change is a Fork. Local
// capability mapping bindings never enter the portable payload and
// therefore never flip the verdict (AC1).
func forkVerdict(submitted, baseline types.AgentReleasePayload) (bool, error) {
	submittedJSON, err := json.Marshal(submitted)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrReleaseLineageUnavailable, err)
	}
	baselineJSON, err := json.Marshal(baseline)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrReleaseLineageUnavailable, err)
	}
	return !bytes.Equal(submittedJSON, baselineJSON), nil
}

// RegisterLicense records (or re-records) one deployment license term. The
// deployment is the governance domain: Admin+ members manage the same
// registry the redistribution gate consults live.
func (s *AgentMarketplaceService) RegisterLicense(ctx context.Context, actorID string, input interfaces.LicenseInput) (types.AgentLicenseEntity, error) {
	actorID, input.ID, input.Name = strings.TrimSpace(actorID), strings.TrimSpace(input.ID), strings.TrimSpace(input.Name)
	if actorID == "" || input.ID == "" || len(input.ID) > 64 || len(input.Name) > 255 {
		return types.AgentLicenseEntity{}, ErrAgentLicenseInvalid
	}
	row, err := s.repo.UpsertLicense(ctx, &types.AgentLicenseEntity{
		ID: input.ID, Name: input.Name, AllowsRedistribution: input.AllowsRedistribution,
		CreatedBy: actorID, UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		return types.AgentLicenseEntity{}, err
	}
	return *row, nil
}

// ListLicenses returns the deployment license registry.
func (s *AgentMarketplaceService) ListLicenses(ctx context.Context) ([]types.AgentLicenseEntity, error) {
	return s.repo.ListLicenses(ctx)
}
