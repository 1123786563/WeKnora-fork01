package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrSemanticScopeInvalid     = errors.New("semantic_scope_invalid")
	ErrSemanticScopeDenied      = errors.New("semantic_scope_denied")
	ErrSemanticScopeExpired     = errors.New("semantic_scope_expired")
	ErrSemanticScopeChanged     = errors.New("semantic_scope_changed")
	ErrSemanticScopeUnavailable = errors.New("semantic_scope_unavailable")
)

// semanticScopeGuard preserves lightweight service construction in tools/tests.
// Production container decorators install the mandatory durable invalidator
// before exposing any ACL writer, independently of semantic.enabled.
type semanticScopeGuard struct {
	semanticInvalidator interfaces.SemanticScopeInvalidator
}

func (s *semanticScopeGuard) SetSemanticScopeInvalidator(i interfaces.SemanticScopeInvalidator) {
	s.semanticInvalidator = i
}
func (s *semanticScopeGuard) invalidateSemanticKB(ctx context.Context, tenant uint64, kb string) error {
	if s.semanticInvalidator == nil {
		return nil
	}
	return s.semanticInvalidator.InvalidateKB(ctx, tenant, kb)
}

func (s *semanticScopeGuard) invalidateSemanticTenant(ctx context.Context, tenant uint64) error {
	if s.semanticInvalidator == nil {
		return nil
	}
	return s.semanticInvalidator.InvalidateTenant(ctx, tenant)
}

func (s *semanticScopeGuard) invalidateSemanticUser(ctx context.Context, user string) error {
	if s.semanticInvalidator == nil {
		return nil
	}
	return s.semanticInvalidator.InvalidateUser(ctx, user)
}
func (s *semanticScopeGuard) invalidateSemanticOrganization(ctx context.Context, org string) error {
	if s.semanticInvalidator == nil {
		return nil
	}
	return s.semanticInvalidator.InvalidateOrganization(ctx, org)
}
func (s *semanticScopeGuard) invalidateSemanticTransfer(ctx context.Context, source, target *types.KnowledgeBase) error {
	if s.semanticInvalidator == nil || source.ID == target.ID && source.TenantID == target.TenantID {
		return nil
	}
	return s.semanticInvalidator.InvalidateTransfer(ctx, types.SemanticScopeKey{TenantID: source.TenantID, KBID: source.ID}, types.SemanticScopeKey{TenantID: target.TenantID, KBID: target.ID})
}

// SemanticScopeSnapshot is the Go-authoritative allowlist consumed by A02.
// Revisions are maxima, not an instruction to expose an unpublished generation.
type SemanticScopeSnapshot struct {
	Scope                   types.SemanticScopeKey      `json:"scope"`
	SubjectID               string                      `json:"subject_id"`
	RequesterTenantID       uint64                      `json:"requester_tenant_id"`
	AllowedDocumentIDs      []string                    `json:"allowed_document_ids"`
	MaxSourceRevisions      map[string]uint64           `json:"max_source_revisions"`
	AllowRetainedPrevious   map[string]bool             `json:"allow_retained_previous"`
	DeniedDocumentRevisions map[string]uint64           `json:"denied_document_revisions"`
	PermissionEpoch         uint64                      `json:"permission_epoch"`
	ScopeHash               string                      `json:"scope_hash"`
	ExpiresAt               string                      `json:"expires_at"`
	Purpose                 types.SemanticAccessPurpose `json:"purpose"`
	Audience                string                      `json:"audience"`
	BudgetRef               string                      `json:"budget_ref"`
	// Bind live role changes even when the resulting readable IDs are identical.
	TenantRole types.TenantRole    `json:"tenant_role"`
	ShareRole  types.OrgMemberRole `json:"share_role,omitempty"`
}

type semanticScopeClaims struct {
	jwt.RegisteredClaims
	Version           int                         `json:"version"`
	RequesterTenantID uint64                      `json:"requester_tenant_id"`
	Scope             types.SemanticScopeKey      `json:"scope"`
	Purpose           types.SemanticAccessPurpose `json:"purpose"`
	BudgetRef         string                      `json:"budget_ref"`
	Epoch             uint64                      `json:"permission_epoch"`
	Hash              string                      `json:"scope_hash"`
}

type SemanticScopeService struct {
	cfg       *config.SemanticServiceConfig
	control   *repository.SemanticControlRepository
	members   interfaces.TenantMemberRepository
	kbs       interfaces.KnowledgeBaseRepository
	knowledge interfaces.KnowledgeRepository
	shares    interfaces.KBShareService
	users     interfaces.UserRepository
	tenants   interfaces.TenantRepository
}

func NewSemanticScopeService(cfg *config.Config, control *repository.SemanticControlRepository, members interfaces.TenantMemberRepository, kbs interfaces.KnowledgeBaseRepository, knowledge interfaces.KnowledgeRepository, shares interfaces.KBShareService, users interfaces.UserRepository, tenants interfaces.TenantRepository) *SemanticScopeService {
	return &SemanticScopeService{cfg: cfg.Semantic, control: control, members: members, kbs: kbs, knowledge: knowledge, shares: shares, users: users, tenants: tenants}
}
func (s *SemanticScopeService) ready() bool {
	return s.cfg != nil && s.cfg.Enabled && s.cfg.HasValidScopeSigningKey() && strings.TrimSpace(s.cfg.Audience) != ""
}
func validSemanticPurpose(p types.SemanticAccessPurpose) bool {
	return p == types.SemanticAccessPurposeSearch || p == types.SemanticAccessPurposeReason || p == types.SemanticAccessPurposeIndex
}

func (s *SemanticScopeService) Issue(ctx context.Context, subject string, owner types.SemanticScopeKey, purpose types.SemanticAccessPurpose, budget string) (types.SemanticAccessScope, error) {
	if !s.ready() {
		return types.SemanticAccessScope{}, ErrSemanticScopeUnavailable
	}
	caller := types.CallerFromContext(ctx)
	if subject == "" || caller.UserID != subject || caller.TenantID == 0 || !validSemanticPurpose(purpose) || strings.TrimSpace(budget) == "" {
		return types.SemanticAccessScope{}, ErrSemanticScopeInvalid
	}
	now := time.Now().UTC()
	c := semanticScopeClaims{Version: 1, RequesterTenantID: caller.TenantID, Scope: owner, Purpose: purpose, BudgetRef: budget, RegisteredClaims: jwt.RegisteredClaims{Subject: subject, Audience: jwt.ClaimStrings{s.cfg.Audience}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute))}}
	snap, err := s.snapshot(ctx, c)
	if err != nil {
		if errors.Is(err, ErrSemanticScopeChanged) {
			return types.SemanticAccessScope{}, ErrSemanticScopeDenied
		}
		return types.SemanticAccessScope{}, err
	}
	c.Epoch, c.Hash = snap.PermissionEpoch, snap.ScopeHash
	ref, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(s.cfg.ScopeSigningKey))
	if err != nil {
		return types.SemanticAccessScope{}, ErrSemanticScopeUnavailable
	}
	return c.metadata(ref), nil
}
func (c semanticScopeClaims) metadata(ref string) types.SemanticAccessScope {
	return types.SemanticAccessScope{Scope: c.Scope, SubjectID: c.Subject, ScopeRef: ref, ScopeHash: c.Hash, ExpiresAt: c.ExpiresAt.Time.UTC().Format(time.RFC3339), Audience: c.Audience[0], BudgetRef: c.BudgetRef, PermissionEpoch: c.Epoch, Purpose: c.Purpose}
}

func (s *SemanticScopeService) parse(ref string) (semanticScopeClaims, error) {
	var c semanticScopeClaims
	if !s.ready() {
		return c, ErrSemanticScopeUnavailable
	}
	token, err := jwt.ParseWithClaims(ref, &c, func(*jwt.Token) (any, error) { return []byte(s.cfg.ScopeSigningKey), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithAudience(s.cfg.Audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) && !errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return c, ErrSemanticScopeExpired
		}
		return c, ErrSemanticScopeInvalid
	}
	if !token.Valid || c.Version != 1 || c.Subject == "" || c.RequesterTenantID == 0 || c.Scope.TenantID == 0 || strings.TrimSpace(c.Scope.KBID) == "" || c.Hash == "" || c.BudgetRef == "" || !validSemanticPurpose(c.Purpose) || len(c.Audience) != 1 || c.IssuedAt == nil || c.ExpiresAt == nil || c.ExpiresAt.Time.Sub(c.IssuedAt.Time) > 5*time.Minute || !c.ExpiresAt.After(c.IssuedAt.Time) {
		return c, ErrSemanticScopeInvalid
	}
	return c, nil
}

func (s *SemanticScopeService) Resolve(ctx context.Context, ref string) (SemanticScopeSnapshot, error) {
	c, err := s.parse(ref)
	if err != nil {
		return SemanticScopeSnapshot{}, err
	}
	return s.resolveClaims(ctx, c)
}
func (s *SemanticScopeService) resolveClaims(ctx context.Context, c semanticScopeClaims) (SemanticScopeSnapshot, error) {
	snap, err := s.snapshot(ctx, c)
	if err != nil {
		return SemanticScopeSnapshot{}, err
	}
	if snap.PermissionEpoch != c.Epoch || snap.ScopeHash != c.Hash {
		return SemanticScopeSnapshot{}, ErrSemanticScopeChanged
	}
	if !time.Now().Before(c.ExpiresAt.Time) {
		return SemanticScopeSnapshot{}, ErrSemanticScopeExpired
	}
	return snap, nil
}
func (s *SemanticScopeService) ValidateDelivery(ctx context.Context, scope types.SemanticAccessScope) error {
	c, err := s.parse(scope.ScopeRef)
	if err != nil {
		return err
	}
	if c.metadata(scope.ScopeRef) != scope {
		return ErrSemanticScopeInvalid
	}
	_, err = s.resolveClaims(ctx, c)
	return err
}

func (s *SemanticScopeService) snapshot(ctx context.Context, c semanticScopeClaims) (SemanticScopeSnapshot, error) {
	result := SemanticScopeSnapshot{}
	user, err := s.users.GetUserByID(ctx, c.Subject)
	if errors.Is(err, repository.ErrUserNotFound) {
		return result, ErrSemanticScopeChanged
	}
	if err != nil {
		return result, ErrSemanticScopeUnavailable
	}
	if user == nil || !user.IsActive || user.DeletedAt.Valid {
		return result, ErrSemanticScopeChanged
	}
	for _, id := range []uint64{c.RequesterTenantID, c.Scope.TenantID} {
		tenant, err := s.tenants.GetTenantByID(ctx, id)
		if errors.Is(err, repository.ErrTenantNotFound) {
			return result, ErrSemanticScopeChanged
		}
		if err != nil {
			return result, ErrSemanticScopeUnavailable
		}
		if tenant == nil || tenant.DeletedAt.Valid || tenant.Status != "active" {
			return result, ErrSemanticScopeChanged
		}
	}
	member, err := s.members.Get(ctx, c.Subject, c.RequesterTenantID)
	if err != nil {
		return result, ErrSemanticScopeUnavailable
	}
	if member == nil || member.Status != types.TenantMemberStatusActive || !member.Role.IsValid() {
		return result, ErrSemanticScopeChanged
	}
	kb, err := s.kbs.GetKnowledgeBaseByID(ctx, c.Scope.KBID)
	if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
		return result, ErrSemanticScopeChanged
	}
	if err != nil {
		return result, ErrSemanticScopeUnavailable
	}
	if kb == nil || kb.TenantID != c.Scope.TenantID || kb.IsTemporary || kb.DeletedAt.Valid {
		return result, ErrSemanticScopeChanged
	}
	var shareRole types.OrgMemberRole
	if kb.TenantID != c.RequesterTenantID {
		role, shared, err := s.shares.CheckTenantKBPermission(ctx, kb.ID, c.RequesterTenantID, member.Role)
		if err != nil {
			return result, ErrSemanticScopeUnavailable
		}
		if !shared || !role.HasPermission(types.OrgRoleViewer) {
			return result, ErrSemanticScopeChanged
		}
		shareRole = role
	}
	epoch, active, denied, err := s.control.ReadSemanticScopeState(ctx, c.Scope)
	if err != nil {
		return result, ErrSemanticScopeUnavailable
	}
	docs, err := s.knowledge.ListKnowledgeByKnowledgeBaseID(ctx, kb.TenantID, kb.ID)
	if err != nil {
		return result, ErrSemanticScopeUnavailable
	}
	result = SemanticScopeSnapshot{Scope: c.Scope, SubjectID: c.Subject, RequesterTenantID: c.RequesterTenantID, AllowedDocumentIDs: []string{}, MaxSourceRevisions: map[string]uint64{}, AllowRetainedPrevious: map[string]bool{}, DeniedDocumentRevisions: denied, PermissionEpoch: epoch, Purpose: c.Purpose, Audience: s.cfg.Audience, BudgetRef: c.BudgetRef, TenantRole: member.Role, ShareRole: shareRole}
	for _, doc := range docs {
		rev, ok := active[doc.ID]
		if !ok || rev == 0 {
			continue
		}
		if _, blocked := denied[doc.ID]; blocked {
			continue
		}
		result.AllowedDocumentIDs = append(result.AllowedDocumentIDs, doc.ID)
		result.MaxSourceRevisions[doc.ID] = rev
		result.AllowRetainedPrevious[doc.ID] = true
	}
	sort.Strings(result.AllowedDocumentIDs)
	// encoding/json sorts map keys; all remaining fields have fixed struct order.
	data, err := json.Marshal(result)
	if err != nil {
		return SemanticScopeSnapshot{}, ErrSemanticScopeUnavailable
	}
	hash := sha256.Sum256(data)
	result.ScopeHash = "sha256:" + hex.EncodeToString(hash[:])
	result.ExpiresAt = c.ExpiresAt.Time.UTC().Format(time.RFC3339)
	return result, nil
}
