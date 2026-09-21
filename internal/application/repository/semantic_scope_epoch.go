package repository

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

// ReadSemanticScopeState uses one database snapshot for epochs, revisions and
// deletion denials. Missing revision rows never imply permission to read.
func (r *SemanticControlRepository) ReadSemanticScopeState(ctx context.Context, scope types.SemanticScopeKey) (epoch uint64, active map[string]uint64, denied map[string]uint64, err error) {
	if scope.TenantID == 0 || strings.TrimSpace(scope.KBID) == "" {
		return 0, nil, nil, ErrSemanticMutationInvalid
	}
	active, denied = map[string]uint64{}, map[string]uint64{}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var e string
		err := tx.Raw("SELECT epoch FROM semantic_access_epochs WHERE tenant_id=? AND kb_id=?", semanticUint(scope.TenantID), scope.KBID).Row().Scan(&e)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil {
			epoch, err = strconv.ParseUint(e, 10, 64)
			if err != nil {
				return err
			}
		}
		for _, query := range []struct {
			sql    string
			target map[string]uint64
		}{
			{"SELECT document_id,revision FROM semantic_document_revisions WHERE tenant_id=? AND kb_id=? AND deleted=false", active},
			{"SELECT document_id,revision FROM semantic_denials WHERE tenant_id=? AND kb_id=?", denied},
		} {
			rows, err := tx.Raw(query.sql, semanticUint(scope.TenantID), scope.KBID).Rows()
			if err != nil {
				return err
			}
			for rows.Next() {
				var id, rev string
				if err = rows.Scan(&id, &rev); err != nil {
					rows.Close()
					return err
				}
				v, e := strconv.ParseUint(rev, 10, 64)
				if e != nil {
					rows.Close()
					return e
				}
				query.target[id] = v
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
		}
		return nil
	})
	return
}

func (r *SemanticControlRepository) BumpSemanticEpochs(ctx context.Context, scopes []types.SemanticScopeKey) error {
	unique := make(map[types.SemanticScopeKey]struct{}, len(scopes))
	for _, s := range scopes {
		if s.TenantID == 0 || strings.TrimSpace(s.KBID) == "" {
			return ErrSemanticMutationInvalid
		}
		unique[s] = struct{}{}
	}
	ordered := make([]types.SemanticScopeKey, 0, len(unique))
	for s := range unique {
		ordered = append(ordered, s)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].TenantID != ordered[j].TenantID {
			return ordered[i].TenantID < ordered[j].TenantID
		}
		return ordered[i].KBID < ordered[j].KBID
	})
	if len(ordered) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, s := range ordered {
			if _, err := r.BumpSemanticEpoch(tx, s); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *SemanticControlRepository) InvalidateKB(ctx context.Context, tenant uint64, kb string) error {
	return r.BumpSemanticEpochs(ctx, []types.SemanticScopeKey{{TenantID: tenant, KBID: kb}})
}
func (r *SemanticControlRepository) InvalidateTransfer(ctx context.Context, source, destination types.SemanticScopeKey) error {
	return r.BumpSemanticEpochs(ctx, []types.SemanticScopeKey{source, destination})
}

func (r *SemanticControlRepository) InvalidateTenant(ctx context.Context, tenant uint64) error {
	scopes, err := r.semanticTenantScopes(ctx, tenant)
	if err != nil {
		return err
	}
	return r.BumpSemanticEpochs(ctx, scopes)
}
func (r *SemanticControlRepository) semanticTenantScopes(ctx context.Context, tenant uint64) ([]types.SemanticScopeKey, error) {
	if tenant == 0 {
		return nil, ErrSemanticMutationInvalid
	}
	var scopes []types.SemanticScopeKey
	err := r.db.WithContext(ctx).Raw(`SELECT DISTINCT k.tenant_id,k.id AS kb_id FROM knowledge_bases k
 WHERE k.deleted_at IS NULL AND (k.tenant_id=? OR EXISTS (
 SELECT 1 FROM kb_shares s JOIN organization_tenant_members m ON m.organization_id=s.organization_id
 JOIN organizations o ON o.id=s.organization_id AND o.deleted_at IS NULL
 WHERE s.knowledge_base_id=k.id AND s.deleted_at IS NULL AND m.tenant_id=?))`, tenant, tenant).Scan(&scopes).Error
	if err != nil {
		return nil, err
	}
	return scopes, nil
}
func (r *SemanticControlRepository) InvalidateUser(ctx context.Context, user string) error {
	if strings.TrimSpace(user) == "" {
		return ErrSemanticMutationInvalid
	}
	var tenants []uint64
	if err := r.db.WithContext(ctx).Table("tenant_members").Where("user_id=? AND status=? AND deleted_at IS NULL", user, types.TenantMemberStatusActive).Distinct("tenant_id").Pluck("tenant_id", &tenants).Error; err != nil {
		return err
	}
	var scopes []types.SemanticScopeKey
	for _, tenant := range tenants {
		affected, err := r.semanticTenantScopes(ctx, tenant)
		if err != nil {
			return err
		}
		scopes = append(scopes, affected...)
	}
	return r.BumpSemanticEpochs(ctx, scopes)
}
func (r *SemanticControlRepository) InvalidateOrganization(ctx context.Context, organization string) error {
	if strings.TrimSpace(organization) == "" {
		return ErrSemanticMutationInvalid
	}
	var scopes []types.SemanticScopeKey
	err := r.db.WithContext(ctx).Raw(`SELECT DISTINCT k.tenant_id,k.id AS kb_id FROM knowledge_bases k
 JOIN kb_shares s ON s.knowledge_base_id=k.id WHERE k.deleted_at IS NULL AND s.deleted_at IS NULL AND s.organization_id=?`, organization).Scan(&scopes).Error
	if err != nil {
		return err
	}
	return r.BumpSemanticEpochs(ctx, scopes)
}
