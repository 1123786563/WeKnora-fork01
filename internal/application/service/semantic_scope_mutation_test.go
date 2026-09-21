package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/policy/access"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

func TestSemanticScopeTenantMutation(t *testing.T) {
	for _, op := range []string{"add", "ensure-owner", "role", "remove", "demote-owner", "remove-owner", "noop", "barrier-failure", "mutation-failure"} {
		t.Run(op, func(t *testing.T) {
			f := newSemanticScopeFixture(t)
			s := NewTenantMemberService(repository.NewTenantMemberRepository(f.DB), nil, nil, nil).(*tenantMemberService)
			s.SetSemanticScopeInvalidator(f.Control)
			operation := "UPDATE"
			if op == "add" || op == "ensure-owner" {
				operation = "INSERT"
			}
			requireEpochBeforeSQL(t, f, "tenant_members", operation, "shared-kb", 1)
			var err error
			switch op {
			case "add":
				_, err = s.AddMember(f.Context, "new-member", 20, types.TenantRoleViewer, nil)
			case "ensure-owner":
				_, err = s.EnsureOwner(f.Context, "new-owner", 20)
			case "role":
				err = s.UpdateRole(f.Context, "member-a", 20, types.TenantRoleViewer)
			case "remove":
				err = s.RemoveMember(f.Context, "member-a", 20)
			case "demote-owner", "remove-owner":
				require.NoError(t, f.DB.Exec("DROP TRIGGER semantic_order").Error)
				require.NoError(t, f.DB.Exec("UPDATE tenant_members SET role='owner' WHERE tenant_id=20").Error)
				f.AddMember("owner-b", 20, types.TenantRoleOwner)
				requireEpochBeforeSQL(t, f, "tenant_members", "UPDATE", "shared-kb", 1)
				if op == "demote-owner" {
					err = s.UpdateRole(f.Context, "member-a", 20, types.TenantRoleViewer)
				} else {
					err = s.RemoveMember(f.Context, "member-a", 20)
				}
			case "noop":
				err = s.UpdateRole(f.Context, "member-a", 20, types.TenantRoleContributor)
				require.NoError(t, err)
				require.Zero(t, semanticEpoch(t, f, "shared-kb"))
				return
			case "barrier-failure":
				s.SetSemanticScopeInvalidator(failingSemanticInvalidator{errors.New("epoch failed")})
				err = s.UpdateRole(f.Context, "member-a", 20, types.TenantRoleViewer)
				require.Error(t, err)
				require.Zero(t, semanticEpoch(t, f, "shared-kb"))
				return
			case "mutation-failure":
				require.NoError(t, f.DB.Exec("CREATE TRIGGER reject_member BEFORE UPDATE ON tenant_members BEGIN SELECT RAISE(ABORT,'write failed'); END").Error)
				err = s.UpdateRole(f.Context, "member-a", 20, types.TenantRoleViewer)
				require.Error(t, err)
				require.Equal(t, 1, semanticEpoch(t, f, "shared-kb"))
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, semanticEpoch(t, f, "shared-kb"))
			require.Zero(t, semanticEpoch(t, f, "kb-a"))
		})
	}
}

func TestSemanticScopeOrganizationMutation(t *testing.T) {
	for _, op := range []string{"add", "join", "leave", "remove", "role", "approve-join", "approve-upgrade", "reject", "terminal", "delete", "barrier-failure"} {
		t.Run(op, func(t *testing.T) {
			f := newSemanticScopeFixture(t)
			require.NoError(t, f.DB.Exec("UPDATE organizations SET owner_tenant_id=10 WHERE id='scope-org'").Error)
			require.NoError(t, f.DB.Exec("INSERT INTO organization_tenant_members(id,organization_id,tenant_id,role) VALUES('admin','scope-org',10,'admin')").Error)
			s := NewOrganizationService(repository.NewOrganizationRepository(f.DB), nil, repository.NewKBShareRepository(f.DB), repository.NewAgentShareRepository(f.DB)).(*organizationService)
			s.SetSemanticScopeInvalidator(f.Control)
			var err error
			switch op {
			case "add":
				requireEpochBeforeSQL(t, f, "organization_tenant_members", "INSERT", "shared-kb", 1)
				err = s.AddTenantMember(f.Context, "scope-org", 30, "user", types.OrgRoleViewer)
			case "join":
				requireEpochBeforeSQL(t, f, "organization_tenant_members", "INSERT", "shared-kb", 1)
				org, e := s.orgRepo.GetByID(f.Context, "scope-org")
				require.NoError(t, e)
				err = s.joinAsViewerWithChecks(f.Context, org, "user", 30)
			case "leave", "remove":
				requireEpochBeforeSQL(t, f, "organization_tenant_members", "DELETE", "shared-kb", 1)
				operator := uint64(10)
				if op == "leave" {
					operator = 20
				}
				err = s.RemoveTenantMember(f.Context, "scope-org", 20, "owner", operator)
			case "role":
				requireEpochBeforeSQL(t, f, "organization_tenant_members", "UPDATE", "shared-kb", 1)
				err = s.UpdateTenantMemberRole(f.Context, "scope-org", 20, types.OrgRoleViewer, "owner", 10)
			case "approve-join", "approve-upgrade", "reject", "terminal":
				tenant := 30
				kind := "join"
				sqlOp := "INSERT"
				if op == "approve-upgrade" {
					tenant = 20
					kind = "upgrade"
					sqlOp = "UPDATE"
				}
				require.NoError(t, f.DB.Exec("INSERT INTO organization_join_requests(id,organization_id,user_id,tenant_id,request_type,requested_role) VALUES('request','scope-org','user',?,?,'viewer')", tenant, kind).Error)
				if op == "terminal" {
					require.NoError(t, f.DB.Exec("UPDATE organization_join_requests SET status='rejected'").Error)
				}
				requireEpochBeforeSQL(t, f, "organization_tenant_members", sqlOp, "shared-kb", 1)
				err = s.ReviewJoinRequest(f.Context, "scope-org", "request", op != "reject", "owner", 10, "", nil)
				if op == "reject" || op == "terminal" {
					if op == "reject" {
						require.NoError(t, err)
					} else {
						require.Error(t, err)
					}
					require.Zero(t, semanticEpoch(t, f, "shared-kb"))
					return
				}
			case "delete":
				requireEpochBeforeSQL(t, f, "kb_shares", "UPDATE", "shared-kb", 1)
				err = s.DeleteOrganization(f.Context, "scope-org", "owner", 10)
			case "barrier-failure":
				s.SetSemanticScopeInvalidator(failingSemanticInvalidator{errors.New("epoch failed")})
				err = s.RemoveTenantMember(f.Context, "scope-org", 20, "member-a", 20)
				require.Error(t, err)
				member, e := s.orgRepo.GetTenantMember(f.Context, "scope-org", 20)
				require.NoError(t, e)
				require.NotNil(t, member)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, semanticEpoch(t, f, "shared-kb"))
			require.Zero(t, semanticEpoch(t, f, "kb-a"))
		})
	}
}

type recordingSemanticInvalidator struct {
	failingSemanticInvalidator
	scopes []types.SemanticScopeKey
}

func (r *recordingSemanticInvalidator) InvalidateTransfer(_ context.Context, source, destination types.SemanticScopeKey) error {
	r.scopes = append(r.scopes, source, destination)
	return r.err
}

func TestSemanticScopeTransferFailsBeforeCheckpoint(t *testing.T) {
	for _, operation := range []access.KBTransferOperation{access.KBTransferMove, access.KBTransferClone} {
		t.Run(string(operation), func(t *testing.T) {
			f := transferFixture(t, operation)
			guard := &recordingSemanticInvalidator{failingSemanticInvalidator: failingSemanticInvalidator{errors.New("epoch failed")}}
			f.svc.SetSemanticScopeInvalidator(guard)
			var err error
			if operation == access.KBTransferMove {
				err = f.svc.ProcessKnowledgeMove(context.Background(), moveTask(t, []string{"doc"}, "reuse_vectors"))
			} else {
				err = f.svc.executeKnowledgeClone(f.ctx, f.kbs.values["kb"], f.kbs.values["other"], nil)
			}
			require.ErrorIs(t, err, guard.err)
			require.Equal(t, []types.SemanticScopeKey{{TenantID: 7, KBID: "kb"}, {TenantID: 7, KBID: "other"}}, guard.scopes)
			row, err := f.repo.GetKnowledgeByID(f.ctx, 7, "doc")
			require.NoError(t, err)
			require.Empty(t, row.Metadata)
			require.Equal(t, "kb", row.KnowledgeBaseID)
			require.Zero(t, f.chunkRepo.writes)
		})
	}
}

func TestSemanticScopeTransferFolderMoveExcluded(t *testing.T) {
	repo := &folderMoveRepoStub{}
	svc := &knowledgeService{repo: repo, kbService: &writeKBLookup{kb: &types.KnowledgeBase{ID: "kb-1", TenantID: 1}}}
	svc.SetSemanticScopeInvalidator(failingSemanticInvalidator{errors.New("must not invalidate")})
	n, err := svc.MoveKnowledgeToFolder(folderMoveContext(), "kb-1", []string{"k1"}, "docs")
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
}

func TestSemanticScopeTransferFAQFailsBeforeCheckpoint(t *testing.T) {
	f := transferFixture(t, access.KBTransferClone)
	require.NoError(t, f.db.Model(&types.Knowledge{}).Where("tenant_id = ?", 7).Update("type", types.KnowledgeTypeFAQ).Error)
	require.NoError(t, f.db.Model(&types.Chunk{}).Where("tenant_id = ?", 7).Update("chunk_type", types.ChunkTypeFAQ).Error)
	f.kbs.values["kb"].Type = types.KnowledgeBaseTypeFAQ
	f.kbs.values["other"].Type = types.KnowledgeBaseTypeFAQ
	guard := &recordingSemanticInvalidator{failingSemanticInvalidator: failingSemanticInvalidator{errors.New("epoch failed")}}
	f.svc.SetSemanticScopeInvalidator(guard)
	err := f.svc.cloneFAQKnowledgeBase(f.ctx, f.kbs.values["kb"], f.kbs.values["other"], &types.KBCloneProgress{}, func(*types.KBCloneProgress, error, string) {})
	require.ErrorIs(t, err, guard.err)
	require.Equal(t, []types.SemanticScopeKey{{TenantID: 7, KBID: "kb"}, {TenantID: 7, KBID: "other"}}, guard.scopes)
	require.Zero(t, f.chunkRepo.writes)
}

type semanticTransferOrderingRepo struct {
	interfaces.KnowledgeRepository
	t      *testing.T
	guard  *recordingSemanticInvalidator
	writes int
}

func (r *semanticTransferOrderingRepo) check() {
	r.t.Helper()
	require.Equal(r.t, []types.SemanticScopeKey{{TenantID: 7, KBID: "kb"}, {TenantID: 7, KBID: "other"}}, r.guard.scopes)
	r.writes++
}

func (r *semanticTransferOrderingRepo) CreateKnowledge(ctx context.Context, k *types.Knowledge) error {
	r.check()
	return r.KnowledgeRepository.CreateKnowledge(ctx, k)
}

func (r *semanticTransferOrderingRepo) UpdateKnowledgeForTransfer(ctx context.Context, before, after *types.Knowledge) error {
	r.check()
	return r.KnowledgeRepository.UpdateKnowledgeForTransfer(ctx, before, after)
}

func TestSemanticScopeTransferBeforeSuccessfulCheckpoint(t *testing.T) {
	for _, operation := range []access.KBTransferOperation{access.KBTransferMove, access.KBTransferClone} {
		t.Run(string(operation), func(t *testing.T) {
			f := transferFixture(t, operation)
			guard := &recordingSemanticInvalidator{}
			f.svc.SetSemanticScopeInvalidator(guard)
			repo := &semanticTransferOrderingRepo{KnowledgeRepository: f.repo, t: t, guard: guard}
			f.svc.repo = repo
			var err error
			if operation == access.KBTransferMove {
				err = f.svc.ProcessKnowledgeMove(context.Background(), moveTask(t, []string{"doc"}, "reuse_vectors"))
			} else {
				err = f.svc.executeKnowledgeClone(f.ctx, f.kbs.values["kb"], f.kbs.values["other"], nil)
			}
			require.NoError(t, err)
			require.Greater(t, repo.writes, 0)
		})
	}
}

func TestSemanticScopeInvitationRevokeDoesNotInvalidate(t *testing.T) {
	f := newSemanticScopeFixture(t)
	members := NewTenantMemberService(repository.NewTenantMemberRepository(f.DB), nil, nil, nil).(*tenantMemberService)
	members.SetSemanticScopeInvalidator(failingSemanticInvalidator{errors.New("must not invalidate")})
	invitations := NewTenantInvitationService(newFakeInvitationRepo(), members, nil)
	inv, err := invitations.Create(f.Context, 20, "invited-user", types.TenantRoleViewer, nil, "")
	require.NoError(t, err)
	require.NoError(t, invitations.Revoke(f.Context, inv.ID))
	require.Zero(t, semanticEpoch(t, f, "shared-kb"))
}

func TestSemanticScopeInvitationAcceptance(t *testing.T) {
	f := newSemanticScopeFixture(t)
	members := NewTenantMemberService(repository.NewTenantMemberRepository(f.DB), nil, nil, nil).(*tenantMemberService)
	members.SetSemanticScopeInvalidator(f.Control)
	invitations := NewTenantInvitationService(newFakeInvitationRepo(), members, nil)
	inv, err := invitations.Create(f.Context, 20, "invited-user", types.TenantRoleViewer, nil, "")
	require.NoError(t, err)
	require.Zero(t, semanticEpoch(t, f, "shared-kb"))
	requireEpochBeforeSQL(t, f, "tenant_members", "INSERT", "shared-kb", 1)
	_, err = invitations.Accept(f.Context, inv.ID, "invited-user")
	require.NoError(t, err)
	require.Equal(t, 1, semanticEpoch(t, f, "shared-kb"))
}

// A real SQL write fails if its semantic epoch was not committed first.
func requireEpochBeforeSQL(t *testing.T, f *semanticScopeFixture, table, operation, kb string, epoch int) {
	t.Helper()
	sql := fmt.Sprintf("CREATE TRIGGER semantic_order BEFORE %s ON %s BEGIN SELECT CASE WHEN COALESCE((SELECT CAST(epoch AS INTEGER) FROM semantic_access_epochs WHERE tenant_id='10' AND kb_id='%s'),0) < %d THEN RAISE(ABORT,'semantic barrier missing') END; END", operation, table, kb, epoch)
	require.NoError(t, f.DB.Exec(sql).Error)
}

func semanticEpoch(t *testing.T, f *semanticScopeFixture, kb string) int {
	t.Helper()
	var epoch int
	require.NoError(t, f.DB.Raw("SELECT COALESCE((SELECT CAST(epoch AS INTEGER) FROM semantic_access_epochs WHERE tenant_id='10' AND kb_id=?),0)", kb).Row().Scan(&epoch))
	return epoch
}

func TestSemanticScopeKBSharePreInvalidation(t *testing.T) {
	for _, op := range []string{"create", "duplicate", "update", "remove", "noop"} {
		t.Run(op, func(t *testing.T) {
			f := newSemanticScopeFixture(t)
			require.NoError(t, f.DB.Exec("INSERT INTO organization_tenant_members(id,organization_id,tenant_id,role) VALUES('source','scope-org',10,'admin')").Error)
			s := NewKBShareService(repository.NewKBShareRepository(f.DB), repository.NewOrganizationRepository(f.DB), repository.NewKnowledgeBaseRepository(f.DB), repository.NewKnowledgeRepository(f.DB), nil, nil).(*kbShareService)
			s.SetSemanticScopeInvalidator(f.Control)
			ctx := context.Background()
			switch op {
			case "create":
				requireEpochBeforeSQL(t, f, "kb_shares", "INSERT", "kb-a", 1)
				_, err := s.ShareKnowledgeBase(ctx, "kb-a", "scope-org", "owner", 10, types.OrgRoleViewer)
				require.NoError(t, err)
				require.Equal(t, 1, semanticEpoch(t, f, "kb-a"))
			case "duplicate":
				requireEpochBeforeSQL(t, f, "kb_shares", "UPDATE", "shared-kb", 1)
				_, err := s.ShareKnowledgeBase(ctx, "shared-kb", "scope-org", "owner", 10, types.OrgRoleViewer)
				require.NoError(t, err)
			case "update":
				requireEpochBeforeSQL(t, f, "kb_shares", "UPDATE", "shared-kb", 1)
				require.NoError(t, s.UpdateSharePermission(ctx, "shared-kb", types.OrgRoleViewer, "owner", 10))
			case "remove":
				requireEpochBeforeSQL(t, f, "kb_shares", "UPDATE", "shared-kb", 1)
				require.NoError(t, s.RemoveShare(ctx, "shared-kb", "owner", 10))
			case "noop":
				require.NoError(t, s.UpdateSharePermission(ctx, "shared-kb", types.OrgRoleEditor, "owner", 10))
				require.Equal(t, 0, semanticEpoch(t, f, "shared-kb"))
				return
			}
			if op != "create" {
				require.Equal(t, 1, semanticEpoch(t, f, "shared-kb"))
			}
		})
	}
}

func TestSemanticScopeKBShareMutationFailureAndBarrierFailure(t *testing.T) {
	for _, failure := range []string{"mutation", "barrier"} {
		t.Run(failure, func(t *testing.T) {
			f := newSemanticScopeFixture(t)
			s := NewKBShareService(repository.NewKBShareRepository(f.DB), repository.NewOrganizationRepository(f.DB), repository.NewKnowledgeBaseRepository(f.DB), nil, nil, nil).(*kbShareService)
			s.SetSemanticScopeInvalidator(f.Control)
			if failure == "mutation" {
				require.NoError(t, f.DB.Exec("CREATE TRIGGER reject_share BEFORE UPDATE ON kb_shares BEGIN SELECT RAISE(ABORT,'write failed'); END").Error)
			} else {
				require.NoError(t, f.DB.Exec("DROP TABLE semantic_access_epochs").Error)
			}
			require.Error(t, s.UpdateSharePermission(context.Background(), "shared-kb", types.OrgRoleViewer, "owner", 10))
			var permission string
			require.NoError(t, f.DB.Raw("SELECT permission FROM kb_shares WHERE id='shared-kb'").Row().Scan(&permission))
			require.Equal(t, "editor", permission)
			if failure == "mutation" {
				require.Equal(t, 1, semanticEpoch(t, f, "shared-kb"))
			}
		})
	}
}

type failingSemanticInvalidator struct{ err error }

func (f failingSemanticInvalidator) InvalidateUser(context.Context, string) error { return f.err }

func (f failingSemanticInvalidator) InvalidateKB(context.Context, uint64, string) error { return f.err }

func (f failingSemanticInvalidator) InvalidateTenant(context.Context, uint64) error { return f.err }

func (f failingSemanticInvalidator) InvalidateOrganization(context.Context, string) error {
	return f.err
}

func (f failingSemanticInvalidator) InvalidateTransfer(context.Context, types.SemanticScopeKey, types.SemanticScopeKey) error {
	return f.err
}

func TestSemanticScopeKBDelete(t *testing.T) {
	for _, failure := range []string{"none", "mutation", "barrier"} {
		t.Run(failure, func(t *testing.T) {
			f := newSemanticScopeFixture(t)
			s := &knowledgeBaseService{repo: repository.NewKnowledgeBaseRepository(f.DB), asynqClient: kbDeleteTaskEnqueuer{}}
			s.SetSemanticScopeInvalidator(f.Control)
			requireEpochBeforeSQL(t, f, "knowledge_bases", "UPDATE", "kb-a", 1)
			if failure == "mutation" {
				require.NoError(t, f.DB.Exec("CREATE TRIGGER reject_kb BEFORE UPDATE ON knowledge_bases BEGIN SELECT RAISE(ABORT,'write failed'); END").Error)
			}
			if failure == "barrier" {
				s.SetSemanticScopeInvalidator(failingSemanticInvalidator{errors.New("epoch unavailable")})
			}
			err := s.DeleteKnowledgeBase(ctxWithTenantStorage(10, "local"), "kb-a")
			var count int64
			require.NoError(t, f.DB.Table("knowledge_bases").Where("id='kb-a' AND deleted_at IS NULL").Count(&count).Error)
			if failure == "none" {
				require.NoError(t, err)
				require.Zero(t, count)
			} else {
				require.Error(t, err)
				require.Equal(t, int64(1), count)
			}
			expected := 1
			if failure == "barrier" {
				expected = 0
			}
			require.Equal(t, expected, semanticEpoch(t, f, "kb-a"))
		})
	}
}

func TestSemanticScopeTenantDelete(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprint(failed), func(t *testing.T) {
			f := newSemanticScopeFixture(t)
			s := &tenantService{repo: repository.NewTenantRepository(f.DB)}
			s.SetSemanticScopeInvalidator(f.Control)
			requireEpochBeforeSQL(t, f, "tenant_members", "UPDATE", "shared-kb", 1)
			if failed {
				s.SetSemanticScopeInvalidator(failingSemanticInvalidator{errors.New("epoch failed")})
			}
			err := s.DeleteTenant(f.Context, 20)
			var count int64
			require.NoError(t, f.DB.Table("tenant_members").Where("tenant_id=20 AND deleted_at IS NULL").Count(&count).Error)
			if failed {
				require.Error(t, err)
				require.Equal(t, int64(1), count)
			} else {
				require.NoError(t, err)
				require.Zero(t, count)
				require.Equal(t, 1, semanticEpoch(t, f, "shared-kb"))
			}
		})
	}
}

func TestSemanticScopeUserDelete(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprint(failed), func(t *testing.T) {
			f := newSemanticScopeFixture(t)
			s := &userService{userRepo: repository.NewUserRepository(f.DB)}
			s.SetSemanticScopeInvalidator(f.Control)
			requireEpochBeforeSQL(t, f, "users", "UPDATE", "shared-kb", 1)
			if failed {
				s.SetSemanticScopeInvalidator(failingSemanticInvalidator{errors.New("epoch failed")})
			}
			err := s.DeleteUser(f.Context, "member-a")
			var count int64
			require.NoError(t, f.DB.Table("users").Where("id='member-a' AND deleted_at IS NULL").Count(&count).Error)
			if failed {
				require.Error(t, err)
				require.Equal(t, int64(1), count)
			} else {
				require.NoError(t, err)
				require.Zero(t, count)
				require.Equal(t, 1, semanticEpoch(t, f, "shared-kb"))
				require.Equal(t, 1, semanticEpoch(t, f, "kb-a"))
			}
		})
	}
}
