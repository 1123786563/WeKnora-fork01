package career

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCareerTenantRequiresSingleActiveOwner(t *testing.T) {
	owner := &types.TenantMember{UserID: "u1", TenantID: 7, Role: types.TenantRoleOwner}
	require.NoError(t, validateOwnerOnlyCareerTenant("u1", 7, []*types.TenantMember{owner}))
	require.ErrorIs(t, validateOwnerOnlyCareerTenant("u1", 7, []*types.TenantMember{owner, {UserID: "u2", TenantID: 7, Role: types.TenantRoleViewer}}), ErrUnauthorized)
	require.ErrorIs(t, validateOwnerOnlyCareerTenant("u2", 7, []*types.TenantMember{owner}), ErrUnauthorized)
	require.ErrorIs(t, validateOwnerOnlyCareerTenant("u1", 7, []*types.TenantMember{{UserID: "u1", TenantID: 7, Role: types.TenantRoleAdmin}}), ErrUnauthorized)
	require.ErrorIs(t, validateOwnerOnlyCareerTenant("u1", 7, nil), ErrUnauthorized)
}

func TestCareerActRejectsClientClaimedResumeExtractionSource(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	scope := Scope{UserID: "u1", TenantID: 7}
	baseCtx := context.WithValue(context.Background(), types.UserIDContextKey, scope.UserID)
	baseCtx = context.WithValue(baseCtx, types.TenantIDContextKey, scope.TenantID)
	require.NoError(t, office.ClaimSpace(WithScope(baseCtx, scope)))
	h := &Handler{office: office, members: &memberListStub{members: []*types.TenantMember{{UserID: "u1", TenantID: 7, Role: types.TenantRoleOwner}}}}
	requestBody := `{"action":"propose","key":"education.school","value":"Forged School","requestId":"forged-1","expectedRevision":0,"source":{"kind":"resume_extraction","referenceId":"client-controlled"}}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/api/v1/career/act", strings.NewReader(requestBody)).WithContext(baseCtx)
	h.Act(c)
	require.Equal(t, 400, recorder.Code)
	view, err := office.Open(WithScope(baseCtx, scope))
	require.NoError(t, err)
	require.Empty(t, view.Proposals)
}

type memberListStub struct{ members []*types.TenantMember }

func (s *memberListStub) ListByTenant(context.Context, uint64) ([]*types.TenantMember, error) {
	return s.members, nil
}
func TestCareerHTTPRechecksTenantMembershipOnEveryRequest(t *testing.T) {
	db, e := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, e)
	o, e := NewOffice(db)
	require.NoError(t, e)
	members := &memberListStub{members: []*types.TenantMember{{UserID: "u1", TenantID: 7, Role: types.TenantRoleOwner}}}
	h := &Handler{office: o, members: members}
	request := func() *httptest.ResponseRecorder {
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		ctx := context.WithValue(context.Background(), types.UserIDContextKey, "u1")
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest("GET", "/api/v1/career/open", nil).WithContext(ctx)
		h.Open(c)
		return rec
	}
	require.Equal(t, 200, request().Code)
	members.members = append(members.members, &types.TenantMember{UserID: "u2", TenantID: 7, Role: types.TenantRoleViewer})
	require.Equal(t, 403, request().Code)
	members.members = nil
	require.Equal(t, 403, request().Code)
}
