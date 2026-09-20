package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCredentialsTestRouter mirrors the production wiring of the credential
// subresource (PUT /datasource/:id/credentials) plus the tenant-context
// middleware the ownDataSource guard reads.
func newCredentialsTestRouter(h *DataSourceCredentialsHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(errorCapture())
	r.Use(func(c *gin.Context) {
		if tenantID, ok := c.Request.Context().Value(types.TenantIDContextKey).(uint64); ok {
			c.Set(types.TenantIDContextKey.String(), tenantID)
		}
		c.Next()
	})
	r.PUT("/datasource/:id/credentials", h.Put)
	return r
}

// TestDataSourceCredentialsPut_ResponseCarriesExpiryMetadata verifies the
// SP2-b §6.4 field-level metadata on the subresource response: configured
// plus expires_at / last_refreshed_at / needs_reauthorization, without any
// credential value on the wire.
func TestDataSourceCredentialsPut_ResponseCarriesExpiryMetadata(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", strings.Repeat("n", 32))
	expiresAt := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)
	blob, err := (&types.DataSourceConfig{
		Type: "feishu",
		Credentials: map[string]interface{}{
			"access_token":      "test-token-4",
			"expires_at":        expiresAt,
			"last_refreshed_at": "2026-09-19T10:00:00Z",
		},
	}).ToJSON()
	require.NoError(t, err)
	ds := &types.DataSource{
		ID: "ds-cred-meta", TenantID: 3, KnowledgeBaseID: "kb-cred-meta", Type: "feishu", Config: blob,
	}

	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) { return ds, nil },
		updateCredentials: func(_ context.Context, id string, credentials map[string]interface{}) (*types.DataSource, error) {
			return ds, nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: ds.KnowledgeBaseID, TenantID: ds.TenantID}, nil
		},
	}
	router := newCredentialsTestRouter(NewDataSourceCredentialsHandler(dsSvc, kbSvc))

	body, _ := json.Marshal(map[string]interface{}{"credentials": map[string]interface{}{"access_token": "test-token-4"}})
	req := withDSCtx(httptest.NewRequest(http.MethodPut, "/datasource/"+ds.ID+"/credentials", bytes.NewReader(body)), ds.TenantID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	s := w.Body.String()
	assert.Contains(t, s, `"configured":true`)
	assert.Contains(t, s, `"needs_reauthorization":true`, "48h horizon sits inside the 7-day window")
	assert.Contains(t, s, `"last_refreshed_at":"2026-09-19T10:00:00Z"`)
	assert.Contains(t, s, `"expires_at":"`+expiresAt+`"`)
	// Metadata only — the stored token value never appears.
	assert.NotContains(t, s, `"test-token-4"`)
}
