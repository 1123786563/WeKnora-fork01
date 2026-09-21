package handler

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
	"github.com/Tencent/WeKnora/internal/modules/execution"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openRegistrationHTTPDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	for _, statement := range []string{
		`CREATE TABLE execution_registration_challenges (tenant_id integer, owner_id text, challenge_id text, runtime_id text, external_target_id text, public_key_fingerprint text, nonce_hash text, nonce text, expires_at datetime, consumed_at datetime, PRIMARY KEY (tenant_id, owner_id, challenge_id))`,
		`CREATE TABLE execution_registrations (tenant_id integer, owner_id text, registration_id text, runtime_id text, external_target_id text, public_key_fingerprint text, credential_version integer, state text, idempotency_key text, request_hash text, created_at datetime, revoked_at datetime, PRIMARY KEY (tenant_id, owner_id, registration_id))`,
		`CREATE TABLE execution_target_identities (tenant_id integer, runtime_id text, external_target_id text, owner_id text, credential_version integer, state text, PRIMARY KEY (tenant_id, runtime_id, external_target_id))`,
		`CREATE TABLE execution_targets (tenant_id integer, id text, owner_id text, kind text, state text, credential_version integer, runtime_id text, external_target_id text, usage_binding_json text NOT NULL DEFAULT '{}', root_ref text, revoked_at datetime, PRIMARY KEY (tenant_id, id))`,
	} {
		require.NoError(t, db.Exec(statement).Error)
	}
	return db
}

func registrationHTTPRouter(h *ExecutionRegistrationHandler) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		if len(c.Errors) > 0 && !c.IsAborted() {
			c.JSON(http.StatusNotFound, gin.H{"error": c.Errors.Last().Error()})
		}
	})
	r.POST("/api/v1/execution-targets/registrations/challenges", h.CreateChallenge)
	r.POST("/api/v1/execution-targets/registrations", h.Complete)
	r.POST("/api/v1/execution-targets/:id/revoke", h.Revoke)
	return r
}

func TestExecutionTargetRegistrationGinSQLiteLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openRegistrationHTTPDB(t)
	svc := execution.NewRegistrationService(db, repository.NewPersonalTargetProvisioner(db))
	r := registrationHTTPRouter(NewExecutionRegistrationHandler(svc))
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	publicText := base64.RawURLEncoding.EncodeToString(public)
	challengeBody, _ := json.Marshal(map[string]string{"runtime_id": "runtime-1", "external_target_id": "external-1", "public_key": publicText})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/execution-targets/registrations/challenges", bytes.NewReader(challengeBody)))
	require.Equal(t, http.StatusCreated, w.Code)
	var challengeResponse struct {
		Data execution.NodeChallenge `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &challengeResponse))
	nonce, err := base64.RawURLEncoding.DecodeString(challengeResponse.Data.Nonce)
	require.NoError(t, err)
	completeBody, _ := json.Marshal(execution.NodeRegistrationRequest{
		ChallengeID: challengeResponse.Data.ID, Nonce: challengeResponse.Data.Nonce, RuntimeID: "runtime-1", ExternalTargetID: "external-1", PublicKey: publicText,
		Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, nonce)), IdempotencyKey: "http-request-1", Confirmed: true,
	})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/execution-targets/registrations", bytes.NewReader(completeBody)))
	require.Equal(t, http.StatusCreated, w.Code)
	var completeResponse struct {
		Data struct {
			Registration execution.NodeRegistration `json:"registration"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &completeResponse))
	require.NotEmpty(t, completeResponse.Data.Registration.ID)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/execution-targets/"+completeResponse.Data.Registration.ID+"/revoke", nil))
	require.Equal(t, http.StatusNoContent, w.Code)
	var state string
	var version int64
	require.NoError(t, db.Raw("SELECT state, credential_version FROM execution_registrations WHERE registration_id = ?", completeResponse.Data.Registration.ID).Row().Scan(&state, &version))
	require.Equal(t, "revoked", state)
	require.EqualValues(t, 2, version)
	require.NoError(t, db.Raw("SELECT state FROM execution_target_identities WHERE runtime_id = ?", "runtime-1").Row().Scan(&state))
	require.Equal(t, "revoked", state)
	// A second revoke is a clean not-found response and cannot bump the fence.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/execution-targets/"+completeResponse.Data.Registration.ID+"/revoke", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}
