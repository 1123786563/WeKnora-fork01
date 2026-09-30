package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestRecoveryRepanicsErrAbortHandler is the OCR high-finding regression:
// the craft export bundle's mid-stream integrity contract panics with
// http.ErrAbortHandler AFTER the 200 head and partial bytes left the
// process. Recovery must re-panic the sentinel (net/http then tears the
// connection down) instead of appending a 500 JSON onto the truncated
// stream and finishing the chunked response cleanly.
func TestRecoveryRepanicsErrAbortHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Recovery())
	router.GET("/export", func(c *gin.Context) {
		c.Status(http.StatusOK)
		_, _ = c.Writer.WriteString("partial-zip-bytes")
		panic(http.ErrAbortHandler)
	})

	repanicked := false
	func() {
		defer func() {
			if err := recover(); err != nil {
				repanicked = true
				require.ErrorIs(t, err.(error), http.ErrAbortHandler)
			}
		}()
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/export", nil))
	}()
	require.True(t, repanicked, "Recovery must re-panic http.ErrAbortHandler so the connection is aborted")
}

// TestRecoveryStillConvertsOtherPanics guards the other direction: ordinary
// panics keep producing the 500 JSON envelope.
func TestRecoveryStillConvertsOtherPanics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Recovery())
	router.GET("/boom", func(c *gin.Context) {
		panic("ordinary failure")
	})
	recorder := httptest.NewRecorder()
	require.NotPanics(t, func() {
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/boom", nil))
	})
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Internal Server Error")
}
