package career

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrSourceNotFound = errors.New("career source not found")

type SourceUpload struct {
	ID               string
	FileName         string
	MIMEType         string
	Size             int64
	Digest           string
	ResourceRef      string
	Text             string
	RequestID        string
	IntentHash       string
	ExpectedRevision uint64
}

var ErrUploadInProgress = errors.New("career upload is already processing")
var ErrUploadClaimLost = errors.New("career upload claim was superseded")

type StaleSourceResource struct {
	ID                 string
	RequestID          string
	IntentHash         string
	ResourceRef        string
	ClaimToken         string
	FinalErrorCategory string
}

type UploadClaimIntent struct {
	FileName         string
	MIMEType         string
	Digest           string
	ExpectedRevision uint64
}

func isTerminalUploadSource(row sourceRevision) bool {
	if row.Status == "ready" {
		return true
	}
	return row.Status == "failed" && !(row.ResourceRef == "" && row.ErrorCategory == "interrupted")
}

func (o *Office) FindUploadClaim(ctx context.Context, requestID string) (UploadClaimIntent, bool, error) {
	s, err := getScope(ctx)
	if err != nil {
		return UploadClaimIntent{}, false, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return UploadClaimIntent{}, false, err
	}
	var row sourceRevision
	err = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return UploadClaimIntent{}, false, nil
	}
	if err != nil {
		return UploadClaimIntent{}, false, err
	}
	return UploadClaimIntent{FileName: row.FileName, MIMEType: row.MIMEType, Digest: row.Digest, ExpectedRevision: row.ExpectedRevision}, true, nil
}

// FindUploadSource performs a read-only lookup for a request that may already
// be executing under another lifecycle guard.
func (o *Office) FindUploadSource(ctx context.Context, requestID string) (CareerSource, bool, error) {
	s, err := getScope(ctx)
	if err != nil {
		return CareerSource{}, false, err
	}
	var row sourceRevision
	err = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CareerSource{}, false, nil
	}
	if err != nil {
		return CareerSource{}, false, err
	}
	return sourceView(row), true, nil
}

// FailStaleUploads turns abandoned processing claims into visible failures.
// Resource references remain until the caller has successfully released them.
func (o *Office) FailStaleUploads(ctx context.Context, cutoff time.Time, excludeRequestID ...string) ([]StaleSourceResource, error) {
	s, err := getScope(ctx)
	if err != nil {
		return nil, err
	}
	var rows []sourceRevision
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		leaseDeadlineSQL := "lease_until <= ?"
		if tx.Dialector.Name() == "sqlite" {
			leaseDeadlineSQL = "julianday(lease_until) <= julianday(?)"
		}
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=? AND ((status='processing' AND lease_until IS NOT NULL AND "+leaseDeadlineSQL+") OR (status='failed' AND error_category LIKE 'cleanup_pending_%' AND resource_ref <> ''))", s.TenantID, s.UserID, cutoff)
		if len(excludeRequestID) > 0 && excludeRequestID[0] != "" {
			query = query.Where("request_id <> ?", excludeRequestID[0])
		}
		if e := query.Find(&rows).Error; e != nil {
			return e
		}
		for i := range rows {
			r := &rows[i]
			if r.Status == "processing" {
				// With no durable locator, a processing lease may represent a
				// provider write whose response was lost. Keep it retryable and
				// leave its lifecycle claim in place; a same-request retry will
				// overwrite/reconcile the deterministic physical key.
				if r.ResourceRef == "" {
					continue
				}
				category := "interrupted"
				if r.ResourceRef != "" {
					category = "cleanup_pending_interrupted"
				}
				newToken := uuid.NewString()
				res := tx.Model(&sourceRevision{}).Where("tenant_id=? AND user_id=? AND id=? AND status='processing' AND claim_token=? AND "+leaseDeadlineSQL, s.TenantID, s.UserID, r.ID, r.ClaimToken, cutoff).Updates(map[string]any{"status": "failed", "claim_token": newToken, "error_category": category, "error_message": "Resume processing was interrupted. Upload again or enter profile details manually.", "extracted_text": "", "lease_until": nil})
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected == 0 {
					r.Status = "superseded"
					continue
				}
				r.Status = "failed"
				r.ClaimToken = newToken
				r.ErrorCategory = category
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]StaleSourceResource, 0, len(rows))
	for _, r := range rows {
		if r.Status == "superseded" || r.ResourceRef == "" {
			continue
		}
		final := strings.TrimPrefix(r.ErrorCategory, "cleanup_pending_")
		out = append(out, StaleSourceResource{ID: r.ID, RequestID: r.RequestID, IntentHash: r.IntentHash, ResourceRef: r.ResourceRef, ClaimToken: r.ClaimToken, FinalErrorCategory: final})
	}
	return out, nil
}

func (o *Office) ClearSourceResource(ctx context.Context, id, token, ref, finalCategory string) error {
	s, e := getScope(ctx)
	if e != nil {
		return e
	}
	if finalCategory == "interrupted" {
		finalCategory = "interrupted_cleaned"
	}
	res := o.db.WithContext(ctx).Model(&sourceRevision{}).Where("tenant_id=? AND user_id=? AND id=? AND status='failed' AND claim_token=? AND resource_ref=? AND error_category LIKE 'cleanup_pending_%'", s.TenantID, s.UserID, id, token, ref).Updates(map[string]any{"resource_ref": "", "error_category": finalCategory})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrUploadClaimLost
	}
	return nil
}

func (o *Office) privateSource(ctx context.Context, id string) (sourceRevision, error) {
	s, e := getScope(ctx)
	if e != nil {
		return sourceRevision{}, e
	}
	var row sourceRevision
	e = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, id).First(&row).Error
	return row, e
}

// ClaimUpload durably binds a scoped request ID to one canonical upload intent
// before storage begins. A stale lease can be resumed with the same source ID.
func (o *Office) ClaimUpload(ctx context.Context, upload SourceUpload) (CareerSource, bool, error) {
	s, err := getScope(ctx)
	if err != nil {
		return CareerSource{}, false, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return CareerSource{}, false, err
	}
	if strings.TrimSpace(upload.RequestID) == "" || strings.TrimSpace(upload.IntentHash) == "" || upload.Size <= 0 || upload.Digest == "" || strings.TrimSpace(upload.FileName) == "" {
		return CareerSource{}, false, ErrInvalidRequest
	}
	var row sourceRevision
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var prior sourceRevision
		e := tx.Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, upload.RequestID).First(&prior).Error
		if e == nil {
			if prior.IntentHash != upload.IntentHash || prior.Digest != upload.Digest || prior.ExpectedRevision != upload.ExpectedRevision {
				return ErrIdempotencyConflict
			}
			if isTerminalUploadSource(prior) {
				row = prior
				return nil
			}
			// Uploads can create private file/catalog state. Admit the scoped
			// request in the same transaction as its active claim, before any
			// retry can invoke SaveBytes or Bind.
			if e = admitLifecycleClaimTx(tx, s, "source_upload", upload.RequestID, upload.IntentHash); e != nil {
				return e
			}
			now := time.Now().UTC()
			if prior.Status == "failed" && prior.ResourceRef == "" && prior.ErrorCategory == "interrupted" {
				newToken := uuid.NewString()
				newLease := now.Add(30 * time.Minute)
				res := tx.Model(&sourceRevision{}).Where("tenant_id=? AND user_id=? AND id=? AND status='failed' AND resource_ref='' AND error_category='interrupted'", s.TenantID, s.UserID, prior.ID).Updates(map[string]any{"status": "processing", "claim_token": newToken, "lease_until": newLease, "error_category": "", "error_message": ""})
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected != 1 {
					return ErrUploadInProgress
				}
				prior.Status = "processing"
				prior.ClaimToken = newToken
				prior.LeaseUntil = &newLease
				prior.ErrorCategory = ""
				prior.ErrorMessage = ""
				row = prior
				return nil
			}
			if prior.LeaseUntil != nil && prior.LeaseUntil.After(now) {
				row = prior
				return ErrUploadInProgress
			}
			leaseDeadlineSQL := "lease_until <= ?"
			if tx.Dialector.Name() == "sqlite" {
				leaseDeadlineSQL = "julianday(lease_until) <= julianday(?)"
			}
			newToken := uuid.NewString()
			newLease := now.Add(30 * time.Minute)
			res := tx.Model(&sourceRevision{}).Where("tenant_id=? AND user_id=? AND id=? AND status='processing' AND claim_token=? AND lease_until IS NOT NULL AND "+leaseDeadlineSQL, s.TenantID, s.UserID, prior.ID, prior.ClaimToken, now).Updates(map[string]any{"claim_token": newToken, "lease_until": newLease})
			if e = res.Error; e != nil {
				return e
			}
			if res.RowsAffected != 1 {
				return ErrUploadInProgress
			}
			prior.ClaimToken = newToken
			prior.LeaseUntil = &newLease
			row = prior
			return nil
		} else if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if e = admitLifecycleClaimTx(tx, s, "source_upload", upload.RequestID, upload.IntentHash); e != nil {
			return e
		}
		p := profile{TenantID: s.TenantID, UserID: s.UserID}
		if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&p).Error; e != nil {
			return e
		}
		if tx.Dialector.Name() == "postgres" {
			if e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&p).Error; e != nil {
				return e
			}
		}
		var last uint64
		if e = tx.Model(&sourceRevision{}).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Select("COALESCE(MAX(revision),0)").Scan(&last).Error; e != nil {
			return e
		}
		now := time.Now().UTC()
		row = sourceRevision{ID: uuid.NewString(), TenantID: s.TenantID, UserID: s.UserID, Revision: last + 1, FileName: upload.FileName, MIMEType: upload.MIMEType, Size: upload.Size, Digest: upload.Digest, RequestID: upload.RequestID, IntentHash: upload.IntentHash, ExpectedRevision: upload.ExpectedRevision, ClaimToken: uuid.NewString(), LeaseUntil: &[]time.Time{now.Add(30 * time.Minute)}[0], Status: "processing"}
		return tx.Create(&row).Error
	})
	if err != nil {
		var prior sourceRevision
		if lookupErr := o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, upload.RequestID).First(&prior).Error; lookupErr == nil {
			if prior.IntentHash != upload.IntentHash || prior.Digest != upload.Digest || prior.ExpectedRevision != upload.ExpectedRevision {
				return CareerSource{}, false, ErrIdempotencyConflict
			}
			if isTerminalUploadSource(prior) {
				return sourceView(prior), true, nil
			}
			if prior.Status == "failed" && prior.ResourceRef == "" && prior.ErrorCategory == "interrupted" {
				return sourceView(prior), false, &OutcomeUnknownError{RequestID: upload.RequestID}
			}
			if prior.LeaseUntil != nil && prior.LeaseUntil.After(time.Now().UTC()) {
				return sourceView(prior), false, ErrUploadInProgress
			}
			return sourceView(prior), false, ErrUploadInProgress
		}
		return sourceView(row), false, err
	}
	return sourceView(row), isTerminalUploadSource(row), nil
}

func (o *Office) PersistUploadResource(ctx context.Context, sourceID, token, ref string) error {
	return o.persistUploadResource(ctx, sourceID, token, ref, "")
}

func (o *Office) PersistUploadResourceOwned(ctx context.Context, sourceID, uploadToken, ownerToken, ref string) error {
	return o.persistUploadResource(ctx, sourceID, uploadToken, ref, ownerToken)
}

func (o *Office) persistUploadResource(ctx context.Context, sourceID, token, ref, ownerToken string) error {
	s, err := getScope(ctx)
	if err != nil {
		return err
	}
	res := o.db.WithContext(ctx).Model(&sourceRevision{}).Where("tenant_id=? AND user_id=? AND id=? AND status='processing' AND claim_token=?", s.TenantID, s.UserID, sourceID, token).Updates(map[string]any{"resource_ref": ref, "lease_until": time.Now().UTC().Add(30 * time.Minute)})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrUploadClaimLost
	}
	var requestID string
	if err := o.db.WithContext(ctx).Model(&sourceRevision{}).Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, sourceID).Select("request_id").Scan(&requestID).Error; err != nil {
		return err
	}
	if requestID == "" {
		return ErrUploadClaimLost
	}
	if ownerToken != "" {
		return nil
	}
	return o.resolveLifecycleClaim(ctx, s, "source_upload", requestID)
}

// CareerSource intentionally omits the private resource handle and extracted text.
type CareerSource struct {
	ID                string     `json:"id"`
	Revision          uint64     `json:"revision"`
	FileName          string     `json:"fileName"`
	MIMEType          string     `json:"mimeType"`
	Size              int64      `json:"size"`
	Digest            string     `json:"digest"`
	Status            string     `json:"status"`
	ErrorCategory     string     `json:"errorCategory,omitempty"`
	ErrorMessage      string     `json:"errorMessage,omitempty"`
	MissingCategories []string   `json:"missingCategories,omitempty"`
	ReviewFlags       []string   `json:"reviewFlags,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	CompletedAt       *time.Time `json:"completedAt,omitempty"`
	ClaimToken        string     `json:"-"`
	ResourceRef       string     `json:"-"`
}

type ExtractedField struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Evidence string `json:"evidence,omitempty"`
}

type UploadResponse struct {
	Source  CareerSource `json:"source"`
	Receipt *Receipt     `json:"receipt,omitempty"`
}

func sourceView(row sourceRevision) CareerSource {
	var missing, flags []string
	_ = json.Unmarshal([]byte(row.MissingCategories), &missing)
	_ = json.Unmarshal([]byte(row.ReviewFlags), &flags)
	return CareerSource{ID: row.ID, Revision: row.Revision, FileName: row.FileName, MIMEType: row.MIMEType, Size: row.Size, Digest: row.Digest, Status: row.Status, ErrorCategory: row.ErrorCategory, ErrorMessage: row.ErrorMessage, MissingCategories: missing, ReviewFlags: flags, CreatedAt: row.CreatedAt, CompletedAt: row.CompletedAt, ClaimToken: row.ClaimToken, ResourceRef: row.ResourceRef}
}

func (o *Office) CreateSource(ctx context.Context, upload SourceUpload, failure string) (CareerSource, error) {
	s, err := getScope(ctx)
	if err != nil {
		return CareerSource{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return CareerSource{}, err
	}
	name := strings.TrimSpace(upload.FileName)
	if name == "" || upload.Size < 0 || strings.TrimSpace(upload.Digest) == "" {
		return CareerSource{}, ErrInvalidRequest
	}
	if failure == "" && (upload.Size == 0 || strings.TrimSpace(upload.ResourceRef) == "" || strings.TrimSpace(upload.Text) == "") {
		return CareerSource{}, ErrInvalidRequest
	}
	id := strings.TrimSpace(upload.ID)
	if id == "" {
		id = uuid.NewString()
	}
	row := sourceRevision{ID: id, TenantID: s.TenantID, UserID: s.UserID, FileName: name, MIMEType: strings.TrimSpace(upload.MIMEType), Size: upload.Size, Digest: upload.Digest, ResourceRef: upload.ResourceRef, ExtractedText: upload.Text, Status: "ready"}
	if failure != "" {
		row.Status = "failed"
		row.ErrorCategory = "parse_failed"
		row.ErrorMessage = "Resume parsing failed. Retry the upload or enter profile details manually."
		row.ResourceRef = ""
		row.ExtractedText = ""
	}
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		p := profile{TenantID: s.TenantID, UserID: s.UserID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&p).Error; err != nil {
			return err
		}
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&p).Error; err != nil {
				return err
			}
		}
		var last uint64
		if err := tx.Model(&sourceRevision{}).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Select("COALESCE(MAX(revision), 0)").Scan(&last).Error; err != nil {
			return err
		}
		row.Revision = last + 1
		return tx.Create(&row).Error
	})
	if err != nil {
		return CareerSource{}, err
	}
	return sourceView(row), nil
}

func (o *Office) CreateProcessingSource(ctx context.Context, upload SourceUpload) (CareerSource, error) {
	if upload.ID == "" || upload.ResourceRef == "" || upload.Digest == "" || upload.Size <= 0 || strings.TrimSpace(upload.FileName) == "" {
		return CareerSource{}, ErrInvalidRequest
	}
	s, err := getScope(ctx)
	if err != nil {
		return CareerSource{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return CareerSource{}, err
	}
	row := sourceRevision{ID: upload.ID, TenantID: s.TenantID, UserID: s.UserID, FileName: upload.FileName, MIMEType: upload.MIMEType, Size: upload.Size, Digest: upload.Digest, ResourceRef: upload.ResourceRef, Status: "processing"}
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		p := profile{TenantID: s.TenantID, UserID: s.UserID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&p).Error; err != nil {
			return err
		}
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&p).Error; err != nil {
				return err
			}
		}
		var last uint64
		if err := tx.Model(&sourceRevision{}).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Select("COALESCE(MAX(revision), 0)").Scan(&last).Error; err != nil {
			return err
		}
		row.Revision = last + 1
		return tx.Create(&row).Error
	})
	if err != nil {
		return CareerSource{}, err
	}
	return sourceView(row), nil
}

func (o *Office) FinishSource(ctx context.Context, id, text string, missing, reviewFlags []string, failure error) (CareerSource, error) {
	return o.finishSource(ctx, id, "", text, missing, reviewFlags, failure)
}

func (o *Office) FinishSourceClaim(ctx context.Context, id, token, text string, missing, reviewFlags []string, failure error) (CareerSource, error) {
	return o.finishSource(ctx, id, token, text, missing, reviewFlags, failure)
}

func (o *Office) finishSource(ctx context.Context, id, token, text string, missing, reviewFlags []string, failure error) (CareerSource, error) {
	s, err := getScope(ctx)
	if err != nil {
		return CareerSource{}, err
	}
	if strings.TrimSpace(id) == "" {
		return CareerSource{}, ErrInvalidRequest
	}
	var row sourceRevision
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, id).First(&row).Error; errors.Is(e, gorm.ErrRecordNotFound) {
			return ErrSourceNotFound
		} else if e != nil {
			return e
		}
		if token != "" && row.ClaimToken != token {
			return ErrUploadClaimLost
		}
		if row.Status != "processing" {
			return ErrInvalidRequest
		}
		if failure != nil {
			row.Status = "failed"
			row.ErrorCategory = "parse_failed"
			row.ErrorMessage = "Resume parsing failed. Retry the upload or enter profile details manually."
			if errors.Is(failure, ErrRevisionConflict) {
				row.ErrorCategory = "revision_conflict"
				row.ErrorMessage = "The profile changed during review. Upload again or enter profile details manually."
			}
			if row.ResourceRef != "" {
				row.ErrorCategory = "cleanup_pending_" + row.ErrorCategory
			}
			if row.ResourceRef == "" {
				row.ResourceRef = ""
			}
			row.ExtractedText = ""
			row.LeaseUntil = nil
		} else {
			if strings.TrimSpace(text) == "" {
				return ErrInvalidRequest
			}
			row.Status = "ready"
			row.LeaseUntil = nil
			row.ExtractedText = text
			missingJSON, _ := json.Marshal(missing)
			flagsJSON, _ := json.Marshal(reviewFlags)
			row.MissingCategories = string(missingJSON)
			row.ReviewFlags = string(flagsJSON)
		}
		updates := map[string]any{"status": row.Status, "error_category": row.ErrorCategory, "error_message": row.ErrorMessage, "resource_ref": row.ResourceRef, "extracted_text": row.ExtractedText, "missing_categories": row.MissingCategories, "review_flags": row.ReviewFlags, "lease_until": row.LeaseUntil}
		q := tx.Model(&sourceRevision{}).Where("tenant_id=? AND user_id=? AND id=? AND status='processing'", s.TenantID, s.UserID, id)
		if token != "" {
			q = q.Where("claim_token=?", token)
		}
		res := q.Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			if token != "" {
				return ErrUploadClaimLost
			}
			return ErrInvalidRequest
		}
		return nil
	})
	return sourceView(row), err
}

func (o *Office) CreateFailedSource(ctx context.Context, upload SourceUpload, failure string) (CareerSource, error) {
	if strings.TrimSpace(failure) == "" {
		return CareerSource{}, ErrInvalidRequest
	}
	if upload.Digest == "" {
		sum := sha256.Sum256([]byte(upload.FileName + failure))
		upload.Digest = hex.EncodeToString(sum[:])
	}
	return o.CreateSource(ctx, upload, failure)
}

func (o *Office) GetSource(ctx context.Context, id string) (CareerSource, error) {
	s, err := getScope(ctx)
	if err != nil {
		return CareerSource{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return CareerSource{}, err
	}
	var row sourceRevision
	err = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CareerSource{}, ErrSourceNotFound
	}
	return sourceView(row), err
}

func (o *Office) ListSources(ctx context.Context) ([]CareerSource, error) {
	s, err := getScope(ctx)
	if err != nil {
		return nil, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return nil, err
	}
	var rows []sourceRevision
	err = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Order("revision desc").Find(&rows).Error
	out := make([]CareerSource, 0, len(rows))
	for _, row := range rows {
		out = append(out, sourceView(row))
	}
	return out, err
}

func validFieldKey(key string) bool {
	parts := strings.Split(key, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}
	allowed := map[string]bool{"education": true, "experience": true, "project": true, "skill": true, "achievement": true, "certificate": true, "preference": true}
	if !allowed[parts[0]] {
		return false
	}
	for _, part := range parts {
		if part == "" || len(part) > 64 {
			return false
		}
		for _, r := range part {
			if !(r == '_' || r == '-' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
				return false
			}
		}
	}
	return true
}

func (o *Office) CompleteIntake(ctx context.Context, sourceID, text string, fields []ExtractedField, missing, reviewFlags []string, requestID string, expectedRevision uint64) (Receipt, error) {
	return o.completeIntake(ctx, sourceID, "", text, fields, missing, reviewFlags, requestID, expectedRevision)
}

func (o *Office) CompleteIntakeClaim(ctx context.Context, sourceID, token, text string, fields []ExtractedField, missing, reviewFlags []string, requestID string, expectedRevision uint64) (Receipt, error) {
	return o.completeIntake(ctx, sourceID, token, text, fields, missing, reviewFlags, requestID, expectedRevision)
}

func (o *Office) completeIntake(ctx context.Context, sourceID, token, text string, fields []ExtractedField, missing, reviewFlags []string, requestID string, expectedRevision uint64) (Receipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return Receipt{}, err
	}
	if strings.TrimSpace(sourceID) == "" || strings.TrimSpace(requestID) == "" || strings.TrimSpace(text) == "" || len(text) > 4*1024*1024 || len(fields) == 0 || len(fields) > 500 {
		return Receipt{}, ErrInvalidRequest
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return Receipt{}, err
	}
	fields = append([]ExtractedField(nil), fields...)
	sort.Slice(fields, func(i, j int) bool {
		if fields[i].Key == fields[j].Key {
			if fields[i].Value == fields[j].Value {
				return fields[i].Evidence < fields[j].Evidence
			}
			return fields[i].Value < fields[j].Value
		}
		return fields[i].Key < fields[j].Key
	})
	seen := map[string]bool{}
	for _, field := range fields {
		if !validFieldKey(field.Key) || strings.TrimSpace(field.Value) == "" || len(field.Value) > 20000 || len(field.Evidence) > 20000 {
			return Receipt{}, ErrInvalidRequest
		}
		canonical := field.Key + "\x00" + field.Value
		if seen[canonical] {
			return Receipt{}, ErrInvalidRequest
		}
		seen[canonical] = true
	}
	proposals := make([]Proposal, 0, len(fields))
	for _, field := range fields {
		proposals = append(proposals, Proposal{ID: uuid.NewString(), Key: field.Key, Value: field.Value, Evidence: field.Evidence, Source: Source{Kind: "resume_extraction", ReferenceID: sourceID}, Status: "pending"})
	}
	return o.mutate(ctx, s, "intake_completed", requestID, expectedRevision, []any{"complete_intake", sourceID, text, fields, missing, reviewFlags, expectedRevision}, func(tx *gorm.DB, next uint64) (Receipt, error) {
		var source sourceRevision
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, sourceID).First(&source).Error; errors.Is(e, gorm.ErrRecordNotFound) {
			return Receipt{}, ErrSourceNotFound
		} else if e != nil {
			return Receipt{}, e
		}
		if token != "" && source.ClaimToken != token {
			return Receipt{}, ErrUploadClaimLost
		}
		if source.Status != "processing" || source.CompletedAt != nil {
			return Receipt{}, ErrInvalidRequest
		}
		now := time.Now().UTC()
		source.Status = "ready"
		source.LeaseUntil = nil
		source.ExtractedText = text
		missingJSON, _ := json.Marshal(missing)
		flagsJSON, _ := json.Marshal(reviewFlags)
		source.MissingCategories = string(missingJSON)
		source.ReviewFlags = string(flagsJSON)
		source.CompletedAt = &now
		res := tx.Model(&sourceRevision{}).Where("tenant_id=? AND user_id=? AND id=? AND status='processing' AND claim_token=?", s.TenantID, s.UserID, sourceID, token).Updates(map[string]any{"status": source.Status, "lease_until": source.LeaseUntil, "extracted_text": source.ExtractedText, "missing_categories": source.MissingCategories, "review_flags": source.ReviewFlags, "completed_at": source.CompletedAt})
		if res.Error != nil {
			return Receipt{}, res.Error
		}
		if res.RowsAffected != 1 {
			return Receipt{}, ErrUploadClaimLost
		}
		for i := range proposals {
			pview := &proposals[i]
			sb, _ := json.Marshal(pview.Source)
			row := proposal{PublicID: pview.ID, TenantID: s.TenantID, UserID: s.UserID, Key: pview.Key, Value: pview.Value, Evidence: pview.Evidence, Source: string(sb), Status: "pending"}
			if e := tx.Create(&row).Error; e != nil {
				return Receipt{}, e
			}
			pview.CreatedAt = row.CreatedAt
		}
		return Receipt{Kind: "intake_completed", RequestID: requestID, Revision: next, Proposals: proposals}, nil
	}, &Change{Kind: "intake_completed"})
}

func (o *Office) sourceText(ctx context.Context, s Scope, id string) (string, error) {
	var row sourceRevision
	err := o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND id=? AND status='ready'", s.TenantID, s.UserID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrSourceNotFound
	}
	return row.ExtractedText, err
}

func (s CareerSource) String() string { encoded, _ := json.Marshal(s); return string(encoded) }
