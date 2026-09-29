package career

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrCareerDeleting = errors.New("career space is deleting")
var ErrCareerOperationsBusy = errors.New("career operations are still unresolved")

// admitLifecycleClaim durably admits an external effect before it can start.
// Re-admission of the same request is the recovery path after process restart.
func (o *Office) admitLifecycleClaim(ctx context.Context, s Scope, operation, requestID string, intentFingerprint ...string) error {
	return o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return admitLifecycleClaimTx(tx, s, operation, requestID, intentFingerprint...)
	})
}

// admitLifecycleClaimTx lets other durable linearization transactions (such
// as a scheduled rule-period claim) atomically couple their operation row to
// the scope deletion gate.
func admitLifecycleClaimTx(tx *gorm.DB, s Scope, operation, requestID string, intentFingerprint ...string) error {
	fingerprint := ""
	if len(intentFingerprint) > 0 {
		fingerprint = intentFingerprint[0]
	}
	gate := lifecycleGate{TenantID: s.TenantID, UserID: s.UserID, Phase: "active"}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&gate).Error; err != nil {
		return err
	}
	var current lifecycleGate
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&current).Error; err != nil {
		return err
	}
	if current.Phase != "active" {
		return ErrCareerDeleting
	}
	if current.DeletionRequestID != "" {
		var existing lifecycleClaim
		err := tx.Where("tenant_id=? AND user_id=? AND operation=? AND request_id=?", s.TenantID, s.UserID, operation, requestID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCareerDeleting
		}
		if err != nil {
			return err
		}
		if fingerprint != "" && existing.Fingerprint != fingerprint {
			return ErrIdempotencyConflict
		}
		return nil // same-request recovery of an effect admitted before deletion
	}
	claim := lifecycleClaim{TenantID: s.TenantID, UserID: s.UserID, Operation: operation, RequestID: requestID, Fingerprint: fingerprint, CreatedAt: time.Now().UTC()}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim).Error; err != nil {
		return err
	}
	if fingerprint == "" {
		return nil
	}
	var stored lifecycleClaim
	if err := tx.Where("tenant_id=? AND user_id=? AND operation=? AND request_id=?", s.TenantID, s.UserID, operation, requestID).First(&stored).Error; err != nil {
		return err
	}
	if stored.Fingerprint != fingerprint {
		return ErrIdempotencyConflict
	}
	return nil
}

func (o *Office) resolveLifecycleClaim(ctx context.Context, s Scope, operation, requestID string) error {
	return o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND operation=? AND request_id=?", s.TenantID, s.UserID, operation, requestID).Delete(&lifecycleClaim{}).Error
}

// beginLifecycleDeletion changes the durable phase only after all previously
// admitted effects have reached a known outcome. Claims are never age-expired.
func (o *Office) beginLifecycleDeletion(ctx context.Context, s Scope, requestID, fingerprint string) error {
	busy := false
	err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		gate := lifecycleGate{TenantID: s.TenantID, UserID: s.UserID, Phase: "active"}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&gate).Error; err != nil {
			return err
		}
		var current lifecycleGate
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&current).Error; err != nil {
			return err
		}
		if current.Phase == "deleting" {
			if current.DeletionRequestID != requestID {
				return ErrCareerDeleting
			}
			if current.DeletionFingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			return nil
		}
		if current.DeletionRequestID != "" {
			if current.DeletionRequestID != requestID {
				return ErrCareerDeleting
			}
			if current.DeletionFingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
		}
		result := tx.Model(&lifecycleGate{}).Where("tenant_id=? AND user_id=? AND phase=?", s.TenantID, s.UserID, "active").Updates(map[string]any{"deletion_request_id": requestID, "deletion_fingerprint": fingerprint})
		if result.Error != nil {
			return result.Error
		}
		var count int64
		if err := tx.Model(&lifecycleClaim{}).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			busy = true
			return nil
		}
		result = tx.Model(&lifecycleGate{}).Where("tenant_id=? AND user_id=? AND phase=?", s.TenantID, s.UserID, "active").Updates(map[string]any{"phase": "deleting", "deletion_request_id": requestID, "deletion_fingerprint": fingerprint})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCareerDeleting
		}
		return nil
	})
	if err != nil {
		return err
	}
	if busy {
		return ErrCareerOperationsBusy
	}
	return nil
}
