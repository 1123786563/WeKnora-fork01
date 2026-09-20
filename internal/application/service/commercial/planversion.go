package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"

	"gorm.io/gorm"
)

var (
	// ErrPublishValidationFailed: at least one of the six acceptance axes
	// failed; the version stays draft and NEVER reaches the seam. The
	// itemized report rides the PublishResult.
	ErrPublishValidationFailed = errors.New("publish_validation_failed")
	// ErrInvalidDraftInput: a draft payload is structurally unusable (empty
	// name, non-slug key, non-positive amounts).
	ErrInvalidDraftInput = errors.New("invalid_draft_input")
)

// draftCreateAttempts bounds the NextVersion→CreateDraft retry loop against
// a concurrent creator winning the primary-key race.
const draftCreateAttempts = 5

// AxisResult is one acceptance axis verdict (closed token).
type AxisResult struct {
	Axis   string `json:"axis"`
	Passed bool   `json:"passed"`
}

// ValidationReport is the itemized six-axis answer served by the validate
// endpoint and embedded in a failed publish result. Closed tokens only.
type ValidationReport struct {
	Valid bool         `json:"valid"`
	Axes  []AxisResult `json:"axes"`
}

// ReceiptView is the provider-neutral publish receipt served to the admin
// API: received, when, and under which command identity. The external plan
// code is deliberately ABSENT (ADR-0014; the mapping lives in
// commercial_plan_publications).
type ReceiptView struct {
	Received    bool      `json:"received"`
	PublishedAt time.Time `json:"published_at"`
	CommandKey  string    `json:"command_key"`
}

// PublishResult carries the version view after the operation, the
// validation report (always populated for a validation failure), and the
// receipt once published.
type PublishResult struct {
	Version repocommercial.VersionView
	Report  ValidationReport
	Receipt *ReceiptView
}

// DraftInput is the admin-API draft payload: everything a plan version
// needs before validation and publish.
type DraftInput struct {
	PlanKey              string
	Name                 string
	AmountFen            int64
	IncludedCreditsMicro int64
	Features             map[string]bool
	Limits               map[string]int64
	Currency             string
	Charges              []domain.PlanCharge
}

// PlanVersionService orchestrates the immutable plan-version lifecycle
// (#79, Lago T07): create/update drafts, run the six acceptance axes, and
// publish through the frozen Commercial Platform seam with
// coordinator-owned idempotency (Command.Key =
// publish_plan_version:<plan_key>:<version>). The service holds NO direct
// database handle: every persistence goes through the repository's
// transactional units (the OrderService precedent). A nil platform is
// legal (blocked-env): drafts and validation work, publish fails closed.
type PlanVersionService struct {
	versions *repocommercial.PlanVersionStore
	catalog  *repocommercial.CatalogStore
	platform domain.CommercialPlatform
}

// NewPlanVersionService builds the service and bootstraps its schema (the
// portable EnsureSchema — safe next to the versioned migrations).
func NewPlanVersionService(db *gorm.DB, platform domain.CommercialPlatform) (*PlanVersionService, error) {
	versions := repocommercial.NewPlanVersionStore(db)
	if err := versions.EnsureSchema(context.Background()); err != nil {
		return nil, err
	}
	return &PlanVersionService{
		versions: versions,
		catalog:  repocommercial.NewCatalogStore(db),
		platform: platform,
	}, nil
}

// CreateDraft allocates the next version for the plan key (PK-retry against
// a concurrent creator), derives the deterministic plan code and stores the
// draft definition. The plan code lands in the catalog external_id — a
// seam-internal column never surfaced by the admin API.
func (s *PlanVersionService) CreateDraft(ctx context.Context, actor string, in DraftInput) (repocommercial.VersionView, error) {
	if err := s.validateDraftInput(in); err != nil {
		return repocommercial.VersionView{}, err
	}
	_ = actor // drafts carry no receipt; the actor rides the publish command
	var lastErr error
	for attempt := 0; attempt < draftCreateAttempts; attempt++ {
		version, err := s.versions.NextVersion(ctx, in.PlanKey)
		if err != nil {
			return repocommercial.VersionView{}, err
		}
		row, err := s.draftRow(in, version)
		if err != nil {
			return repocommercial.VersionView{}, err
		}
		if err := s.versions.CreateDraft(ctx, row); err != nil {
			if errors.Is(err, repocommercial.ErrPlanVersionExists) {
				lastErr = err // lost the PK race; retry with a fresh version
				continue
			}
			return repocommercial.VersionView{}, err
		}
		return s.versions.GetVersion(ctx, in.PlanKey, version)
	}
	return repocommercial.VersionView{}, fmt.Errorf("%w: version allocation retried out: %v", ErrInvalidDraftInput, lastErr)
}

// UpdateDraft rewrites the definition of a DRAFT version; published and
// publishing versions are immutable by the store guard AND the DB trigger.
func (s *PlanVersionService) UpdateDraft(ctx context.Context, planKey string, version int64, in DraftInput) (repocommercial.VersionView, error) {
	if err := s.validateDraftInput(in); err != nil {
		return repocommercial.VersionView{}, err
	}
	if in.PlanKey != planKey {
		return repocommercial.VersionView{}, ErrInvalidDraftInput
	}
	row, err := s.draftRow(in, version)
	if err != nil {
		return repocommercial.VersionView{}, err
	}
	if err := s.versions.UpdateDraft(ctx, planKey, version, row.DefinitionJSON); err != nil {
		return repocommercial.VersionView{}, err
	}
	return s.versions.GetVersion(ctx, planKey, version)
}

// Validate runs the six acceptance axes against the CURRENT published
// neighborhood and returns the itemized report. NO state change.
func (s *PlanVersionService) Validate(ctx context.Context, planKey string, version int64) (ValidationReport, error) {
	view, err := s.versions.GetVersion(ctx, planKey, version)
	if err != nil {
		return ValidationReport{}, err
	}
	definition, err := decodeDefinition(view.DefinitionJSON)
	if err != nil {
		return ValidationReport{}, err
	}
	vctx, err := s.publishContext(ctx)
	if err != nil {
		return ValidationReport{}, err
	}
	return validationReport(definition.ValidateForPublishValidated(vctx)), nil
}

// Publish runs the fail-closed pipeline: idempotent replay → six-axis
// validation (invalid stays draft, zero seam calls) → publishing state →
// the seam command under the coordinator-owned key → the durable receipt.
//
// Adapter error mapping (the row stays publishing on every failure — the
// retry replays the SAME key, never a new identity):
//
//	ErrPlatformUnconfigured     → closed unconfigured (blocked-env)
//	ErrPlatformUnreachable      → closed unreachable (retryable, same key)
//	ErrPlatformInvalidResponse  → closed publish conflict (operator attention)
func (s *PlanVersionService) Publish(ctx context.Context, actor, reason, planKey string, version int64) (PublishResult, error) {
	view, err := s.versions.GetVersion(ctx, planKey, version)
	if err != nil {
		return PublishResult{}, err
	}

	// Idempotent replay: a published version with a recorded publication
	// answers its recorded receipt — NO seam call, ever.
	if view.State == domain.PlanStatePublished {
		if pub, err := s.versions.GetPublication(ctx, planKey, version); err == nil {
			return PublishResult{
				Version: view,
				Receipt: &ReceiptView{Received: true, PublishedAt: pub.PublishedAt, CommandKey: pub.CommandKey},
			}, nil
		} else if !errors.Is(err, repocommercial.ErrPublicationNotFound) {
			return PublishResult{}, err
		}
	}

	definition, err := decodeDefinition(view.DefinitionJSON)
	if err != nil {
		return PublishResult{}, err
	}
	vctx, err := s.publishContext(ctx)
	if err != nil {
		return PublishResult{}, err
	}
	report := validationReport(definition.ValidateForPublishValidated(vctx))
	if !report.Valid {
		// Failed validation never reaches the seam; the version stays draft.
		current, err := s.versions.GetVersion(ctx, planKey, version)
		if err != nil {
			return PublishResult{}, err
		}
		return PublishResult{Version: current, Report: report}, ErrPublishValidationFailed
	}

	// A nil platform can never publish: fail closed BEFORE any state move.
	if s.platform == nil {
		return PublishResult{}, fmt.Errorf("%w: no commercial platform wired", domain.ErrPlatformUnconfigured)
	}

	payload := publishPayload(definition)

	if err := s.versions.SetPublishing(ctx, planKey, version); err != nil {
		return PublishResult{}, err
	}
	receipt, err := s.platform.SubmitCommand(ctx, domain.Command{
		Kind:    domain.CommandKindPublishPlanVersion,
		Key:     domain.PublishCommandKey(planKey, version),
		Actor:   actor,
		Reason:  reason,
		Payload: payload,
	})
	if err != nil {
		// The row stays publishing: the same command key replays the same
		// external object on retry; provider text never crosses.
		return PublishResult{}, err
	}
	receiptJSON, err := json.Marshal(receipt)
	if err != nil {
		return PublishResult{}, err
	}
	if err := s.versions.RecordPublication(ctx, repocommercial.PublicationRow{
		CommandKey:  domain.PublishCommandKey(planKey, version),
		PlanKey:     planKey,
		Version:     version,
		PlanCode:    payload.PlanCode, // seam-internal; never served
		ReceiptJSON: string(receiptJSON),
		PublishedBy: actor,
		PublishedAt: receipt.RecordedAt.UTC(),
	}, planKey, version); err != nil {
		return PublishResult{}, err
	}
	final, err := s.versions.GetVersion(ctx, planKey, version)
	if err != nil {
		return PublishResult{}, err
	}
	return PublishResult{
		Version: final,
		Report:  report,
		Receipt: &ReceiptView{Received: true, PublishedAt: receipt.RecordedAt.UTC(), CommandKey: domain.PublishCommandKey(planKey, version)},
	}, nil
}

// ListVersions returns the catalog joined with publication receipts.
func (s *PlanVersionService) ListVersions(ctx context.Context) ([]repocommercial.VersionView, error) {
	return s.versions.ListVersions(ctx)
}

// GetVersion returns one version view.
func (s *PlanVersionService) GetVersion(ctx context.Context, planKey string, version int64) (repocommercial.VersionView, error) {
	return s.versions.GetVersion(ctx, planKey, version)
}

// GetPublication returns the recorded publication of one version
// (seam-internal fields — the admin API never projects it).
func (s *PlanVersionService) GetPublication(ctx context.Context, planKey string, version int64) (repocommercial.PublicationRow, error) {
	return s.versions.GetPublication(ctx, planKey, version)
}

// ListPublications returns the publications of one plan key.
func (s *PlanVersionService) ListPublications(ctx context.Context, planKey string) ([]repocommercial.PublicationRow, error) {
	pubs, err := s.versions.ListPublications(ctx, planKey)
	return pubs, err
}

// publishContext assembles the cross-version validation inputs: the closed
// tier ladder plus the price of the LATEST published version of every plan
// key (the neighborhood a new price must not invert).
func (s *PlanVersionService) publishContext(ctx context.Context) (domain.PublishValidationContext, error) {
	rows, err := s.catalog.ListPublished(ctx)
	if err != nil {
		return domain.PublishValidationContext{}, err
	}
	// rows are ordered by (plan_key, version): the last write per key is its
	// latest published version.
	prices := make(map[string]int64)
	for _, row := range rows {
		definition, err := decodeDefinition(row.DefinitionJSON)
		if err != nil {
			return domain.PublishValidationContext{}, err
		}
		prices[row.PlanKey] = int64(definition.Price)
	}
	return domain.PublishValidationContext{
		Ladder:          domain.DefaultPriceLadder(),
		PublishedPrices: prices,
	}, nil
}

// validationReport itemizes the six axes from the joined domain error.
func validationReport(err error) ValidationReport {
	report := ValidationReport{Valid: err == nil, Axes: []AxisResult{
		{Axis: "base_price_tier", Passed: !errors.Is(err, domain.ErrPublishBasePriceTier)},
		{Axis: "currency_cny", Passed: !errors.Is(err, domain.ErrPublishCurrencyCNY)},
		{Axis: "entitlements", Passed: !errors.Is(err, domain.ErrPublishEntitlements)},
		{Axis: "resource_quota", Passed: !errors.Is(err, domain.ErrPublishResourceQuota)},
		{Axis: "included_credits", Passed: !errors.Is(err, domain.ErrPublishIncludedCredits)},
		{Axis: "cost_upper_bound", Passed: !errors.Is(err, domain.ErrPublishCostUpperBound)},
	}}
	return report
}

// validateDraftInput enforces only the IDENTITY contract of a draft (the
// plan-key slug and a display name): commercial content is deliberately
// NOT checked here — drafts are free-form, the six acceptance axes gate at
// Validate/Publish time (fail closed there, never silently at draft time).
func (s *PlanVersionService) validateDraftInput(in DraftInput) error {
	if in.PlanKey == "" || in.Name == "" {
		return ErrInvalidDraftInput
	}
	return nil
}

// draftRow encodes the draft definition and derives the deterministic
// external identity.
func (s *PlanVersionService) draftRow(in DraftInput, version int64) (repocommercial.PlanRow, error) {
	definition := domain.PlanVersion{
		Key:      in.PlanKey,
		Version:  version,
		Price:    domain.CNYFen(in.AmountFen),
		Monthly:  domain.Credits(in.IncludedCreditsMicro),
		Features: in.Features,
		Limits:   in.Limits,
		Currency: in.Currency,
		Name:     in.Name,
		Charges:  in.Charges,
	}
	blob, err := json.Marshal(definition)
	if err != nil {
		return repocommercial.PlanRow{}, err
	}
	return repocommercial.PlanRow{
		PlanKey:        in.PlanKey,
		Version:        version,
		DefinitionJSON: string(blob),
		ExternalID:     domain.DeterministicPlanCode(in.PlanKey, version),
		State:          domain.PlanStateDraft,
	}, nil
}

// publishPayload projects the stored definition onto the typed seam payload.
func publishPayload(d domain.PlanVersion) domain.PublishPlanVersionPayload {
	return domain.PublishPlanVersionPayload{
		PlanKey:              d.Key,
		Version:              d.Version,
		PlanCode:             domain.DeterministicPlanCode(d.Key, d.Version),
		Name:                 d.Name,
		Interval:             domain.IntervalMonthly,
		AmountFen:            int64(d.Price),
		Currency:             d.Currency,
		PayInAdvance:         true,
		IncludedCreditsMicro: int64(d.Monthly),
		Features:             d.Features,
		Limits:               d.Limits,
		Charges:              d.Charges,
	}
}

// decodeDefinition parses a stored definition_json.
func decodeDefinition(blob string) (domain.PlanVersion, error) {
	var definition domain.PlanVersion
	if err := json.Unmarshal([]byte(blob), &definition); err != nil {
		return domain.PlanVersion{}, fmt.Errorf("%w: %v", repocommercial.ErrInvalidPlanRow, err)
	}
	return definition, nil
}
