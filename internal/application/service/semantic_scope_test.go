package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const semanticTestKey = "test-only-scope-key-32-bytes-long!!"

type semanticScopeFixture struct {
	t       *testing.T
	DB      *gorm.DB
	Service *SemanticScopeService
	Control *repository.SemanticControlRepository
	Context context.Context
}

func (f *semanticScopeFixture) ContextFor(user string, tenant uint64) context.Context {
	return types.WithCaller(context.Background(), types.Caller{UserID: user, TenantID: tenant})
}

type semanticOrganizationReadFailure struct {
	interfaces.OrganizationRepository
	err error
}

type semanticSlowKnowledgeRead struct {
	interfaces.KnowledgeRepository
	until time.Time
}

func (r semanticSlowKnowledgeRead) ListKnowledgeByKnowledgeBaseID(ctx context.Context, tenant uint64, kb string) ([]*types.Knowledge, error) {
	timer := time.NewTimer(time.Until(r.until))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}
	return r.KnowledgeRepository.ListKnowledgeByKnowledgeBaseID(ctx, tenant, kb)
}
func TestSemanticScopeRejectsExpiryDuringDeliveryRead(t *testing.T) {
	f := newSemanticScopeFixture(t)
	s := f.Issue("member-a", 10, "kb-a", types.SemanticAccessPurposeSearch, "budget")
	expiry := time.Now().Add(2 * time.Second).Truncate(time.Second)
	s.ScopeRef = mutateSemanticClaims(t, s.ScopeRef, func(c jwt.MapClaims) { c["exp"] = expiry.Unix() })
	s.ExpiresAt = expiry.UTC().Format(time.RFC3339)
	f.Service.knowledge = semanticSlowKnowledgeRead{KnowledgeRepository: f.Service.knowledge, until: expiry.Add(10 * time.Millisecond)}
	require.ErrorIs(t, f.Service.ValidateDelivery(f.Context, s), ErrSemanticScopeExpired)
}

func (r semanticOrganizationReadFailure) GetTenantMember(context.Context, string, uint64) (*types.OrganizationTenantMember, error) {
	return nil, r.err
}
func (f *semanticScopeFixture) FailOrganizationRead(err error) {
	s := f.Service.shares.(*kbShareService)
	s.orgRepo = semanticOrganizationReadFailure{OrganizationRepository: s.orgRepo, err: err}
}

func TestSemanticScopeFailsClosedWhenOrganizationLookupFails(t *testing.T) {
	f := newSemanticScopeFixture(t)
	f.FailOrganizationRead(errors.New("organization store unavailable"))
	_, err := f.Service.Issue(f.ContextFor("member-a", 20), "member-a", types.SemanticScopeKey{TenantID: 10, KBID: "shared-kb"}, types.SemanticAccessPurposeSearch, "budget")
	require.ErrorIs(t, err, ErrSemanticScopeUnavailable)
}
func TestSemanticScopeRejectsDeletedOrganizationWithSurvivingShare(t *testing.T) {
	f := newSemanticScopeFixture(t)
	s := f.Issue("member-a", 20, "shared-kb", types.SemanticAccessPurposeSearch, "budget")
	require.NoError(t, f.DB.Exec("UPDATE organizations SET deleted_at=CURRENT_TIMESTAMP WHERE id='scope-org'").Error)
	require.Error(t, f.Service.ValidateDelivery(f.Context, s))
	_, err := f.Service.Issue(f.ContextFor("member-a", 20), "member-a", s.Scope, s.Purpose, s.BudgetRef)
	require.Error(t, err)
}
func TestSemanticScopeCanonicalLargeSet(t *testing.T) {
	f := newSemanticScopeFixture(t)
	for i := 49; i >= 0; i-- {
		f.AddActiveSemanticDocument(10, "kb-a", fmt.Sprintf("doc-%03d", i), uint64(i+1))
	}
	a := f.Issue("member-a", 10, "kb-a", types.SemanticAccessPurposeSearch, "budget")
	b := f.Issue("member-a", 10, "kb-a", types.SemanticAccessPurposeSearch, "budget")
	require.Equal(t, a.ScopeHash, b.ScopeHash)
	snap, err := f.ResolveScope(a.ScopeRef)
	require.NoError(t, err)
	require.Len(t, snap.AllowedDocumentIDs, 50)
	require.Equal(t, "doc-000", snap.AllowedDocumentIDs[0])
	require.Equal(t, "doc-049", snap.AllowedDocumentIDs[49])
	require.Equal(t, uint64(50), snap.MaxSourceRevisions["doc-049"])
}

func newSemanticScopeFixture(t *testing.T) *semanticScopeFixture {
	db := openDurableRunTestDB(t)
	// Existing organization status writes use PostgreSQL NOW(). Register only
	// this clock function in the SQLite fixture; the production SQL still runs.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	conn, err := sqlDB.Conn(context.Background())
	require.NoError(t, err)
	require.NoError(t, conn.Raw(func(driverConn any) error {
		return driverConn.(*sqlite3.SQLiteConn).RegisterFunc("NOW", func() string { return time.Now().UTC().Format("2006-01-02 15:04:05") }, false)
	}))
	require.NoError(t, conn.Close())
	f := &semanticScopeFixture{t: t, DB: db, Control: repository.NewSemanticControlRepository(db), Context: context.Background()}
	shares := NewKBShareService(repository.NewKBShareRepository(db), repository.NewOrganizationRepository(db), repository.NewKnowledgeBaseRepository(db), repository.NewKnowledgeRepository(db), nil, nil)
	f.Service = NewSemanticScopeService(&config.Config{Semantic: &config.SemanticServiceConfig{Enabled: true, Audience: "semantic-test", ScopeSigningKey: semanticTestKey}}, f.Control, repository.NewTenantMemberRepository(db), repository.NewKnowledgeBaseRepository(db), repository.NewKnowledgeRepository(db), shares, repository.NewUserRepository(db), repository.NewTenantRepository(db))
	f.AddOwnedKB(10, "kb-a")
	f.AddSharedKB(10, 20, "shared-kb")
	f.AddMember("member-a", 10, types.TenantRoleContributor)
	f.AddMember("member-a", 20, types.TenantRoleContributor)
	return f
}
func (f *semanticScopeFixture) AddOwnedKB(tenant uint64, kb string) {
	f.t.Helper()
	require.NoError(f.t, f.DB.Exec("INSERT INTO tenants(id,name,business,status) VALUES(?,'scope tenant','test','active') ON CONFLICT(id) DO NOTHING", tenant).Error)
	require.NoError(f.t, f.DB.Exec("INSERT INTO knowledge_bases(id,name,tenant_id,embedding_model_id,summary_model_id) VALUES(?,?,?,'','') ON CONFLICT(id) DO NOTHING", kb, kb, tenant).Error)
}
func (f *semanticScopeFixture) AddMember(user string, tenant uint64, role types.TenantRole) {
	f.t.Helper()
	require.NoError(f.t, f.DB.Exec("INSERT INTO tenants(id,name,business,status) VALUES(?,'scope tenant','test','active') ON CONFLICT(id) DO NOTHING", tenant).Error)
	require.NoError(f.t, f.DB.Exec("INSERT INTO users(id,username,email,password_hash,tenant_id,is_active) VALUES(?,?,?,'test-only',?,true) ON CONFLICT(id) DO NOTHING", user, user, user+"@scope.test", tenant).Error)
	require.NoError(f.t, f.DB.Exec("INSERT INTO tenant_members(user_id,tenant_id,role,status) VALUES(?,?,?,'active')", user, tenant, role).Error)
}

func (f *semanticScopeFixture) DeleteUser(user string) error {
	s := &userService{userRepo: repository.NewUserRepository(f.DB)}
	s.SetSemanticScopeInvalidator(f.Control)
	return s.DeleteUser(f.Context, user)
}
func (f *semanticScopeFixture) DeleteTenant(tenant uint64) error {
	s := &tenantService{repo: repository.NewTenantRepository(f.DB)}
	s.SetSemanticScopeInvalidator(f.Control)
	return s.DeleteTenant(f.Context, tenant)
}
func (f *semanticScopeFixture) UpdateTenantRole(user string, tenant uint64, role types.TenantRole) {
	f.t.Helper()
	require.NoError(f.t, f.DB.Exec("UPDATE tenant_members SET role=? WHERE user_id=? AND tenant_id=?", role, user, tenant).Error)
}
func (f *semanticScopeFixture) SoftDeleteOrganization(org string) {
	f.t.Helper()
	require.NoError(f.t, f.DB.Exec("UPDATE organizations SET deleted_at=CURRENT_TIMESTAMP WHERE id=?", org).Error)
}
func TestSemanticScopeRejectsDeletedUserAndTenant(t *testing.T) {
	for _, op := range []string{"user", "requester", "owner", "kb", "inactive-user"} {
		t.Run(op, func(t *testing.T) {
			f := newSemanticScopeFixture(t)
			s := f.Issue("member-a", 20, "shared-kb", types.SemanticAccessPurposeSearch, "budget")
			var sql string
			switch op {
			case "user":
				sql = "UPDATE users SET deleted_at=CURRENT_TIMESTAMP WHERE id='member-a'"
			case "requester":
				sql = "UPDATE tenants SET deleted_at=CURRENT_TIMESTAMP WHERE id=20"
			case "owner":
				sql = "UPDATE tenants SET deleted_at=CURRENT_TIMESTAMP WHERE id=10"
			case "kb":
				sql = "UPDATE knowledge_bases SET deleted_at=CURRENT_TIMESTAMP WHERE id='shared-kb'"
			case "inactive-user":
				sql = "UPDATE users SET is_active=false WHERE id='member-a'"
			}
			require.NoError(t, f.DB.Exec(sql).Error)
			require.ErrorIs(t, f.Service.ValidateDelivery(f.Context, s), ErrSemanticScopeChanged)
			_, err := f.Service.Issue(f.Context, "member-a", s.Scope, s.Purpose, s.BudgetRef)
			require.Error(t, err)
		})
	}
}
func TestSemanticScopeRoleChangeWithUnchangedReadSet(t *testing.T) {
	f := newSemanticScopeFixture(t)
	s := f.Issue("member-a", 10, "kb-a", types.SemanticAccessPurposeSearch, "budget")
	f.UpdateTenantRole("member-a", 10, types.TenantRoleViewer)
	require.ErrorIs(t, f.Service.ValidateDelivery(f.Context, s), ErrSemanticScopeChanged)
}
func (f *semanticScopeFixture) AddSharedKB(owner, member uint64, kb string) {
	f.t.Helper()
	f.AddOwnedKB(owner, kb)
	require.NoError(f.t, f.DB.Exec("INSERT INTO organizations(id,name,owner_id) VALUES('scope-org','scope-org','owner') ON CONFLICT(id) DO NOTHING").Error)
	require.NoError(f.t, f.DB.Exec("INSERT INTO organization_tenant_members(id,organization_id,tenant_id,role) VALUES('scope-member','scope-org',?,'editor') ON CONFLICT(id) DO NOTHING", member).Error)
	require.NoError(f.t, f.DB.Exec("INSERT INTO kb_shares(id,knowledge_base_id,organization_id,shared_by_user_id,source_tenant_id,permission) VALUES(?,?,'scope-org','owner',?,'editor') ON CONFLICT(id) DO NOTHING", kb, kb, owner).Error)
}
func (f *semanticScopeFixture) AddActiveSemanticDocument(tenant uint64, kb, doc string, revision uint64) {
	f.t.Helper()
	require.NoError(f.t, f.DB.Exec("INSERT INTO knowledges(id,tenant_id,knowledge_base_id,type,title,source) VALUES(?,?,?,'file',?,'test')", doc, tenant, kb, doc).Error)
	require.NoError(f.t, f.DB.Exec("INSERT INTO semantic_document_revisions(tenant_id,kb_id,document_id,revision,content_hash,deleted) VALUES(?,?,?,?, 'hash',false)", tenant, kb, doc, revision).Error)
}
func (f *semanticScopeFixture) AddDeniedDocument(tenant uint64, kb, doc string, revision uint64) {
	f.t.Helper()
	require.NoError(f.t, f.DB.Exec("INSERT INTO semantic_denials(tenant_id,kb_id,document_id,revision) VALUES(?,?,?,?)", tenant, kb, doc, revision).Error)
}
func (f *semanticScopeFixture) Issue(user string, tenant uint64, kb string, purpose types.SemanticAccessPurpose, budget string) types.SemanticAccessScope {
	f.t.Helper()
	var owner uint64
	require.NoError(f.t, f.DB.Raw("SELECT tenant_id FROM knowledge_bases WHERE id=?", kb).Row().Scan(&owner))
	f.Context = types.WithCaller(context.Background(), types.Caller{UserID: user, TenantID: tenant})
	s, err := f.Service.Issue(f.Context, user, types.SemanticScopeKey{TenantID: owner, KBID: kb}, purpose, budget)
	require.NoError(f.t, err)
	return s
}
func (f *semanticScopeFixture) BumpEpoch(scope types.SemanticScopeKey) {
	f.t.Helper()
	require.NoError(f.t, f.Control.InvalidateKB(f.Context, scope.TenantID, scope.KBID))
}
func (f *semanticScopeFixture) ResolveScope(ref string) (SemanticScopeSnapshot, error) {
	return f.Service.Resolve(f.Context, ref)
}
func (f *semanticScopeFixture) IssueWithExpiry(user string, tenant uint64, kb string, purpose types.SemanticAccessPurpose, budget string, expiry time.Time) types.SemanticAccessScope {
	s := f.Issue(user, tenant, kb, purpose, budget)
	s.ScopeRef = mutateSemanticClaims(f.t, s.ScopeRef, func(c jwt.MapClaims) { c["exp"] = expiry.Unix(); c["iat"] = expiry.Add(-time.Minute).Unix() })
	return s
}
func mutateSemanticClaims(t *testing.T, ref string, mutate func(jwt.MapClaims)) string {
	t.Helper()
	token, err := jwt.Parse(ref, func(*jwt.Token) (any, error) { return []byte(semanticTestKey), nil })
	require.NoError(t, err)
	claims := token.Claims.(jwt.MapClaims)
	mutate(claims)
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(semanticTestKey))
	require.NoError(t, err)
	return signed
}

func TestSemanticScopeRejectsEpochChange(t *testing.T) {
	f := newSemanticScopeFixture(t)
	s := f.Issue("member-a", 20, "shared-kb", types.SemanticAccessPurposeSearch, "budget")
	f.BumpEpoch(s.Scope)
	require.ErrorIs(t, f.Service.ValidateDelivery(f.Context, s), ErrSemanticScopeChanged)
}
func TestSemanticScopeSharedKBUsesOwnerTenantAndExcludesDeniedRows(t *testing.T) {
	f := newSemanticScopeFixture(t)
	f.AddActiveSemanticDocument(10, "shared-kb", "visible", 4)
	f.AddActiveSemanticDocument(10, "shared-kb", "deleted", 5)
	f.AddDeniedDocument(10, "shared-kb", "deleted", 5)
	s := f.Issue("member-a", 20, "shared-kb", types.SemanticAccessPurposeSearch, "budget")
	snap, err := f.ResolveScope(s.ScopeRef)
	require.NoError(t, err)
	require.Equal(t, uint64(10), snap.Scope.TenantID)
	require.Equal(t, []string{"visible"}, snap.AllowedDocumentIDs)
	require.Equal(t, uint64(4), snap.MaxSourceRevisions["visible"])
	require.True(t, snap.AllowRetainedPrevious["visible"])
	require.NoError(t, f.Service.ValidateDelivery(f.Context, s))
}
func TestSemanticScopeRejectsTamperingAndExpiry(t *testing.T) {
	f := newSemanticScopeFixture(t)
	s := f.Issue("member-a", 10, "kb-a", types.SemanticAccessPurposeSearch, "budget")
	for _, change := range []func(*types.SemanticAccessScope){func(s *types.SemanticAccessScope) { s.ScopeHash = "forged" }, func(s *types.SemanticAccessScope) { s.SubjectID = "other" }, func(s *types.SemanticAccessScope) { s.Scope.TenantID = 20 }, func(s *types.SemanticAccessScope) { s.Audience = "other" }, func(s *types.SemanticAccessScope) { s.Purpose = types.SemanticAccessPurposeReason }, func(s *types.SemanticAccessScope) { s.BudgetRef = "other" }, func(s *types.SemanticAccessScope) { s.PermissionEpoch++ }, func(s *types.SemanticAccessScope) { s.ExpiresAt = "2050-01-01T00:00:00Z" }} {
		tampered := s
		change(&tampered)
		require.ErrorIs(t, f.Service.ValidateDelivery(f.Context, tampered), ErrSemanticScopeInvalid)
	}
	_, err := f.ResolveScope(s.ScopeRef + "x")
	require.ErrorIs(t, err, ErrSemanticScopeInvalid)
	expired := f.IssueWithExpiry("member-a", 10, "kb-a", types.SemanticAccessPurposeSearch, "budget", time.Unix(1, 0))
	_, err = f.ResolveScope(expired.ScopeRef)
	require.ErrorIs(t, err, ErrSemanticScopeExpired)
}
func TestSemanticScopeRejectsInvalidClaims(t *testing.T) {
	f := newSemanticScopeFixture(t)
	s := f.Issue("member-a", 10, "kb-a", types.SemanticAccessPurposeSearch, "budget")
	for _, mutate := range []func(jwt.MapClaims){func(c jwt.MapClaims) { c["aud"] = "wrong" }, func(c jwt.MapClaims) { c["version"] = 99 }, func(c jwt.MapClaims) { c["purpose"] = "unknown" }, func(c jwt.MapClaims) { delete(c, "exp") }, func(c jwt.MapClaims) { c["exp"] = time.Now().Add(time.Hour).Unix() }} {
		_, err := f.ResolveScope(mutateSemanticClaims(t, s.ScopeRef, mutate))
		require.Error(t, err)
	}
}
func TestSemanticScopeRejectsLiveChangesWithoutEpoch(t *testing.T) {
	for _, mutation := range []string{"role", "removed", "denied", "moved", "org-removed"} {
		t.Run(mutation, func(t *testing.T) {
			f := newSemanticScopeFixture(t)
			f.AddActiveSemanticDocument(10, "shared-kb", "doc", 1)
			s := f.Issue("member-a", 20, "shared-kb", types.SemanticAccessPurposeSearch, "budget")
			switch mutation {
			case "role":
				require.NoError(t, f.DB.Exec("UPDATE tenant_members SET role='viewer' WHERE tenant_id=20").Error)
			case "removed":
				require.NoError(t, f.DB.Exec("DELETE FROM tenant_members WHERE tenant_id=20").Error)
			case "denied":
				f.AddDeniedDocument(10, "shared-kb", "doc", 2)
			case "moved":
				require.NoError(t, f.DB.Exec("UPDATE knowledges SET knowledge_base_id='kb-a' WHERE id='doc'").Error)
			case "org-removed":
				require.NoError(t, f.DB.Exec("DELETE FROM organization_tenant_members").Error)
			}
			require.Error(t, f.Service.ValidateDelivery(f.Context, s))
		})
	}
}
func TestSemanticScopeRejectsUntrustedCallerAndOwner(t *testing.T) {
	f := newSemanticScopeFixture(t)
	ctx := types.WithCaller(f.Context, types.Caller{TenantID: 10, UserID: "member-a"})
	_, err := f.Service.Issue(ctx, "other", types.SemanticScopeKey{TenantID: 10, KBID: "kb-a"}, types.SemanticAccessPurposeSearch, "budget")
	require.ErrorIs(t, err, ErrSemanticScopeInvalid)
	_, err = f.Service.Issue(ctx, "member-a", types.SemanticScopeKey{TenantID: 20, KBID: "kb-a"}, types.SemanticAccessPurposeSearch, "budget")
	require.Error(t, err)
	require.NoError(t, f.DB.Exec("UPDATE knowledge_bases SET is_temporary=true WHERE id='kb-a'").Error)
	_, err = f.Service.Issue(ctx, "member-a", types.SemanticScopeKey{TenantID: 10, KBID: "kb-a"}, types.SemanticAccessPurposeSearch, "budget")
	require.Error(t, err)
}
func TestSemanticScopeEmptyAndPermissionFailure(t *testing.T) {
	f := newSemanticScopeFixture(t)
	require.NoError(t, f.DB.Exec("INSERT INTO knowledges(id,tenant_id,knowledge_base_id,type,title,source) VALUES('unindexed',10,'kb-a','file','unindexed','test')").Error)
	s := f.Issue("member-a", 10, "kb-a", types.SemanticAccessPurposeSearch, "budget")
	snap, err := f.ResolveScope(s.ScopeRef)
	require.NoError(t, err)
	require.Empty(t, snap.AllowedDocumentIDs)
	require.NoError(t, f.DB.Exec("DROP TABLE tenant_members").Error)
	_, err = f.ResolveScope(s.ScopeRef)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "tenant_members")
	require.NotContains(t, err.Error(), semanticTestKey)
}
func TestSemanticScopeFailsClosedOnOrganizationLookupError(t *testing.T) {
	f := newSemanticScopeFixture(t)
	s := f.Issue("member-a", 20, "shared-kb", types.SemanticAccessPurposeSearch, "budget")
	require.NoError(t, f.DB.Exec("DROP TABLE organization_tenant_members").Error)
	_, err := f.ResolveScope(s.ScopeRef)
	require.Error(t, err)
	require.False(t, strings.Contains(err.Error(), "SELECT"))
}
