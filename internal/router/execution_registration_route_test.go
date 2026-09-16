package router

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/execution"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type routeTargetStore struct{ revoked []string }

func (*routeTargetStore) CreateTarget(context.Context, execution.Target, string) error { return nil }
func (*routeTargetStore) CreateTargetIfTrusted(context.Context, execution.Target, string) error {
	return nil
}
func (*routeTargetStore) GetOwnedTarget(context.Context, uint64, string, string) (execution.Target, error) {
	return execution.Target{}, nil
}
func (*routeTargetStore) ListOwnedTargets(context.Context, uint64, string) ([]execution.Target, error) {
	return nil, nil
}
func (s *routeTargetStore) RevokeTarget(_ context.Context, _ uint64, _ string, id string) error {
	s.revoked = append(s.revoked, id)
	return nil
}
func (*routeTargetStore) CreateWorkspace(context.Context, execution.Workspace) error { return nil }
func (*routeTargetStore) GetOwnedWorkspace(context.Context, uint64, string, string) (execution.Workspace, error) {
	return execution.Workspace{}, nil
}

func TestExecutionRegistrationRoutesUseTargetFacadeForContractRevoke(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &routeTargetStore{}
	target := handler.NewExecutionTargetHandler(store, nil)
	registration := handler.NewExecutionRegistrationHandler(nil)
	g := &rbacGuards{}
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "owner")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	api := engine.Group("/api/v1")
	RegisterExecutionRegistrationRoutes(api, registration, g, target)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/execution-targets/node-1/revoke", nil)
	res := httptest.NewRecorder()
	engine.ServeHTTP(res, req)
	require.Equal(t, http.StatusNoContent, res.Code)
	require.Equal(t, []string{"node-1"}, store.revoked)
}

func TestExecutionRegistrationRoutesUseRealRegistrationTargetRevokeLifecycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	for _, ddl := range []string{
		`CREATE TABLE execution_registration_challenges (tenant_id integer, owner_id text, challenge_id text, runtime_id text, external_target_id text, public_key_fingerprint text, nonce_hash text, nonce text, expires_at datetime, consumed_at datetime, PRIMARY KEY (tenant_id, owner_id, challenge_id))`,
		`CREATE TABLE execution_registrations (tenant_id integer, owner_id text, registration_id text, runtime_id text, external_target_id text, public_key_fingerprint text, credential_version integer, state text, idempotency_key text, request_hash text, created_at datetime, revoked_at datetime, PRIMARY KEY (tenant_id, owner_id, registration_id))`,
		`CREATE TABLE execution_target_identities (tenant_id integer, runtime_id text, external_target_id text, owner_id text, credential_version integer, state text, PRIMARY KEY (tenant_id, runtime_id, external_target_id))`,
		`CREATE TABLE execution_targets (tenant_id integer, id text, owner_id text, kind text, state text, credential_version integer, runtime_id text, external_target_id text, root_ref text, revoked_at datetime, PRIMARY KEY (tenant_id, id))`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	svc := execution.NewRegistrationService(db, repository.NewPersonalTargetProvisioner(db))
	registration := handler.NewExecutionRegistrationHandler(svc)
	target := handler.NewExecutionTargetHandler(repository.NewExecutionTargetStore(db), repository.NewExecutionTargetIdentityProvider(db))
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		tenant := uint64(1)
		if c.GetHeader("X-Test-Tenant") == "2" {
			tenant = 2
		}
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, tenant)
		owner := "u1"
		if value := c.GetHeader("X-Test-Owner"); value != "" {
			owner = value
		}
		ctx = context.WithValue(ctx, types.UserIDContextKey, owner)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		if len(c.Errors) > 0 && !c.IsAborted() {
			c.JSON(http.StatusNotFound, gin.H{"error": c.Errors.Last().Error()})
		}
	})
	RegisterExecutionRegistrationRoutes(engine.Group("/api/v1"), registration, &rbacGuards{}, target)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	publicText := base64.RawURLEncoding.EncodeToString(public)
	body, _ := json.Marshal(map[string]string{"runtime_id": "runtime-1", "external_target_id": "external-1", "public_key": publicText})
	res := httptest.NewRecorder()
	engine.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/execution-targets/registrations/challenges", bytes.NewReader(body)))
	require.Equal(t, http.StatusCreated, res.Code)
	var challenge struct {
		Data execution.NodeChallenge `json:"data"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &challenge))
	nonce, err := base64.RawURLEncoding.DecodeString(challenge.Data.Nonce)
	require.NoError(t, err)
	complete, _ := json.Marshal(execution.NodeRegistrationRequest{ChallengeID: challenge.Data.ID, Nonce: challenge.Data.Nonce, RuntimeID: "runtime-1", ExternalTargetID: "external-1", PublicKey: publicText, Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, nonce)), IdempotencyKey: "route-lifecycle", Confirmed: true})
	res = httptest.NewRecorder()
	engine.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/execution-targets/registrations", bytes.NewReader(complete)))
	require.Equal(t, http.StatusCreated, res.Code)
	var completed struct {
		Data struct {
			Registration execution.NodeRegistration `json:"registration"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &completed))
	require.NotEmpty(t, completed.Data.Registration.ID)

	res = httptest.NewRecorder()
	engine.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/execution-targets/"+completed.Data.Registration.ID+"/revoke", nil))
	require.Equal(t, http.StatusNoContent, res.Code)
	for _, table := range []string{"execution_registrations", "execution_targets", "execution_target_identities"} {
		var state string
		require.NoError(t, db.Raw("SELECT state FROM "+table).Row().Scan(&state))
		require.Equal(t, "revoked", state, table)
	}
	res = httptest.NewRecorder()
	crossTenant := httptest.NewRequest(http.MethodPost, "/api/v1/execution-targets/"+completed.Data.Registration.ID+"/revoke", nil)
	crossTenant.Header.Set("X-Test-Tenant", "2")
	engine.ServeHTTP(res, crossTenant)
	require.Equal(t, http.StatusNotFound, res.Code)
	res = httptest.NewRecorder()
	crossOwner := httptest.NewRequest(http.MethodPost, "/api/v1/execution-targets/"+completed.Data.Registration.ID+"/revoke", nil)
	crossOwner.Header.Set("X-Test-Owner", "u2")
	engine.ServeHTTP(res, crossOwner)
	require.Equal(t, http.StatusNotFound, res.Code)
	// A replay of the original completion idempotency key after revoke must be
	// rejected and must not issue a fresh active grant.
	res = httptest.NewRecorder()
	replay := httptest.NewRequest(http.MethodPost, "/api/v1/execution-targets/registrations", bytes.NewReader(complete))
	engine.ServeHTTP(res, replay)
	require.Equal(t, http.StatusNotFound, res.Code)
	res = httptest.NewRecorder()
	engine.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/execution-targets/"+completed.Data.Registration.ID+"/revoke", nil))
	require.Equal(t, http.StatusNotFound, res.Code)
}
