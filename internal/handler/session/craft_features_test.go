package session

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/gin-gonic/gin"
)

func TestCraftFeatureRoutesInheritGuardsAndFreezeInNameOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	features := NewCraftFeatureRoutes()
	var mounted []string
	for _, name := range []string{"sources", "access"} {
		name := name
		if err := features.Register(name, func(group craftRouteGroup) {
			mounted = append(mounted, name)
			group.GET("/:id/craft/"+name, func(c *gin.Context) { c.Status(http.StatusOK) })
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := features.Register("access", func(craftRouteGroup) {}); err == nil {
		t.Fatal("duplicate feature name accepted")
	}
	if err := features.Register("nil", nil); err == nil {
		t.Fatal("nil route feature accepted")
	}
	if err := features.Register("1invalid", func(CraftRouteGroup) {}); err == nil {
		t.Fatal("feature name beginning with a digit accepted")
	}
	r := gin.New()
	sessions := r.Group("/sessions", func(c *gin.Context) {
		if c.GetHeader("X-Test-Auth") != "yes" {
			c.AbortWithStatus(http.StatusUnauthorized)
		}
	})
	if err := features.Mount(sessions); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(mounted, []string{"access", "sources"}) {
		t.Fatalf("mount order = %v", mounted)
	}
	if err := features.Register("late", func(craftRouteGroup) {}); err == nil {
		t.Fatal("registration accepted after mount")
	}
	second := gin.New().Group("/sessions")
	if err := features.Mount(second); err != nil {
		t.Fatalf("frozen route table should mount in another guarded router: %v", err)
	}
	for _, name := range []string{"access", "sources"} {
		unauth := httptest.NewRecorder()
		r.ServeHTTP(unauth, httptest.NewRequest(http.MethodGet, "/sessions/s1/craft/"+name, nil))
		if unauth.Code != http.StatusUnauthorized {
			t.Fatalf("%s bypassed parent guard: %d", name, unauth.Code)
		}
		auth := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/sessions/s1/craft/"+name, nil)
		req.Header.Set("X-Test-Auth", "yes")
		r.ServeHTTP(auth, req)
		if auth.Code != http.StatusOK {
			t.Fatalf("%s route missing: %d", name, auth.Code)
		}
	}
}

func TestCraftFeatureRoutesRejectNamespaceEscapesBeforeRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct{ method, path string }{
		{"GET", "/:id/craft/../share"},
		{"GET", "/:id/craft/%2e%2e/share"},
		{"GET", "/:id/craft//share"},
		{"GET", "/:id/artifact-versions"},
		{"GET", "/api/v1/sessions/:id/craft/sources"},
		{"GET", "https://example.invalid/sources"},
		{"POST", "/:id/craft/share"},
		{"POST", "/:session_id/craft/../../share"},
		{"POST", "/:session_id/share"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			r := gin.New()
			features := NewCraftFeatureRoutes()
			if err := features.Register("escape", func(group CraftRouteGroup) {
				h := func(c *gin.Context) { c.Status(http.StatusOK) }
				if tc.method == "GET" {
					group.GET(tc.path, h)
				} else {
					group.POST(tc.path, h)
				}
			}); err != nil {
				t.Fatal(err)
			}
			var panicValue any
			func() {
				defer func() { panicValue = recover() }()
				_ = features.Mount(r.Group("/sessions"))
			}()
			if panicValue == nil || !strings.Contains(fmt.Sprint(panicValue), "invalid Craft feature path") {
				t.Fatalf("path %q was not rejected by Craft registrar: %v", tc.path, panicValue)
			}
			if got := len(r.Routes()); got != 0 {
				t.Fatalf("invalid path registered %d routes", got)
			}
		})
	}
}

func TestCraftOptionalWebFactsPreserveLegacyDTO(t *testing.T) {
	legacyInput := craftInputDTO(craft.Input{Ref: "r1"})
	if _, ok := legacyInput["recognition"]; ok {
		t.Fatal("legacy input gained an unsupported field")
	}
	input := craftInputDTO(craft.Input{Ref: "r1", Recognition: &craft.InputRecognition{Accepted: true, Understood: false, Reason: "opaque"}})
	if input["recognition"] == nil {
		t.Fatal("recognition fact omitted")
	}
	legacyVersion := craftVersionDTO(craft.Version{ID: "v1"})
	if _, ok := legacyVersion["web_evidence"]; ok {
		t.Fatal("legacy version gained an unsupported field")
	}
	version := craftVersionDTO(craft.Version{ID: "v1", WebEvidence: &craft.WebCheckEvidence{Build: craft.WebCheckPassed, Entry: craft.WebCheckPassed, PreviewReachable: craft.WebCheckPassed, PageLoaded: craft.WebCheckNotRun}})
	if version["web_evidence"] == nil {
		t.Fatal("independent web evidence omitted")
	}
}
