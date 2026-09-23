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
	ID          string
	FileName    string
	MIMEType    string
	Size        int64
	Digest      string
	ResourceRef string
	Text        string
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
	return CareerSource{ID: row.ID, Revision: row.Revision, FileName: row.FileName, MIMEType: row.MIMEType, Size: row.Size, Digest: row.Digest, Status: row.Status, ErrorCategory: row.ErrorCategory, ErrorMessage: row.ErrorMessage, MissingCategories: missing, ReviewFlags: flags, CreatedAt: row.CreatedAt, CompletedAt: row.CompletedAt}
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
			row.ResourceRef = ""
			row.ExtractedText = ""
		} else {
			if strings.TrimSpace(text) == "" {
				return ErrInvalidRequest
			}
			row.Status = "ready"
			row.ExtractedText = text
			missingJSON, _ := json.Marshal(missing)
			flagsJSON, _ := json.Marshal(reviewFlags)
			row.MissingCategories = string(missingJSON)
			row.ReviewFlags = string(flagsJSON)
		}
		return tx.Save(&row).Error
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
		if source.Status != "processing" || source.CompletedAt != nil {
			return Receipt{}, ErrInvalidRequest
		}
		now := time.Now().UTC()
		source.Status = "ready"
		source.ExtractedText = text
		missingJSON, _ := json.Marshal(missing)
		flagsJSON, _ := json.Marshal(reviewFlags)
		source.MissingCategories = string(missingJSON)
		source.ReviewFlags = string(flagsJSON)
		source.CompletedAt = &now
		if e := tx.Save(&source).Error; e != nil {
			return Receipt{}, e
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
