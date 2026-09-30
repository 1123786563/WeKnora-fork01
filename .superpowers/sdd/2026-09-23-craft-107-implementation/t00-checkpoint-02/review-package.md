# T00 fix round 1 incremental review package

From checkpoint-01 (14 files) to checkpoint-02 (15 files). HEAD remains 4bcad69baf033a1310b4dce1372c8153e66adc81.


## internal/handler/session/craft_features.go

--- checkpoint-01/internal/handler/session/craft_features.go
+++ checkpoint-02/internal/handler/session/craft_features.go
@@ -3,12 +3,15 @@
 import (
 	"fmt"
 	"sort"
+	"strings"
 	"sync"
+
+	"github.com/gin-gonic/gin"
 )
 
 // CraftFeatureRoutes is a construction-time registry. Each feature receives
-// only the existing authenticated /sessions route group; it cannot mount an
-// unguarded root route through this seam. Services still check Task access.
+// only a constrained view of the authenticated /sessions route group. Services
+// still check Task access.
 type CraftFeatureRoutes struct {
 	mu     sync.Mutex
 	frozen bool
@@ -65,10 +68,41 @@
 		names = append(names, name)
 	}
 	sort.Strings(names)
+	guarded := craftFeatureRouteGroup{group: group}
 	for _, name := range names {
-		r.routes[name](group)
+		r.routes[name](guarded)
 	}
 	return nil
+}
+
+// craftFeatureRouteGroup rejects unsafe paths before the router or API-key
+// policy registry sees them. Gin/path.Join would otherwise clean traversal and
+// could move a feature endpoint into a sibling /sessions namespace.
+type craftFeatureRouteGroup struct{ group CraftRouteGroup }
+
+func (g craftFeatureRouteGroup) GET(route string, handlers ...gin.HandlerFunc) gin.IRoutes {
+	validateCraftFeaturePath("GET", route)
+	return g.group.GET(route, handlers...)
+}
+
+func (g craftFeatureRouteGroup) POST(route string, handlers ...gin.HandlerFunc) gin.IRoutes {
+	validateCraftFeaturePath("POST", route)
+	return g.group.POST(route, handlers...)
+}
+
+func validateCraftFeaturePath(method, route string) {
+	prefix := "/:id/craft/"
+	if method == "POST" {
+		prefix = "/:session_id/craft/"
+	}
+	if !strings.HasPrefix(route, prefix) || len(route) == len(prefix) || strings.ContainsAny(route, "\\%?#") {
+		panic(fmt.Sprintf("invalid Craft feature path %q", route))
+	}
+	for _, segment := range strings.Split(strings.TrimPrefix(route, prefix), "/") {
+		if segment == "" || segment == "." || segment == ".." {
+			panic(fmt.Sprintf("invalid Craft feature path %q", route))
+		}
+	}
 }
 
 var registeredCraftFeatureRoutes = NewCraftFeatureRoutes()


## internal/handler/session/craft_features_test.go

--- checkpoint-01/internal/handler/session/craft_features_test.go
+++ checkpoint-02/internal/handler/session/craft_features_test.go
@@ -1,9 +1,11 @@
 package session
 
 import (
+	"fmt"
 	"net/http"
 	"net/http/httptest"
 	"reflect"
+	"strings"
 	"testing"
 
 	"github.com/Tencent/WeKnora/internal/modules/craft"
@@ -67,6 +69,48 @@
 	}
 }
 
+func TestCraftFeatureRoutesRejectNamespaceEscapesBeforeRegistration(t *testing.T) {
+	gin.SetMode(gin.TestMode)
+	cases := []struct{ method, path string }{
+		{"GET", "/:id/craft/../share"},
+		{"GET", "/:id/craft/%2e%2e/share"},
+		{"GET", "/:id/craft//share"},
+		{"GET", "/:id/artifact-versions"},
+		{"GET", "/api/v1/sessions/:id/craft/sources"},
+		{"GET", "https://example.invalid/sources"},
+		{"POST", "/:id/craft/share"},
+		{"POST", "/:session_id/craft/../../share"},
+		{"POST", "/:session_id/share"},
+	}
+	for _, tc := range cases {
+		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
+			r := gin.New()
+			features := NewCraftFeatureRoutes()
+			if err := features.Register("escape", func(group CraftRouteGroup) {
+				h := func(c *gin.Context) { c.Status(http.StatusOK) }
+				if tc.method == "GET" {
+					group.GET(tc.path, h)
+				} else {
+					group.POST(tc.path, h)
+				}
+			}); err != nil {
+				t.Fatal(err)
+			}
+			var panicValue any
+			func() {
+				defer func() { panicValue = recover() }()
+				_ = features.Mount(r.Group("/sessions"))
+			}()
+			if panicValue == nil || !strings.Contains(fmt.Sprint(panicValue), "invalid Craft feature path") {
+				t.Fatalf("path %q was not rejected by Craft registrar: %v", tc.path, panicValue)
+			}
+			if got := len(r.Routes()); got != 0 {
+				t.Fatalf("invalid path registered %d routes", got)
+			}
+		})
+	}
+}
+
 func TestCraftOptionalWebFactsPreserveLegacyDTO(t *testing.T) {
 	legacyInput := craftInputDTO(craft.Input{Ref: "r1"})
 	if _, ok := legacyInput["recognition"]; ok {


## internal/modules/craft/web_contracts.go

--- checkpoint-01/internal/modules/craft/web_contracts.go
+++ checkpoint-02/internal/modules/craft/web_contracts.go
@@ -181,10 +181,40 @@
 }
 
 type ExportFile struct {
-	Path       string `json:"path"`
-	SHA256     string `json:"sha256"`
-	Restricted bool   `json:"restricted"`
-}
+	Path       string            `json:"path"`
+	SHA256     string            `json:"sha256"`
+	Restricted bool              `json:"restricted"`
+	Origins    []ExportOriginRef `json:"origins"`
+}
+
+// ExportOriginRef identifies one immutable input or evidence item that
+// contributed to a derived file. Ref is an opaque identity, never a reusable
+// provider URL or a grant to open the original material.
+type ExportOriginKind string
+
+const (
+	ExportOriginKnowledge ExportOriginKind = "knowledge"
+	ExportOriginInput     ExportOriginKind = "input"
+	ExportOriginArtifact  ExportOriginKind = "artifact"
+)
+
+type ExportOriginRef struct {
+	Kind       ExportOriginKind `json:"kind"`
+	Ref        string           `json:"ref"`
+	SHA256     string           `json:"sha256"`
+	Restricted bool             `json:"restricted"`
+}
+
+func (o ExportOriginRef) Validate() error {
+	if o.Kind != ExportOriginKnowledge && o.Kind != ExportOriginInput && o.Kind != ExportOriginArtifact {
+		return fmt.Errorf("%w: invalid export origin kind", ErrInvalidInput)
+	}
+	if o.Ref == "" || o.SHA256 == "" {
+		return fmt.Errorf("%w: incomplete export origin", ErrInvalidInput)
+	}
+	return nil
+}
+
 type ExportManifest struct {
 	VersionID      string       `json:"version_id"`
 	ManifestDigest string       `json:"manifest_digest"`
@@ -196,9 +226,37 @@
 		return fmt.Errorf("%w: invalid export manifest", ErrInvalidInput)
 	}
 	for _, f := range m.Files {
-		if f.Path == "" || f.SHA256 == "" {
+		if f.Path == "" || f.SHA256 == "" || f.Origins == nil {
 			return fmt.Errorf("%w: invalid export file", ErrInvalidInput)
 		}
+		for _, origin := range f.Origins {
+			if err := origin.Validate(); err != nil {
+				return err
+			}
+		}
+	}
+	return nil
+}
+
+// ExportConsentView keeps the exact manifest, including file origins,
+// alongside an optional owner decision bound to that manifest identity.
+type ExportConsentView struct {
+	Manifest ExportManifest  `json:"manifest"`
+	Decision *ExportDecision `json:"decision"`
+}
+
+func (v ExportConsentView) Validate() error {
+	if err := v.Manifest.Validate(); err != nil {
+		return err
+	}
+	if v.Decision == nil {
+		return nil
+	}
+	if err := v.Decision.Validate(); err != nil {
+		return err
+	}
+	if v.Decision.VersionID != v.Manifest.VersionID || v.Decision.ManifestDigest != v.Manifest.ManifestDigest {
+		return fmt.Errorf("%w: export decision does not bind this manifest", ErrInvalidInput)
 	}
 	return nil
 }


## internal/modules/craft/web_contracts_test.go

--- checkpoint-01/internal/modules/craft/web_contracts_test.go
+++ checkpoint-02/internal/modules/craft/web_contracts_test.go
@@ -1,6 +1,9 @@
 package craft
 
-import "testing"
+import (
+	"encoding/json"
+	"testing"
+)
 
 func TestWebContractFacts(t *testing.T) {
 	input := InputRecognition{Accepted: true, Understood: false, Reason: "unrecognized_extension"}
@@ -61,7 +64,7 @@
 	if err := (ShareDecision{VersionID: "v1", EvidenceDigest: "sha256:a", OwnerID: "u1", Decision: DecisionStatus("maybe")}).Validate(); err == nil {
 		t.Fatal("unknown share decision accepted")
 	}
-	manifest := ExportManifest{VersionID: "v1", ManifestDigest: "sha256:m", Files: []ExportFile{{Path: "index.html", SHA256: "sha256:f", Restricted: false}}}
+	manifest := ExportManifest{VersionID: "v1", ManifestDigest: "sha256:m", Files: []ExportFile{{Path: "index.html", SHA256: "sha256:f", Restricted: false, Origins: []ExportOriginRef{}}}}
 	if err := manifest.Validate(); err != nil {
 		t.Fatal(err)
 	}
@@ -72,3 +75,30 @@
 		t.Fatal("unbound export decision accepted")
 	}
 }
+
+func TestWebExportConsentPreservesOriginsAndBindsDigest(t *testing.T) {
+	manifest := ExportManifest{VersionID: "v1", ManifestDigest: "sha256:manifest", Files: []ExportFile{{Path: "data.csv", SHA256: "sha256:derived", Restricted: true, Origins: []ExportOriginRef{{Kind: ExportOriginKnowledge, Ref: "source-1", SHA256: "sha256:source", Restricted: true}}}}}
+	view := ExportConsentView{Manifest: manifest, Decision: &ExportDecision{VersionID: "v1", ManifestDigest: "sha256:manifest", OwnerID: "owner-1", Decision: DecisionApproved}}
+	if err := view.Validate(); err != nil {
+		t.Fatal(err)
+	}
+	raw, err := json.Marshal(view)
+	if err != nil {
+		t.Fatal(err)
+	}
+	var decoded ExportConsentView
+	if err := json.Unmarshal(raw, &decoded); err != nil {
+		t.Fatal(err)
+	}
+	if got := decoded.Manifest.Files[0].Origins[0].Ref; got != "source-1" {
+		t.Fatalf("origin lost: %q", got)
+	}
+	decoded.Decision.ManifestDigest = "sha256:other"
+	if err := decoded.Validate(); err == nil {
+		t.Fatal("decision accepted for different manifest")
+	}
+	manifest.Files[0].Origins[0].Kind = ExportOriginKind("web")
+	if err := manifest.Validate(); err == nil {
+		t.Fatal("unknown origin kind accepted")
+	}
+}


## internal/router/routes_craft_features_test.go

--- /dev/null
+++ checkpoint-02/internal/router/routes_craft_features_test.go
@@ -0,0 +1,97 @@
+package router
+
+import (
+	"context"
+	"fmt"
+	"net/http"
+	"net/http/httptest"
+	"strings"
+	"testing"
+
+	"github.com/Tencent/WeKnora/internal/config"
+	"github.com/Tencent/WeKnora/internal/handler/session"
+	"github.com/Tencent/WeKnora/internal/types"
+	"github.com/gin-gonic/gin"
+)
+
+// Use the same Viewer+ group, chat API-key wrapper and authorizer that
+// RegisterSessionRoutes uses in production, rather than a synthetic header.
+func TestCraftFeatureProductionRouterPolicyAndEscape(t *testing.T) {
+	gin.SetMode(gin.TestMode)
+	enabled := true
+	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}}
+	r := gin.New()
+	r.Use(func(c *gin.Context) {
+		ctx := c.Request.Context()
+		if c.GetHeader("X-Test-Revoked") == "yes" {
+			ctx = context.WithValue(ctx, types.CallerContextKey, types.Caller{Role: types.TenantRole("revoked")})
+		}
+		if c.GetHeader("X-Test-Key") != "" {
+			scope := types.TenantAPIKeyScope{KeyID: 1}
+			if c.GetHeader("X-Test-Key") == "chat" {
+				scope.Capabilities = types.StringArray{string(types.APIKeyCapabilityChat)}
+			}
+			ctx = types.WithTenantAPIKeyScope(ctx, scope)
+		}
+		c.Request = c.Request.WithContext(ctx)
+		c.Next()
+	})
+	r.Use(g.ensureAPIKeyAuthorizer().Middleware())
+	v1 := r.Group("/api/v1")
+	sessions := g.apiKeyGroup(v1.Group("/sessions", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
+	features := session.NewCraftFeatureRoutes()
+	if err := features.Register("sources", func(group session.CraftRouteGroup) {
+		group.GET("/:id/craft/sources", func(c *gin.Context) { c.Status(http.StatusOK) })
+	}); err != nil {
+		t.Fatal(err)
+	}
+	if err := features.Mount(sessions); err != nil {
+		t.Fatal(err)
+	}
+	path := "/api/v1/sessions/task-1/craft/sources"
+	for _, tc := range []struct {
+		name, header, value string
+		want                int
+	}{
+		{"viewer", "", "", http.StatusOK},
+		{"revoked", "X-Test-Revoked", "yes", http.StatusForbidden},
+		{"scoped key without chat", "X-Test-Key", "no-chat", http.StatusForbidden},
+		{"scoped key with chat", "X-Test-Key", "chat", http.StatusOK},
+	} {
+		t.Run(tc.name, func(t *testing.T) {
+			req := httptest.NewRequest(http.MethodGet, path, nil)
+			if tc.header != "" {
+				req.Header.Set(tc.header, tc.value)
+			}
+			w := httptest.NewRecorder()
+			r.ServeHTTP(w, req)
+			if w.Code != tc.want {
+				t.Fatalf("status = %d, want %d", w.Code, tc.want)
+			}
+		})
+	}
+	policy := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/sessions/:id/craft/sources")
+	if !policy.RequireFullAccess || !policyHasCapability(policy, types.APIKeyCapabilityChat) {
+		t.Fatalf("Craft feature policy = %#v", policy)
+	}
+
+	escape := session.NewCraftFeatureRoutes()
+	if err := escape.Register("escape", func(group session.CraftRouteGroup) {
+		group.GET("/:id/craft/../../share", func(c *gin.Context) { c.Status(http.StatusOK) })
+	}); err != nil {
+		t.Fatal(err)
+	}
+	var rejected any
+	func() { defer func() { rejected = recover() }(); _ = escape.Mount(sessions) }()
+	if rejected == nil || !strings.Contains(fmt.Sprint(rejected), "invalid Craft feature path") {
+		t.Fatalf("escape was not rejected before router registration: %v", rejected)
+	}
+	if _, ok := g.apiKeyAuthorizer.Lookup(http.MethodGet, "/api/v1/sessions/share"); ok {
+		t.Fatal("escape registered an API-key policy outside Craft")
+	}
+	for _, route := range r.Routes() {
+		if route.Path == "/api/v1/sessions/share" {
+			t.Fatal("escape registered a sibling route")
+		}
+	}
+}


## packages/contracts/src/craft/web-artifact.test.ts

--- checkpoint-01/packages/contracts/src/craft/web-artifact.test.ts
+++ checkpoint-02/packages/contracts/src/craft/web-artifact.test.ts
@@ -13,6 +13,7 @@
   parseCraftShareDecision,
   parseCraftExportManifest,
   parseCraftExportDecision,
+  parseCraftExportConsentView,
 } from './index.ts';
 
 const version = { id: 'v1', workspace_id: 'w1', run_id: 'r1', kind: 'web', files: [], checks: [] };
@@ -22,6 +23,8 @@
   assert.throws(() => parseCraftInputRecognition({ accepted: false, understood: true, reason: '' }), /understood/);
   assert.equal(parseCraftInputView({ ref: 'r', name: 'x', sha256: 'h', bytes: 1, citation_id: '' }).recognition, null);
   assert.equal(parseCraftInputView({ ref: 'r', name: 'x', sha256: 'h', bytes: 1, citation_id: '', recognition: { accepted: true, understood: false, reason: 'opaque' } }).recognition?.understood, false);
+  assert.throws(() => parseCraftInputRecognition({ accepted: true, understood: true, reason: 17 }), /reason/);
+  assert.throws(() => parseCraftInputView({ ref: 'r', name: 'x', sha256: 'h', bytes: 1, citation_id: '', recognition: { accepted: true, understood: true, reason: { unexpected: true } } }), /reason/);
 });
 
 test('version keeps four independent gates and rejects unknown statuses', () => {
@@ -45,9 +48,22 @@
   assert.equal(parseCraftRestrictedContribution({ version_id: 'v1', evidence_digest: 'sha256:a', restricted: true }).restricted, true);
   assert.equal(parseCraftShareDecision({ version_id: 'v1', evidence_digest: 'sha256:a', owner_id: 'u1', decision: 'approved' }).decision, 'approved');
   assert.throws(() => parseCraftShareDecision({ version_id: 'v1', evidence_digest: 'sha256:a', owner_id: 'u1', decision: 'maybe' }), /decision/);
-  assert.equal(parseCraftExportManifest({ version_id: 'v1', manifest_digest: 'sha256:m', files: [{ path: 'index.html', sha256: 'sha256:f', restricted: false }] }).files.length, 1);
+  assert.equal(parseCraftExportManifest({ version_id: 'v1', manifest_digest: 'sha256:m', files: [{ path: 'index.html', sha256: 'sha256:f', restricted: false, origins: [] }] }).files.length, 1);
   assert.equal(parseCraftExportDecision({ version_id: 'v1', manifest_digest: 'sha256:m', owner_id: 'u1', decision: 'rejected' }).decision, 'rejected');
   assert.throws(() => parseCraftExportDecision({ version_id: 'v1', manifest_digest: '', owner_id: 'u1', decision: 'approved' }), /manifest_digest/);
+});
+
+test('export consent view preserves immutable derived-file origins and binds decision digest', () => {
+  const raw = {
+    manifest: { version_id: 'v1', manifest_digest: 'sha256:manifest', files: [{ path: 'data.csv', sha256: 'sha256:derived', restricted: true, origins: [{ kind: 'knowledge', ref: 'source-1', sha256: 'sha256:source', restricted: true }] }] },
+    decision: { version_id: 'v1', manifest_digest: 'sha256:manifest', owner_id: 'owner-1', decision: 'approved' },
+  };
+  const view = parseCraftExportConsentView(raw);
+  assert.deepEqual(JSON.parse(JSON.stringify(view)), raw);
+  assert.equal(view.manifest.files[0]?.origins[0]?.ref, 'source-1');
+  assert.throws(() => parseCraftExportConsentView({ ...raw, decision: { ...raw.decision, manifest_digest: 'sha256:other' } }), /manifest_digest/);
+  assert.throws(() => parseCraftExportManifest({ ...raw.manifest, files: [{ ...raw.manifest.files[0], origins: [{ kind: 'web', ref: 'source-1', sha256: 'sha256:source', restricted: true }] }] }), /kind/);
+  assert.throws(() => parseCraftExportManifest({ ...raw.manifest, files: [{ ...raw.manifest.files[0], origins: [{ kind: 'knowledge', ref: '', sha256: 'sha256:source', restricted: true }] }] }), /ref/);
 });
 
 test('run budget pause is a typed optional fact on the existing waiting state', () => {


## packages/contracts/src/craft/web-artifact.ts

--- checkpoint-01/packages/contracts/src/craft/web-artifact.ts
+++ checkpoint-02/packages/contracts/src/craft/web-artifact.ts
@@ -16,9 +16,14 @@
 export interface CraftBudgetPause { run_id: string; reason: string; limit: number; used: number }
 export interface CraftRestrictedContribution { version_id: string; evidence_digest: string; restricted: boolean }
 export interface CraftShareDecision { version_id: string; evidence_digest: string; owner_id: string; decision: CraftDecisionStatus }
-export interface CraftExportFile { path: string; sha256: string; restricted: boolean }
+export const CRAFT_EXPORT_ORIGIN_KINDS = ['knowledge', 'input', 'artifact'] as const;
+export type CraftExportOriginKind = (typeof CRAFT_EXPORT_ORIGIN_KINDS)[number];
+/** Opaque, immutable provenance identity; opening the original reauthorizes. */
+export interface CraftExportOriginRef { kind: CraftExportOriginKind; ref: string; sha256: string; restricted: boolean }
+export interface CraftExportFile { path: string; sha256: string; restricted: boolean; origins: CraftExportOriginRef[] }
 export interface CraftExportManifest { version_id: string; manifest_digest: string; files: CraftExportFile[] }
 export interface CraftExportDecision { version_id: string; manifest_digest: string; owner_id: string; decision: CraftDecisionStatus }
+export interface CraftExportConsentView { manifest: CraftExportManifest; decision: CraftExportDecision | null }
 
 function row(value: unknown, label: string): Record<string, unknown> {
   if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid ' + label);
@@ -46,7 +51,8 @@
   const accepted = flag(v.accepted, 'accepted');
   const understood = flag(v.understood, 'understood');
   if (understood && !accepted) throw new Error('invalid understood');
-  const reason = typeof v.reason === 'string' ? v.reason : '';
+  if (typeof v.reason !== 'string') throw new Error('invalid reason');
+  const reason = v.reason;
   if (accepted && !understood && reason === '') throw new Error('invalid reason');
   return { accepted, understood, reason };
 }
@@ -72,8 +78,22 @@
 export function parseCraftExportManifest(value: unknown): CraftExportManifest {
   const v = row(value, 'export manifest');
   if (!Array.isArray(v.files)) throw new Error('invalid files');
-  return { version_id: id(v.version_id, 'version_id'), manifest_digest: id(v.manifest_digest, 'manifest_digest'), files: v.files.map((file) => { const f = row(file, 'export file'); return { path: id(f.path, 'path'), sha256: id(f.sha256, 'sha256'), restricted: flag(f.restricted, 'restricted') }; }) };
+  return { version_id: id(v.version_id, 'version_id'), manifest_digest: id(v.manifest_digest, 'manifest_digest'), files: v.files.map((file) => {
+    const f = row(file, 'export file');
+    if (!Array.isArray(f.origins)) throw new Error('invalid origins');
+    return { path: id(f.path, 'path'), sha256: id(f.sha256, 'sha256'), restricted: flag(f.restricted, 'restricted'), origins: f.origins.map((origin) => {
+      const o = row(origin, 'export origin');
+      return { kind: choice(o.kind, CRAFT_EXPORT_ORIGIN_KINDS, 'kind'), ref: id(o.ref, 'ref'), sha256: id(o.sha256, 'sha256'), restricted: flag(o.restricted, 'restricted') };
+    }) };
+  }) };
 }
 export function parseCraftExportDecision(value: unknown): CraftExportDecision {
   const v = row(value, 'export decision'); return { version_id: id(v.version_id, 'version_id'), manifest_digest: id(v.manifest_digest, 'manifest_digest'), owner_id: id(v.owner_id, 'owner_id'), decision: choice(v.decision, CRAFT_DECISIONS, 'decision') };
 }
+export function parseCraftExportConsentView(value: unknown): CraftExportConsentView {
+  const v = row(value, 'export consent view');
+  const manifest = parseCraftExportManifest(v.manifest);
+  const decision = v.decision === undefined || v.decision === null ? null : parseCraftExportDecision(v.decision);
+  if (decision !== null && (decision.version_id !== manifest.version_id || decision.manifest_digest !== manifest.manifest_digest)) throw new Error('invalid manifest_digest');
+  return { manifest, decision };
+}
