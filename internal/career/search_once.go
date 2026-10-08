package career

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Frozen one-shot search statuses, qualification values, uncertainty values,
// and bounded failure codes (T11). A search is never a continuous rule: every
// request ID runs exactly one durable search with an immutable result set.
const (
	SearchStatusCompleted = "completed"
	SearchStatusFailed    = "failed"
	searchStatusClaiming  = "claiming"

	SearchQualificationNeedsReview  = "needs_review"
	SearchQualificationQualified    = "qualified"
	SearchQualificationNotQualified = "not_qualified"

	SearchUncertaintyLowConfidence = "low_confidence"

	SearchFailureNoVettedSources       = "no_vetted_sources"
	SearchFailureAllSourcesUnavailable = "all_sources_unavailable"
	SearchFailureSourceMisconfigured   = "source_misconfigured"
	searchOnceKind                     = "search_once"
	searchOnceClaimKind                = "search_once_claim"
	maxSearchQueryBytes                = 512
	maxSearchResultsPerSource          = 50
	searchClaimLease                   = 60 * time.Second
	searchClaimWaitLimit               = 3 * time.Second
	searchClaimInterval                = 25 * time.Millisecond
)

var (
	ErrSearchNotFound     = errors.New("career search not found")
	ErrSearchQuotaRefused = errors.New("career search quota refused")
	errSearchClaimLost    = errors.New("career search claim lost")
	errSearchReceipt      = errors.New("unreadable career search receipt")
)

// SearchOnceInput is the frozen request body of the one-shot search. The
// client supplies the instruction, the request ID, and the expected profile
// revision it observed.
type SearchOnceInput struct {
	RequestID        string `json:"requestId"`
	Query            string `json:"query"`
	ExpectedRevision uint64 `json:"expectedRevision"`
}

// SearchSourceCoverage reports one source's honest availability, access
// method, and city coverage as actually configured and checked.
type SearchSourceCoverage struct {
	SourceID      string   `json:"sourceId"`
	Label         string   `json:"label"`
	AccessMethods []string `json:"accessMethods"`
	Cities        []string `json:"cities"`
	Available     bool     `json:"available"`
	FailureCode   string   `json:"failureCode,omitempty"`
}

// SearchCoverage is the truthful coverage listing carried by every search
// receipt. Production starts with an empty list because no source is vetted.
type SearchCoverage struct {
	Sources []SearchSourceCoverage `json:"sources"`
}

// SearchResultRow is one discovered job: check time, qualification, the
// original link, and an uncertainty annotation.
type SearchResultRow struct {
	ResultID      string    `json:"resultId"`
	SourceID      string    `json:"sourceId"`
	Link          string    `json:"link"`
	CheckedAt     time.Time `json:"checkedAt"`
	Qualification string    `json:"qualification"`
	Uncertainty   string    `json:"uncertainty"`
}

// SearchOnceReceipt is the durable terminal receipt of a one-shot search and
// the frozen contract view served by all three search endpoints.
type SearchOnceReceipt struct {
	Kind        string            `json:"kind"`
	RequestID   string            `json:"requestId"`
	SearchID    string            `json:"searchId"`
	Status      string            `json:"status"`
	Query       string            `json:"query"`
	Coverage    SearchCoverage    `json:"coverage"`
	ScopeNotes  []string          `json:"scopeNotes"`
	FailureCode string            `json:"failureCode,omitempty"`
	Results     []SearchResultRow `json:"results"`
	CheckedAt   time.Time         `json:"checkedAt"`
}

// VettedSearchSource describes one vetted source that can be searched today,
// with the honest coverage facts surfaced to the user. SearchURLTemplate is an
// absolute http(s) URL that may contain one {query} placeholder.
type VettedSearchSource struct {
	ID                string
	Label             string
	SearchURLTemplate string
	AccessMethods     []string
	Cities            []string
}

// SearchSourceRegistry enumerates the vetted searchable sources. The
// production registry starts empty: real source vetting is a separate,
// authorized review process and nothing here may invent a source.
type SearchSourceRegistry interface {
	SearchSources() []VettedSearchSource
}

type emptySearchSourceRegistry struct{}

func (emptySearchSourceRegistry) SearchSources() []VettedSearchSource { return nil }

// SearchQuotaGate decides whether one one-shot search is admitted. Since T21
// the production implementation is the real usage ledger (see usage.go): it
// reserves quota by request ID before execution and must never fabricate
// quota state. A refusal is a typed error and leaves the request ID fully
// recoverable. The request ID parameter is what makes admission idempotent —
// the same request replayed or retried never reserves or charges twice.
type SearchQuotaGate interface {
	AdmitSearch(ctx context.Context, scope Scope, requestID, query string) error
}

type passThroughSearchQuotaGate struct{}

func (passThroughSearchQuotaGate) AdmitSearch(context.Context, Scope, string, string) error {
	return nil
}

// searchRecord is the durable one-shot search. The row starts as a bounded
// claim before any network I/O and becomes terminal exactly once; the receipt
// body is stored verbatim so replays never refetch.
type searchRecord struct {
	ID          string `gorm:"primaryKey;size:36"`
	TenantID    uint64 `gorm:"uniqueIndex:career_search_scope_request;index:idx_career_search_scope"`
	UserID      string `gorm:"uniqueIndex:career_search_scope_request;index:idx_career_search_scope;size:512"`
	RequestID   string `gorm:"uniqueIndex:career_search_scope_request;size:128"`
	Fingerprint string `gorm:"size:64;not null"`
	Status      string `gorm:"size:16;not null;index:idx_career_search_status"`
	ClaimToken  string `gorm:"size:36;not null;default:''"`
	LeaseUntil  *time.Time
	Query       string `gorm:"size:512;not null"`
	ReceiptBody string `gorm:"type:text;not null"`
	CreatedAt   time.Time
	CompletedAt *time.Time
}

func (searchRecord) TableName() string { return "career_searches" }

// searchResultRecord is one immutable result row of a completed search.
type searchResultRecord struct {
	ID            string `gorm:"primaryKey;size:36"`
	TenantID      uint64 `gorm:"uniqueIndex:career_search_result_scope_link;index:idx_career_search_result_scope"`
	UserID        string `gorm:"uniqueIndex:career_search_result_scope_link;index:idx_career_search_result_scope;size:512"`
	SearchID      string `gorm:"uniqueIndex:career_search_result_scope_link;size:36;index"`
	SourceID      string `gorm:"size:64;not null;default:''"`
	Link          string `gorm:"uniqueIndex:career_search_result_scope_link;size:2048;not null"`
	CheckedAt     time.Time
	Qualification string `gorm:"size:32;not null"`
	Uncertainty   string `gorm:"size:32;not null"`
	CreatedAt     time.Time
}

func (searchResultRecord) TableName() string { return "career_search_results" }

type searchClaimBody struct {
	Kind       string    `json:"kind"`
	ClaimToken string    `json:"claimToken"`
	LeaseUntil time.Time `json:"leaseUntil"`
}

type searchClaimState int

const (
	searchClaimProceed searchClaimState = iota
	searchClaimTerminal
	searchClaimInFlight
)

type searchClaimOutcome struct {
	state   searchClaimState
	receipt SearchOnceReceipt
	token   string
}

// searchQualificationFromEvaluationStatus maps the frozen T10 evaluation
// statuses onto a search row's qualification. Missing or unknown evaluations
// stay needs_review: a search result must never fabricate a conclusion.
func searchQualificationFromEvaluationStatus(status string) string {
	switch status {
	case EvaluationEligible:
		return SearchQualificationQualified
	case EvaluationIneligible:
		return SearchQualificationNotQualified
	default:
		return SearchQualificationNeedsReview
	}
}

// SearchOnce runs exactly one durable search for a request ID. All network
// I/O happens outside any database transaction; the claim/receipt pair keeps
// concurrent and unknown outcomes recoverable under the original request ID.
// It never creates any continuous rule structure.
func (o *Office) SearchOnce(ctx context.Context, input SearchOnceInput) (SearchOnceReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return SearchOnceReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return SearchOnceReceipt{}, err
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.Query = strings.TrimSpace(input.Query)
	if input.RequestID == "" || len(input.RequestID) > 128 || input.Query == "" || len(input.Query) > maxSearchQueryBytes {
		return SearchOnceReceipt{}, ErrInvalidRequest
	}
	fingerprint, err := searchFingerprint(input)
	if err != nil {
		return SearchOnceReceipt{}, err
	}
	// Exact replay must never be blocked by the quota gate, so an existing
	// terminal receipt is answered before admission is consulted.
	if replay, found, lookupErr := o.replaySearch(ctx, s, input.RequestID, fingerprint); lookupErr != nil {
		return SearchOnceReceipt{}, lookupErr
	} else if found {
		return replay, nil
	}
	// Quota admission is a narrow injected seam. A refusal is typed and leaves
	// no durable state, so the same request ID can be replayed later.
	if o.searchQuotaGate == nil {
		o.searchQuotaGate = passThroughSearchQuotaGate{}
	}
	if err = o.searchQuotaGate.AdmitSearch(ctx, s, input.RequestID, input.Query); err != nil {
		return SearchOnceReceipt{}, err
	}

	claim, err := o.claimSearchRequest(ctx, s, input, fingerprint)
	if err != nil {
		return SearchOnceReceipt{}, err
	}
	switch claim.state {
	case searchClaimTerminal:
		return claim.receipt, nil
	case searchClaimInFlight:
		receipt, found, awaitErr := o.awaitSearchTerminal(ctx, s, input.RequestID, fingerprint, searchClaimWaitLimit)
		if awaitErr != nil {
			return SearchOnceReceipt{}, awaitErr
		}
		if found {
			return receipt, nil
		}
		return SearchOnceReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
	}

	// Network I/O starts here, strictly outside any database transaction and
	// only through the existing SourcePolicy/SourceTransport seams.
	receipt := o.executeSearch(ctx, s, input)
	return o.commitSearch(ctx, s, input, fingerprint, claim.token, receipt)
}

func searchFingerprint(input SearchOnceInput) (string, error) {
	intentBytes, err := json.Marshal([]any{searchOnceKind, input.RequestID, input.Query, input.ExpectedRevision})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(intentBytes)
	return hex.EncodeToString(sum[:]), nil
}

// searchOnceUnderStoredFingerprint replays or takes over a durable search
// under the fingerprint already stored for its request ID (ocr3-017). The
// rule trigger seam needs it: its request ID is deterministic per period
// while the profile revision legitimately moves with every write, so a
// claiming row left by a crashed attempt would answer every later retry
// with ErrIdempotencyConflict and strand that rule period (and with it the
// whole trigger) forever. The stored fingerprint is the durable identity of
// the search and the stored query is its intent, so the retry converges on
// exactly what was first claimed. Quota admission is not repeated: the
// first claim already consumed it, and an exact replay must never be
// blocked by the gate.
func (o *Office) searchOnceUnderStoredFingerprint(ctx context.Context, s Scope, requestID string) (SearchOnceReceipt, error) {
	var row searchRecord
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SearchOnceReceipt{}, ErrIdempotencyConflict
	}
	if err != nil {
		return SearchOnceReceipt{}, err
	}
	input := SearchOnceInput{RequestID: requestID, Query: row.Query}
	claim, err := o.claimSearchRequest(ctx, s, input, row.Fingerprint)
	if err != nil {
		return SearchOnceReceipt{}, err
	}
	switch claim.state {
	case searchClaimTerminal:
		return claim.receipt, nil
	case searchClaimInFlight:
		receipt, found, awaitErr := o.awaitSearchTerminal(ctx, s, requestID, row.Fingerprint, searchClaimWaitLimit)
		if awaitErr != nil {
			return SearchOnceReceipt{}, awaitErr
		}
		if found {
			return receipt, nil
		}
		return SearchOnceReceipt{}, &OutcomeUnknownError{RequestID: requestID}
	}
	receipt := o.executeSearch(ctx, s, input)
	return o.commitSearch(ctx, s, input, row.Fingerprint, claim.token, receipt)
}

// claimSearchRequest durably claims the scoped request before any network
// I/O, checking the expected revision on first claim. Concurrent identical
// claims resolve to one owner; the losers wait for the terminal receipt.
func (o *Office) claimSearchRequest(ctx context.Context, s Scope, input SearchOnceInput, fingerprint string) (searchClaimOutcome, error) {
	outcome := searchClaimOutcome{}
	token := uuid.NewString()
	claimJSON := func() (string, error) {
		body, err := json.Marshal(searchClaimBody{Kind: searchOnceClaimKind, ClaimToken: token, LeaseUntil: time.Now().UTC().Add(searchClaimLease)})
		return string(body), err
	}
	register := func(tx *gorm.DB) error {
		var existing searchRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
			First(&existing).Error
		if err == nil {
			if existing.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			receipt, claim, decodeErr := decodeSearchBody(existing.ReceiptBody)
			if decodeErr != nil {
				return decodeErr
			}
			if receipt != nil {
				outcome.state, outcome.receipt = searchClaimTerminal, *receipt
				return nil
			}
			// The lease column is the durable truth; the JSON lease is only a
			// projection for readers. An expired lease is taken over so a
			// crashed fetch cannot strand the request ID forever.
			if claim.ClaimToken != "" && existing.LeaseUntil != nil && existing.LeaseUntil.After(time.Now().UTC()) {
				outcome.state = searchClaimInFlight
				return nil
			}
			// An expired claim is taken over so a crashed fetch cannot strand
			// the request ID forever.
			takeover, err := claimJSON()
			if err != nil {
				return err
			}
			leaseUntil := time.Now().UTC().Add(searchClaimLease)
			if err = tx.Model(&searchRecord{}).
				Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
				Updates(map[string]any{"receipt_body": takeover, "claim_token": token, "lease_until": leaseUntil}).Error; err != nil {
				return err
			}
			outcome.state, outcome.token = searchClaimProceed, token
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := requireGateActiveTx(tx, s); err != nil {
			return err
		}
		// The expected revision pins the caller's observed profile view. A
		// search never mutates the profile, so no revision is advanced.
		var head profile
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			head.Revision = 0
		} else if err != nil {
			return err
		}
		if head.Revision != input.ExpectedRevision {
			return &RevisionConflictError{CurrentRevision: head.Revision}
		}
		claimBody, err := claimJSON()
		if err != nil {
			return err
		}
		leaseUntil := time.Now().UTC().Add(searchClaimLease)
		err = tx.Create(&searchRecord{
			ID: uuid.NewString(), TenantID: s.TenantID, UserID: s.UserID,
			RequestID: input.RequestID, Fingerprint: fingerprint, Status: searchStatusClaiming,
			ClaimToken: token, LeaseUntil: &leaseUntil, Query: input.Query,
			ReceiptBody: claimBody, CreatedAt: time.Now().UTC(),
		}).Error
		if err != nil {
			if isSQLiteBusy(err) {
				return err
			}
			if isReceiptRaceError(err) {
				outcome.state = searchClaimInFlight
				return nil
			}
			return err
		}
		outcome.state, outcome.token = searchClaimProceed, token
		return nil
	}
	err := o.runImportTransaction(ctx, register)
	if err != nil {
		if isSQLiteBusy(err) {
			return searchClaimOutcome{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return searchClaimOutcome{}, err
	}
	return outcome, nil
}

func decodeSearchBody(body string) (*SearchOnceReceipt, searchClaimBody, error) {
	var claim searchClaimBody
	if err := json.Unmarshal([]byte(body), &claim); err != nil {
		return nil, searchClaimBody{}, errSearchReceipt
	}
	switch claim.Kind {
	case searchOnceKind:
		var receipt SearchOnceReceipt
		if err := json.Unmarshal([]byte(body), &receipt); err != nil {
			return nil, searchClaimBody{}, errSearchReceipt
		}
		return &receipt, searchClaimBody{}, nil
	case searchOnceClaimKind:
		return nil, claim, nil
	default:
		return nil, searchClaimBody{}, errSearchReceipt
	}
}

// executeSearch performs the one-shot search through the existing source
// seams. Results only contain postings whose link literally appeared in
// fetched text; nothing is ever fabricated.
func (o *Office) executeSearch(ctx context.Context, s Scope, input SearchOnceInput) SearchOnceReceipt {
	sources := []VettedSearchSource{}
	if o.searchRegistry != nil {
		sources = o.searchRegistry.SearchSources()
	}
	checkedAt := time.Now().UTC()
	receipt := SearchOnceReceipt{
		Kind: searchOnceKind, RequestID: input.RequestID, SearchID: uuid.NewString(),
		Query: input.Query, Coverage: SearchCoverage{Sources: []SearchSourceCoverage{}},
		ScopeNotes: []string{}, Results: []SearchResultRow{}, CheckedAt: checkedAt,
	}
	if len(sources) == 0 {
		receipt.Status = SearchStatusFailed
		receipt.FailureCode = SearchFailureNoVettedSources
		receipt.ScopeNotes = append(receipt.ScopeNotes,
			"no vetted search source is configured; nothing was fetched and no result is fabricated",
			"coverage reflects only actually vetted sources")
		return receipt
	}
	policy := o.sourcePolicy
	if policy == nil {
		policy = emptySourcePolicy{}
	}
	rows := []SearchResultRow{}
	anyAvailable := false
	for _, source := range sources {
		coverage := SearchSourceCoverage{
			SourceID: source.ID, Label: source.Label,
			AccessMethods: append([]string{}, source.AccessMethods...),
			Cities:        append([]string{}, source.Cities...),
			Available:     false,
		}
		searchURL, urlErr := buildSearchURL(source, input.Query)
		if urlErr != nil {
			coverage.FailureCode = SearchFailureSourceMisconfigured
			receipt.ScopeNotes = append(receipt.ScopeNotes,
				fmt.Sprintf("source %s is misconfigured and was not searched", source.ID))
			receipt.Coverage.Sources = append(receipt.Coverage.Sources, coverage)
			continue
		}
		approved, verifyErr := policy.Verify(searchURL)
		if verifyErr != nil {
			coverage.FailureCode = FailureSourceUnverified
			receipt.Coverage.Sources = append(receipt.Coverage.Sources, coverage)
			continue
		}
		result, fetchErr := o.sourceTransport.Fetch(ctx, approved, searchURL)
		if fetchErr != nil {
			code := FailureNetworkError
			var fetchFailure *SourceFetchError
			if errors.As(fetchErr, &fetchFailure) {
				code = boundedSourceFailureCode(fetchFailure.Code)
			}
			coverage.FailureCode = code
			receipt.ScopeNotes = append(receipt.ScopeNotes,
				fmt.Sprintf("source %s was unavailable during this search (failure code %s); no result was substituted", source.ID, code))
			receipt.Coverage.Sources = append(receipt.Coverage.Sources, coverage)
			continue
		}
		coverage.Available = true
		receipt.Coverage.Sources = append(receipt.Coverage.Sources, coverage)
		anyAvailable = true
		parsed, truncated := parseSearchPostings(source.ID, result.Text, checkedAt)
		if truncated {
			receipt.ScopeNotes = append(receipt.ScopeNotes,
				fmt.Sprintf("source %s returned more entries than recorded; the listing was truncated", source.ID))
		}
		for _, row := range parsed {
			rows = append(rows, SearchResultRow{
				ResultID: row.ID, SourceID: row.SourceID, Link: row.Link,
				CheckedAt: row.CheckedAt, Qualification: row.Qualification, Uncertainty: row.Uncertainty,
			})
		}
	}
	if !anyAvailable {
		receipt.Status = SearchStatusFailed
		receipt.FailureCode = SearchFailureAllSourcesUnavailable
		receipt.ScopeNotes = append(receipt.ScopeNotes,
			"no vetted source could be searched in this run; retry with the same request ID to recover")
		return receipt
	}
	receipt.Status = SearchStatusCompleted
	// The same job link can surface through several sources; one search stores
	// one row per link, credited to the source that surfaced it first.
	deduped := rows[:0]
	seenLinks := map[string]bool{}
	for _, row := range rows {
		if seenLinks[row.Link] {
			continue
		}
		seenLinks[row.Link] = true
		deduped = append(deduped, row)
	}
	receipt.Results = deduped
	return receipt
}

// buildSearchURL expands the {query} placeholder of a vetted source template.
func buildSearchURL(source VettedSearchSource, query string) (string, error) {
	template := strings.TrimSpace(source.SearchURLTemplate)
	if template == "" || len(template) > maxSourceURLBytes {
		return "", errors.New("empty or oversized search template")
	}
	built := strings.ReplaceAll(template, "{query}", url.QueryEscape(query))
	parsed, err := url.Parse(built)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return "", errors.New("search template must be an absolute http(s) URL")
	}
	return built, nil
}

// parseSearchPostings extracts honest postings from fetched listing text.
// Only whitespace-delimited tokens that parse as absolute http(s) URLs become
// rows — the link was literally present in what the source returned.
func parseSearchPostings(sourceID, text string, checkedAt time.Time) (rows []searchResultRecord, truncated bool) {
	seen := map[string]bool{}
	for _, field := range strings.Fields(text) {
		if len(rows) >= maxSearchResultsPerSource {
			truncated = true
			break
		}
		if !strings.HasPrefix(field, "http://") && !strings.HasPrefix(field, "https://") {
			continue
		}
		parsed, err := url.Parse(field)
		if err != nil || parsed.Hostname() == "" || len(field) > maxSourceURLBytes {
			continue
		}
		if seen[field] {
			continue
		}
		seen[field] = true
		rows = append(rows, searchResultRecord{
			ID: uuid.NewString(), SourceID: sourceID, Link: field,
			CheckedAt: checkedAt, Qualification: searchQualificationFromEvaluationStatus(""),
			Uncertainty: SearchUncertaintyLowConfidence,
		})
	}
	return rows, truncated
}

// commitSearch atomically persists the result rows and the terminal receipt.
func (o *Office) commitSearch(ctx context.Context, s Scope, input SearchOnceInput, fingerprint, claimToken string, receipt SearchOnceReceipt) (SearchOnceReceipt, error) {
	var stored SearchOnceReceipt
	err := o.runImportTransaction(ctx, func(tx *gorm.DB) error {
		var row searchRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
			First(&row).Error; err != nil {
			return err
		}
		if row.Fingerprint != fingerprint {
			return ErrIdempotencyConflict
		}
		terminal, claim, decodeErr := decodeSearchBody(row.ReceiptBody)
		if decodeErr != nil {
			return decodeErr
		}
		if terminal != nil {
			// A concurrent reconcile already committed the terminal receipt.
			stored = *terminal
			return nil
		}
		if claim.ClaimToken == "" || claim.ClaimToken != claimToken {
			return errSearchClaimLost
		}
		now := time.Now().UTC()
		for i := range receipt.Results {
			record := searchResultRecord{
				ID: receipt.Results[i].ResultID, TenantID: s.TenantID, UserID: s.UserID,
				SearchID: receipt.SearchID, SourceID: receipt.Results[i].SourceID,
				Link: receipt.Results[i].Link, CheckedAt: receipt.Results[i].CheckedAt,
				Qualification: receipt.Results[i].Qualification, Uncertainty: receipt.Results[i].Uncertainty,
				CreatedAt: now,
			}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		}
		body, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		if o.failSearchTerminalCommit != nil {
			if hookErr := o.failSearchTerminalCommit(); hookErr != nil {
				return hookErr
			}
		}
		return tx.Model(&searchRecord{}).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
			Updates(map[string]any{
				"id": receipt.SearchID, "status": receipt.Status, "claim_token": "",
				"lease_until": nil, "receipt_body": string(body), "completed_at": now,
			}).Error
	})
	if err == nil {
		if stored.Kind == searchOnceKind && stored.SearchID != "" {
			// A concurrent reconcile committed the terminal receipt first.
			return stored, nil
		}
		return receipt, nil
	}
	if errors.Is(err, errSearchClaimLost) || isSQLiteBusy(err) {
		// Our claim was taken over after a lease expiry, or writer-lock
		// contention exhausted the retry budget. Reconcile once with the
		// original request ID instead of guessing the outcome.
		replay, found, lookupErr := o.awaitSearchTerminal(context.WithoutCancel(ctx), s, input.RequestID, fingerprint, 0)
		if lookupErr != nil {
			return SearchOnceReceipt{}, lookupErr
		}
		if found {
			return replay, nil
		}
		return SearchOnceReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
	}
	return SearchOnceReceipt{}, err
}

// awaitSearchTerminal polls the durable search row until it turns terminal.
// A maxWait of zero performs exactly one bounded read.
func (o *Office) awaitSearchTerminal(ctx context.Context, s Scope, requestID, fingerprint string, maxWait time.Duration) (SearchOnceReceipt, bool, error) {
	deadline := time.Now().Add(maxWait)
	for {
		var row searchRecord
		err := o.db.WithContext(ctx).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
			First(&row).Error
		if err == nil {
			if row.Fingerprint != fingerprint {
				return SearchOnceReceipt{}, false, ErrIdempotencyConflict
			}
			receipt, _, decodeErr := decodeSearchBody(row.ReceiptBody)
			if decodeErr != nil {
				return SearchOnceReceipt{}, false, decodeErr
			}
			if receipt != nil {
				return *receipt, true, nil
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) && !isSQLiteBusy(err) {
			return SearchOnceReceipt{}, false, err
		}
		if time.Now().After(deadline) {
			return SearchOnceReceipt{}, false, nil
		}
		select {
		case <-ctx.Done():
			return SearchOnceReceipt{}, false, &OutcomeUnknownError{RequestID: requestID}
		case <-time.After(searchClaimInterval):
		}
	}
}

func (o *Office) replaySearch(ctx context.Context, s Scope, requestID, fingerprint string) (SearchOnceReceipt, bool, error) {
	var row searchRecord
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SearchOnceReceipt{}, false, nil
	}
	if err != nil {
		return SearchOnceReceipt{}, false, err
	}
	if row.Fingerprint != fingerprint {
		return SearchOnceReceipt{}, true, ErrIdempotencyConflict
	}
	receipt, _, decodeErr := decodeSearchBody(row.ReceiptBody)
	if decodeErr != nil {
		return SearchOnceReceipt{}, true, decodeErr
	}
	if receipt == nil {
		// A live claim is not a terminal receipt; fall through to admission
		// and the claim path so in-flight coordination stays in one place.
		return SearchOnceReceipt{}, false, nil
	}
	return *receipt, true, nil
}

// FindSearchReceipt replays the terminal receipt of a one-shot search by its
// original request ID under the authenticated scope.
func (o *Office) FindSearchReceipt(ctx context.Context, requestID string) (SearchOnceReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return SearchOnceReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return SearchOnceReceipt{}, err
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 128 {
		return SearchOnceReceipt{}, ErrInvalidRequest
	}
	var row searchRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SearchOnceReceipt{}, ErrSearchNotFound
	}
	if err != nil {
		return SearchOnceReceipt{}, err
	}
	receipt, _, decodeErr := decodeSearchBody(row.ReceiptBody)
	if decodeErr != nil {
		return SearchOnceReceipt{}, decodeErr
	}
	if receipt == nil {
		return SearchOnceReceipt{}, ErrSearchNotFound
	}
	return *receipt, nil
}

// Search returns the stored terminal receipt of one durable search by its ID.
// Cross-scope lookups fail as not found without leaking existence.
func (o *Office) Search(ctx context.Context, searchID string) (SearchOnceReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return SearchOnceReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return SearchOnceReceipt{}, err
	}
	searchID = strings.TrimSpace(searchID)
	if searchID == "" || len(searchID) > 36 {
		return SearchOnceReceipt{}, ErrInvalidRequest
	}
	var row searchRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, searchID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SearchOnceReceipt{}, ErrSearchNotFound
	}
	if err != nil {
		return SearchOnceReceipt{}, err
	}
	receipt, _, decodeErr := decodeSearchBody(row.ReceiptBody)
	if decodeErr != nil {
		return SearchOnceReceipt{}, decodeErr
	}
	if receipt == nil {
		return SearchOnceReceipt{}, ErrSearchNotFound
	}
	return *receipt, nil
}
