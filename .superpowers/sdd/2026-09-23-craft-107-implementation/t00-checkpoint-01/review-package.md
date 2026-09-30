# T00 / #119 review package — uncommitted checkpoint 01

BASE/HEAD: 4bcad69baf033a1310b4dce1372c8153e66adc81

Tracked modified: internal/handler/session/craft.go, internal/modules/craft/contracts.go, packages/contracts/src/craft/index.ts, packages/views/src/craft/workbench.stop.test.tsx, packages/views/src/craft/workbench.tsx

New files: docs/plans/2026-09-23-craft-107-ownership.md, internal/container/craft_features.go, internal/handler/session/craft_features.go, internal/handler/session/craft_features_test.go, internal/modules/craft/web_contracts.go, internal/modules/craft/web_contracts_test.go, packages/contracts/src/craft/web-artifact.test.ts, packages/contracts/src/craft/web-artifact.ts, packages/views/src/craft/workbench-features.tsx

## Tracked diff

diff --git a/internal/handler/session/craft.go b/internal/handler/session/craft.go
index 7feaab089..a7254266e 100644
--- a/internal/handler/session/craft.go
+++ b/internal/handler/session/craft.go
@@ -104,10 +104,11 @@ func NewCraftSnapshotHandler(svc CraftSnapshotAPI) *CraftSnapshotHandler {
 // craftRouteGroup is the route-mounting subset satisfied by both a raw gin
 // group and the router's API-key-policy wrapper, so the craft routes can be
 // mounted through whichever wrapper declares their auth policy.
-type craftRouteGroup interface {
+type CraftRouteGroup interface {
 	GET(string, ...gin.HandlerFunc) gin.IRoutes
 	POST(string, ...gin.HandlerFunc) gin.IRoutes
 }
+type craftRouteGroup = CraftRouteGroup
 
 // RegisterCraftSessionRoutes mounts W03's craft API table. craftSessions is
 // the /craft/sessions group (create + list); sessions is the existing
@@ -151,6 +152,11 @@ func RegisterCraftSessionRoutes(craftSessions, sessions craftRouteGroup, craftHa
 		sessions.POST("/:session_id/craft/versions/:version_id/preview",
 			craftPreviewFailureMetrics(previewHandler.IssueCraftPreview))
 	}
+	if craftHandler != nil && registeredCraftFeatureRoutes.hasFeatures() {
+		if err := registeredCraftFeatureRoutes.Mount(sessions); err != nil {
+			panic(err)
+		}
+	}
 }
 
 // craftPreviewFailureMetrics counts a failed craft preview issuance at the
@@ -305,17 +311,25 @@ func craftVersionDTO(v craft.Version) gin.H {
 	for _, check := range v.Checks {
 		checks = append(checks, craftCheckDTO(check))
 	}
-	return gin.H{
+	dto := gin.H{
 		"id": v.ID, "workspace_id": v.WorkspaceID, "run_id": v.RunID,
 		"kind": v.Kind, "files": files, "checks": checks,
 	}
+	if v.WebEvidence != nil {
+		dto["web_evidence"] = v.WebEvidence
+	}
+	return dto
 }
 
 func craftInputDTO(in craft.Input) gin.H {
-	return gin.H{
+	dto := gin.H{
 		"ref": in.Ref, "name": in.Name, "sha256": in.SHA256,
 		"bytes": in.Bytes, "citation_id": in.CitationID,
 	}
+	if in.Recognition != nil {
+		dto["recognition"] = in.Recognition
+	}
+	return dto
 }
 
 // -----------------------------------------------------------------------------
diff --git a/internal/modules/craft/contracts.go b/internal/modules/craft/contracts.go
index fcb58c725..9d7ecb0ca 100644
--- a/internal/modules/craft/contracts.go
+++ b/internal/modules/craft/contracts.go
@@ -58,6 +58,7 @@ type Workspace struct {
 type Input struct {
 	Ref, Name, SHA256, CitationID string
 	Bytes                         int64
+	Recognition                   *InputRecognition
 }
 
 // Task is the durable delegation request executed by the OpenCode runtime
@@ -89,6 +90,7 @@ type Version struct {
 	ID, WorkspaceID, RunID, Kind string
 	Files                        []File
 	Checks                       []Check
+	WebEvidence                  *WebCheckEvidence
 }
 
 // Observation is the OpenCode session state projected by Observe.
diff --git a/packages/contracts/src/craft/index.ts b/packages/contracts/src/craft/index.ts
index 4626c2f51..69a24fd4f 100644
--- a/packages/contracts/src/craft/index.ts
+++ b/packages/contracts/src/craft/index.ts
@@ -8,6 +8,9 @@
 // file bytes travel through the platform transport, never through domain
 // state.
 
+export * from './web-artifact.ts';
+import { parseCraftBudgetPause, parseCraftInputRecognition, parseCraftWebCheckEvidence, type CraftBudgetPause, type CraftInputRecognition, type CraftWebCheckEvidence } from './web-artifact.ts';
+
 /** Craft artifact kinds (internal/craft/request.go closed set). */
 export const CRAFT_SESSION_KINDS = ['web', 'document', 'spreadsheet', 'slides'] as const;
 export type CraftSessionKind = (typeof CRAFT_SESSION_KINDS)[number];
@@ -68,6 +71,7 @@ export interface CraftRunView {
   /** Server-assigned event watermark; 0 on the POST /runs admission view. */
   seq: number;
   pending_id: string | null;
+  budget_pause: CraftBudgetPause | null;
 }
 
 export interface CraftFileVersionView {
@@ -91,6 +95,7 @@ export interface CraftVersionView {
   kind: string;
   files: CraftFileVersionView[];
   checks: CraftVersionCheckView[];
+  web_evidence: CraftWebCheckEvidence | null;
 }
 
 export interface CraftVersionsPageView {
@@ -129,6 +134,7 @@ export interface CraftInputView {
   sha256: string;
   bytes: number;
   citation_id: string;
+  recognition: CraftInputRecognition | null;
 }
 
 export interface CraftPreviewTicketView {
@@ -244,6 +250,7 @@ export function parseCraftRunView(value: unknown): CraftRunView {
     epoch: nonNegativeInt(v.epoch, 'epoch', 'craft run view'),
     seq: nonNegativeInt(v.seq, 'seq', 'craft run view'),
     pending_id: optionalString(v.pending_id, 'pending_id', 'craft run view'),
+    budget_pause: v.budget_pause === undefined || v.budget_pause === null ? null : parseCraftBudgetPause(v.budget_pause),
   };
 }
 
@@ -278,6 +285,7 @@ export function parseCraftVersionView(value: unknown): CraftVersionView {
     kind: nonEmptyString(v.kind, 'kind', 'craft version'),
     files: v.files.map(parseFileVersion),
     checks: v.checks.map(parseVersionCheck),
+    web_evidence: v.web_evidence === undefined || v.web_evidence === null ? null : parseCraftWebCheckEvidence(v.web_evidence),
   };
 }
 
@@ -330,6 +338,7 @@ export function parseCraftInputView(value: unknown): CraftInputView {
     sha256: typeof v.sha256 === 'string' ? v.sha256 : '',
     bytes: nonNegativeInt(v.bytes, 'bytes', 'craft input'),
     citation_id: typeof v.citation_id === 'string' ? v.citation_id : '',
+    recognition: v.recognition === undefined || v.recognition === null ? null : parseCraftInputRecognition(v.recognition),
   };
 }
 
diff --git a/packages/views/src/craft/workbench.stop.test.tsx b/packages/views/src/craft/workbench.stop.test.tsx
index bcdf65838..3e7f55799 100644
--- a/packages/views/src/craft/workbench.stop.test.tsx
+++ b/packages/views/src/craft/workbench.stop.test.tsx
@@ -65,6 +65,7 @@ if (hooks.registerHooks) {
 // react-dom resolves through the web app's install (views stays renderer-free).
 const { createRoot } = await import('../../../../apps/web/node_modules/react-dom/client.js');
 const { CraftWorkbench } = await import('./workbench.tsx');
+const { createCraftWorkbenchFeatures } = await import('./workbench-features.tsx');
 const { createCraftMessageLog } = await import('./presentation.ts');
 
 // The controller state shape the W04 reducer publishes (domain CraftState).
@@ -165,6 +166,24 @@ test('stop button appears while a run is active and calls onStopRun once', async
   });
 });
 
+test('two named workbench features render in deterministic slots and registry freezes', async () => {
+  document.body.replaceChildren();
+  const features = createCraftWorkbenchFeatures([
+    { name: 'sources', slot: 'aside', render: () => <span data-testid="sources-feature">Sources</span> },
+    { name: 'access', slot: 'header', render: () => <span data-testid="access-feature">Access</span> },
+  ]);
+  assert.deepEqual(features.map((feature) => feature.name), ['access', 'sources']);
+  assert.throws(() => createCraftWorkbenchFeatures([
+    { name: 'access', slot: 'header', render: () => null },
+    { name: 'access', slot: 'aside', render: () => null },
+  ]), /duplicate/);
+  assert.throws(() => createCraftWorkbenchFeatures([{ name: 'missing', slot: 'header', render: null as never }]), /render/);
+  const root = await mount(<CraftWorkbench {...baseProps(fakeController(null))} features={features} />);
+  assert.ok(document.querySelector('[data-testid="access-feature"]'));
+  assert.ok(document.querySelector('[data-testid="sources-feature"]'));
+  await act(async () => { root.unmount(); });
+});
+
 test('stop button hidden when no run is active', async () => {
   document.body.replaceChildren();
   // A finished run (terminal main status) AND a workbench with no run at all
diff --git a/packages/views/src/craft/workbench.tsx b/packages/views/src/craft/workbench.tsx
index 0aef530e4..5b5cbc1bf 100644
--- a/packages/views/src/craft/workbench.tsx
+++ b/packages/views/src/craft/workbench.tsx
@@ -61,6 +61,9 @@ import './craft.css';
 // re-exported here because the assembly imports it from this module.
 export { createCraftMessageLog } from './presentation.ts';
 export type { CraftMessageSnapshot } from './presentation.ts';
+export { createCraftWorkbenchFeatures } from './workbench-features.tsx';
+export type { CraftWorkbenchFeature, CraftWorkbenchFeatureContext, CraftWorkbenchSlot } from './workbench-features.tsx';
+import type { CraftWorkbenchFeature, CraftWorkbenchSlot } from './workbench-features.tsx';
 
 // ---------------------------------------------------------------------------
 // Workbench component
@@ -81,6 +84,8 @@ export interface CraftInteractionActionInput {
 }
 
 export interface CraftWorkbenchProps {
+	/** Immutable, keyed feature list assembled before this view mounts. */
+	features?: readonly CraftWorkbenchFeature[];
   locale: CraftLocale;
   sessionId: string;
   title: string;
@@ -738,6 +743,10 @@ export function CraftWorkbench(props: CraftWorkbenchProps) {
     );
   };
 
+  const renderFeatures = (slot: CraftWorkbenchSlot) => (props.features ?? [])
+    .filter((feature) => feature.slot === slot)
+    .map((feature) => <div key={feature.name} data-craft-feature={feature.name}>{feature.render({ sessionId: props.sessionId, canWrite: props.canWrite, selectedVersionId })}</div>);
+
   return (
     <main className="wk-craft wk-craft-page" aria-label={props.title || strings.craftHomeTitle}>
       <div className="wk-craft-head">
@@ -750,6 +759,7 @@ export function CraftWorkbench(props: CraftWorkbenchProps) {
         </div>
       </div>
       {props.syncError !== null ? <p className="wk-craft-error" role="alert">{strings.craftStreamReconnecting} ({props.syncError})</p> : null}
+      {renderFeatures('header')}
       {restoreEntryVisible && restoreBlockedReason !== null && selectedVersionId !== null && !runActive && mainStatus !== 'waiting_user' ? (
         <p className="wk-craft-hint" data-testid="craft-restore-reason">{restoreBlockedReason}</p>
       ) : null}
@@ -829,6 +839,7 @@ export function CraftWorkbench(props: CraftWorkbenchProps) {
             aria-label={strings.craftTabConversation}
             data-narrow-hidden={narrow && narrowTab !== 'conversation'}
           >
+            {renderFeatures('conversation')}
             {conversation}
           </section>
           {!narrow ? (
@@ -859,7 +870,7 @@ export function CraftWorkbench(props: CraftWorkbenchProps) {
                 {tabButton('details', strings.craftTabDetails)}
               </div>
             ) : null}
-            <div className="wk-craft-panel">{sidePanel}</div>
+            <div className="wk-craft-panel">{sidePanel}{renderFeatures('aside')}</div>
           </section>
         </div>
       </div>


## New file docs/plans/2026-09-23-craft-107-ownership.md

--- /dev/null
+++ b/docs/plans/2026-09-23-craft-107-ownership.md
@@ -0,0 +1,41 @@
+# Craft #107 ownership and integration locks
+
+Source: approved Craft web-artifact Spec, #119–#139 DAG, and implementation plan. Task ID remains the existing Session ID; one Run represents one execution. This matrix assigns writes; dependency order remains the DAG.
+
+## Central seams
+
+| Files | Owner | Rule |
+| --- | --- | --- |
+| `internal/modules/craft/contracts.go`, `web_contracts.go`, public HTTP DTOs in `internal/handler/session/craft.go`, `packages/contracts/src/craft/index.ts`, `web-artifact.ts` | T00; T20 for integration corrections | Feature lanes consume the frozen names and request a contract amendment through the controller; they do not edit these files. |
+| `internal/handler/session/craft_features.go`, `internal/container/craft_features.go`, `packages/views/src/craft/workbench-features.tsx`, `workbench.tsx` | T00; T20 for final composition | Lane handlers register by unique name before router build. Web panels use unique names and one typed slot; T20 passes the immutable feature list. |
+| `internal/router/routes_chat.go`, `internal/container/container.go`, `apps/web/src/features/craft/routes.tsx` | T20 | Lane packages expose constructors/adapters; T20 wires them after reviewed checkpoints are integrated. |
+| `internal/database/migration.go`, migration numbers and migration files | T20 | Lanes submit schema requirements without reserving a number. T20 allocates ordered numbers at integration. |
+
+## Feature lanes
+
+| Task | Exclusive extension files or area |
+| --- | --- |
+| T01 | `internal/modules/craft/input.go`, `internal/application/service/craft_inputs.go`, `packages/views/src/craft/files.tsx` |
+| T02 | `internal/modules/craft/archive.go`, `internal/application/service/craft_archive.go`, archive fixtures |
+| T03 | `internal/modules/craft/input_code.go`, `internal/application/service/craft_delegate.go`, sandbox material policy adapter |
+| T04 | Pinned web template/runtime image and lock, OpenCode web skill, `internal/container/craft_runtime.go` |
+| T05 | `internal/modules/craft/knowledge.go`, `internal/application/service/craft_knowledge.go`, `packages/views/src/craft/sources.tsx` |
+| T06 | `internal/modules/craft/citation.go`, `internal/application/service/craft_citations.go`, generated web citation view |
+| T07 | `internal/modules/craft/version.go`, `internal/application/repository/craft_version.go`; `craft_artifacts.go` requires controller lock |
+| T08 | `internal/modules/craft/access.go`, `internal/application/service/craft_access.go`, `internal/handler/session/craft_access.go`, `packages/views/src/craft/access.tsx`; supplies `craft.TaskAccessChecker` |
+| T09 | `internal/application/service/craft_collaborator_run.go`, `packages/views/src/craft/workbench-edit.tsx` |
+| T10 | `internal/application/service/craft_source_open.go`, `internal/handler/session/artifact_reference.go` |
+| T11 | `internal/modules/craft/share.go`, `internal/application/service/craft_share.go`, `internal/handler/session/craft_share.go`, `packages/views/src/craft/share.tsx` |
+| T12 | `internal/modules/craft/export_manifest.go`, `internal/application/service/craft_export.go`, `internal/handler/session/artifact_download.go` |
+| T13 | `internal/modules/craft/export_consent.go`, `internal/application/service/craft_export_consent.go`, `internal/handler/session/craft_export_consent.go`, `packages/views/src/craft/export.tsx` |
+| T14 | `internal/modules/craft/preview.go`, `internal/application/service/craft_preview.go`, `internal/handler/session/craft_preview.go`, preview network policy |
+| T15 | `internal/modules/craft/release.go`, `internal/application/repository/craft_preview_check.go`; `craft_artifacts.go` requires controller lock |
+| T16 | `internal/modules/craft/lifecycle.go`, `internal/application/repository/craft_workspace.go`, `internal/application/service/craft_workspace.go` |
+| T17 | `internal/application/service/craft_control.go`, `packages/views/src/craft/status-notice.tsx`; `lifecycle.go` requires controller lock |
+| T18 | `internal/modules/craft/recovery.go`, `internal/application/service/craft_recovery.go`, `packages/domain/src/craft/reconnect.ts` |
+| T19 | `internal/modules/craft/budget.go`, `internal/application/service/craft_budget.go`, `packages/views/src/craft/usage.tsx` |
+| T20 | Final central registration, route composition, migration allocation, web adapter, browser journey and acceptance evidence |
+
+Focused tests follow the owning production file. The controller grants an exclusive lock before any lane edits a shared extension file named above. Backend tests using one database, migrations, fixed ports, image builds, and generated assets must be serialized or given isolated resources. No lane uses a shared stash or mutable test database to coordinate another lane.
+
+The frozen Task-access injection contract is `craft.TaskAccessChecker.CheckTaskAccess(context.Context, craft.Scope, craft.TaskAction) error`. T08 owns the implementation; T05 and T14 consume it for fresh authorization. `craft.RequireTaskAccess` fails closed when the checker or authenticated scope is absent. Feature route registration supplies only the already guarded session group; the handler's service must still enforce Task authorization.


## New file internal/container/craft_features.go

--- /dev/null
+++ b/internal/container/craft_features.go
@@ -0,0 +1,18 @@
+package container
+
+import (
+	"fmt"
+
+	"github.com/Tencent/WeKnora/internal/handler/session"
+)
+
+// RegisterCraftFeature installs a lane-owned handler only after its service
+// has been assembled. T20 invokes registrations before router construction.
+// A missing handler fails assembly; routes inherit the authenticated session
+// group and the handler/service remains responsible for Task authorization.
+func RegisterCraftFeature(name string, mount func(session.CraftRouteGroup)) error {
+	if mount == nil {
+		return fmt.Errorf("Craft feature %q unavailable", name)
+	}
+	return session.RegisterCraftFeatureRoute(name, mount)
+}


## New file internal/handler/session/craft_features.go

--- /dev/null
+++ b/internal/handler/session/craft_features.go
@@ -0,0 +1,80 @@
+package session
+
+import (
+	"fmt"
+	"sort"
+	"sync"
+)
+
+// CraftFeatureRoutes is a construction-time registry. Each feature receives
+// only the existing authenticated /sessions route group; it cannot mount an
+// unguarded root route through this seam. Services still check Task access.
+type CraftFeatureRoutes struct {
+	mu     sync.Mutex
+	frozen bool
+	routes map[string]func(CraftRouteGroup)
+}
+
+func NewCraftFeatureRoutes() *CraftFeatureRoutes {
+	return &CraftFeatureRoutes{routes: make(map[string]func(CraftRouteGroup))}
+}
+
+func (r *CraftFeatureRoutes) hasFeatures() bool {
+	r.mu.Lock()
+	defer r.mu.Unlock()
+	return len(r.routes) > 0
+}
+
+func (r *CraftFeatureRoutes) Register(name string, mount func(CraftRouteGroup)) error {
+	if r == nil || mount == nil || name == "" {
+		return fmt.Errorf("invalid Craft feature route")
+	}
+	if name[0] < 'a' || name[0] > 'z' {
+		return fmt.Errorf("invalid Craft feature name %q", name)
+	}
+	for _, ch := range name {
+		if ch < 'a' || ch > 'z' {
+			if ch < '0' || ch > '9' {
+				if ch != '_' && ch != '-' {
+					return fmt.Errorf("invalid Craft feature name %q", name)
+				}
+			}
+		}
+	}
+	r.mu.Lock()
+	defer r.mu.Unlock()
+	if r.frozen {
+		return fmt.Errorf("Craft feature routes already mounted")
+	}
+	if _, exists := r.routes[name]; exists {
+		return fmt.Errorf("duplicate Craft feature %q", name)
+	}
+	r.routes[name] = mount
+	return nil
+}
+
+func (r *CraftFeatureRoutes) Mount(group CraftRouteGroup) error {
+	if r == nil || group == nil {
+		return fmt.Errorf("Craft feature route group unavailable")
+	}
+	r.mu.Lock()
+	defer r.mu.Unlock()
+	r.frozen = true
+	names := make([]string, 0, len(r.routes))
+	for name := range r.routes {
+		names = append(names, name)
+	}
+	sort.Strings(names)
+	for _, name := range names {
+		r.routes[name](group)
+	}
+	return nil
+}
+
+var registeredCraftFeatureRoutes = NewCraftFeatureRoutes()
+
+// RegisterCraftFeatureRoute is called by the container before router build.
+// Failed registration must fail assembly rather than silently omit a feature.
+func RegisterCraftFeatureRoute(name string, mount func(CraftRouteGroup)) error {
+	return registeredCraftFeatureRoutes.Register(name, mount)
+}


## New file internal/handler/session/craft_features_test.go

--- /dev/null
+++ b/internal/handler/session/craft_features_test.go
@@ -0,0 +1,87 @@
+package session
+
+import (
+	"net/http"
+	"net/http/httptest"
+	"reflect"
+	"testing"
+
+	"github.com/Tencent/WeKnora/internal/modules/craft"
+	"github.com/gin-gonic/gin"
+)
+
+func TestCraftFeatureRoutesInheritGuardsAndFreezeInNameOrder(t *testing.T) {
+	gin.SetMode(gin.TestMode)
+	features := NewCraftFeatureRoutes()
+	var mounted []string
+	for _, name := range []string{"sources", "access"} {
+		name := name
+		if err := features.Register(name, func(group craftRouteGroup) {
+			mounted = append(mounted, name)
+			group.GET("/:id/craft/"+name, func(c *gin.Context) { c.Status(http.StatusOK) })
+		}); err != nil {
+			t.Fatal(err)
+		}
+	}
+	if err := features.Register("access", func(craftRouteGroup) {}); err == nil {
+		t.Fatal("duplicate feature name accepted")
+	}
+	if err := features.Register("nil", nil); err == nil {
+		t.Fatal("nil route feature accepted")
+	}
+	if err := features.Register("1invalid", func(CraftRouteGroup) {}); err == nil {
+		t.Fatal("feature name beginning with a digit accepted")
+	}
+	r := gin.New()
+	sessions := r.Group("/sessions", func(c *gin.Context) {
+		if c.GetHeader("X-Test-Auth") != "yes" {
+			c.AbortWithStatus(http.StatusUnauthorized)
+		}
+	})
+	if err := features.Mount(sessions); err != nil {
+		t.Fatal(err)
+	}
+	if !reflect.DeepEqual(mounted, []string{"access", "sources"}) {
+		t.Fatalf("mount order = %v", mounted)
+	}
+	if err := features.Register("late", func(craftRouteGroup) {}); err == nil {
+		t.Fatal("registration accepted after mount")
+	}
+	second := gin.New().Group("/sessions")
+	if err := features.Mount(second); err != nil {
+		t.Fatalf("frozen route table should mount in another guarded router: %v", err)
+	}
+	for _, name := range []string{"access", "sources"} {
+		unauth := httptest.NewRecorder()
+		r.ServeHTTP(unauth, httptest.NewRequest(http.MethodGet, "/sessions/s1/craft/"+name, nil))
+		if unauth.Code != http.StatusUnauthorized {
+			t.Fatalf("%s bypassed parent guard: %d", name, unauth.Code)
+		}
+		auth := httptest.NewRecorder()
+		req := httptest.NewRequest(http.MethodGet, "/sessions/s1/craft/"+name, nil)
+		req.Header.Set("X-Test-Auth", "yes")
+		r.ServeHTTP(auth, req)
+		if auth.Code != http.StatusOK {
+			t.Fatalf("%s route missing: %d", name, auth.Code)
+		}
+	}
+}
+
+func TestCraftOptionalWebFactsPreserveLegacyDTO(t *testing.T) {
+	legacyInput := craftInputDTO(craft.Input{Ref: "r1"})
+	if _, ok := legacyInput["recognition"]; ok {
+		t.Fatal("legacy input gained an unsupported field")
+	}
+	input := craftInputDTO(craft.Input{Ref: "r1", Recognition: &craft.InputRecognition{Accepted: true, Understood: false, Reason: "opaque"}})
+	if input["recognition"] == nil {
+		t.Fatal("recognition fact omitted")
+	}
+	legacyVersion := craftVersionDTO(craft.Version{ID: "v1"})
+	if _, ok := legacyVersion["web_evidence"]; ok {
+		t.Fatal("legacy version gained an unsupported field")
+	}
+	version := craftVersionDTO(craft.Version{ID: "v1", WebEvidence: &craft.WebCheckEvidence{Build: craft.WebCheckPassed, Entry: craft.WebCheckPassed, PreviewReachable: craft.WebCheckPassed, PageLoaded: craft.WebCheckNotRun}})
+	if version["web_evidence"] == nil {
+		t.Fatal("independent web evidence omitted")
+	}
+}


## New file internal/modules/craft/web_contracts.go

--- /dev/null
+++ b/internal/modules/craft/web_contracts.go
@@ -0,0 +1,218 @@
+package craft
+
+import (
+	"context"
+	"fmt"
+)
+
+// TaskAccessChecker is injected by the membership lane. The session ID in
+// Scope is the Task ID; callers must not substitute a second aggregate ID.
+// The checker owns current membership and permission policy, including fresh
+// authorization on reads of restricted originals.
+type TaskAccessChecker interface {
+	CheckTaskAccess(context.Context, Scope, TaskAction) error
+}
+
+type TaskAction string
+
+const (
+	TaskRead       TaskAction = "read"
+	TaskWrite      TaskAction = "write"
+	TaskShare      TaskAction = "share"
+	TaskOpenSource TaskAction = "open_source"
+	TaskPreview    TaskAction = "preview"
+)
+
+// RequireTaskAccess has no permissive default when an assembly lacks T08.
+func RequireTaskAccess(ctx context.Context, checker TaskAccessChecker, scope Scope, action TaskAction) error {
+	if checker == nil {
+		return ErrForbidden
+	}
+	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
+		return ErrForbidden
+	}
+	switch action {
+	case TaskRead, TaskWrite, TaskShare, TaskOpenSource, TaskPreview:
+	default:
+		return ErrInvalidInput
+	}
+	return checker.CheckTaskAccess(ctx, scope, action)
+}
+
+// These records are immutable-version and run facts. They do not grant access
+// or perform a transition: the owning application service remains authoritative.
+type InputRecognition struct {
+	Accepted   bool   `json:"accepted"`
+	Understood bool   `json:"understood"`
+	Reason     string `json:"reason"`
+}
+
+func (r InputRecognition) Validate() error {
+	if r.Understood && !r.Accepted {
+		return fmt.Errorf("%w: understood input was not accepted", ErrInvalidInput)
+	}
+	if r.Accepted && !r.Understood && r.Reason == "" {
+		return fmt.Errorf("%w: unrecognized input needs a reason", ErrInvalidInput)
+	}
+	return nil
+}
+
+type CheckOutcome string
+
+const (
+	WebCheckPassed CheckOutcome = "passed"
+	WebCheckFailed CheckOutcome = "failed"
+	WebCheckNotRun CheckOutcome = "not_run"
+)
+
+func (s CheckOutcome) valid() bool {
+	return s == WebCheckPassed || s == WebCheckFailed || s == WebCheckNotRun
+}
+
+// Each gate is separate evidence; page load cannot be inferred from HTTP reachability.
+type WebCheckEvidence struct {
+	Build            CheckOutcome `json:"build"`
+	Entry            CheckOutcome `json:"entry"`
+	PreviewReachable CheckOutcome `json:"preview_reachable"`
+	PageLoaded       CheckOutcome `json:"page_loaded"`
+}
+
+func (e WebCheckEvidence) Validate() error {
+	if !e.Build.valid() || !e.Entry.valid() || !e.PreviewReachable.valid() || !e.PageLoaded.valid() {
+		return fmt.Errorf("%w: invalid web check outcome", ErrInvalidInput)
+	}
+	return nil
+}
+func (e WebCheckEvidence) Ready() bool {
+	return e.Validate() == nil && e.Build == WebCheckPassed && e.Entry == WebCheckPassed && e.PreviewReachable == WebCheckPassed && e.PageLoaded == WebCheckPassed
+}
+
+type StopOutcomeStatus string
+
+const (
+	StopRequested StopOutcomeStatus = "requested"
+	StopConfirmed StopOutcomeStatus = "confirmed"
+	StopUnknown   StopOutcomeStatus = "unknown"
+)
+
+type StopOutcome struct {
+	RunID  string            `json:"run_id"`
+	Status StopOutcomeStatus `json:"status"`
+}
+
+func (o StopOutcome) Validate() error {
+	if o.RunID == "" || (o.Status != StopRequested && o.Status != StopConfirmed && o.Status != StopUnknown) {
+		return fmt.Errorf("%w: invalid stop outcome", ErrInvalidInput)
+	}
+	return nil
+}
+
+type WriterAcquireStatus string
+
+const (
+	WriterAcquired WriterAcquireStatus = "acquired"
+	WriterConflict WriterAcquireStatus = "conflict"
+	WriterUnknown  WriterAcquireStatus = "unknown"
+)
+
+type WriterAcquireOutcome struct {
+	WorkspaceID string              `json:"workspace_id"`
+	Status      WriterAcquireStatus `json:"status"`
+}
+
+func (o WriterAcquireOutcome) Validate() error {
+	if o.WorkspaceID == "" || (o.Status != WriterAcquired && o.Status != WriterConflict && o.Status != WriterUnknown) {
+		return fmt.Errorf("%w: invalid writer acquisition", ErrInvalidInput)
+	}
+	return nil
+}
+
+type BudgetPause struct {
+	RunID  string `json:"run_id"`
+	Reason string `json:"reason"`
+	Limit  int64  `json:"limit"`
+	Used   int64  `json:"used"`
+}
+
+func (p BudgetPause) Validate() error {
+	if p.RunID == "" || p.Reason == "" || p.Limit < 0 || p.Used < 0 {
+		return fmt.Errorf("%w: invalid budget pause", ErrInvalidInput)
+	}
+	return nil
+}
+
+type RestrictedContribution struct {
+	VersionID      string `json:"version_id"`
+	EvidenceDigest string `json:"evidence_digest"`
+	Restricted     bool   `json:"restricted"`
+}
+
+func (c RestrictedContribution) Validate() error {
+	if c.VersionID == "" || c.EvidenceDigest == "" {
+		return fmt.Errorf("%w: invalid restricted contribution", ErrInvalidInput)
+	}
+	return nil
+}
+
+type DecisionStatus string
+
+const (
+	DecisionApproved DecisionStatus = "approved"
+	DecisionRejected DecisionStatus = "rejected"
+	DecisionUnknown  DecisionStatus = "unknown"
+)
+
+func (d DecisionStatus) valid() bool {
+	return d == DecisionApproved || d == DecisionRejected || d == DecisionUnknown
+}
+
+type ShareDecision struct {
+	VersionID      string         `json:"version_id"`
+	EvidenceDigest string         `json:"evidence_digest"`
+	OwnerID        string         `json:"owner_id"`
+	Decision       DecisionStatus `json:"decision"`
+}
+
+func (d ShareDecision) Validate() error {
+	if d.VersionID == "" || d.EvidenceDigest == "" || d.OwnerID == "" || !d.Decision.valid() {
+		return fmt.Errorf("%w: invalid share decision", ErrInvalidInput)
+	}
+	return nil
+}
+
+type ExportFile struct {
+	Path       string `json:"path"`
+	SHA256     string `json:"sha256"`
+	Restricted bool   `json:"restricted"`
+}
+type ExportManifest struct {
+	VersionID      string       `json:"version_id"`
+	ManifestDigest string       `json:"manifest_digest"`
+	Files          []ExportFile `json:"files"`
+}
+
+func (m ExportManifest) Validate() error {
+	if m.VersionID == "" || m.ManifestDigest == "" {
+		return fmt.Errorf("%w: invalid export manifest", ErrInvalidInput)
+	}
+	for _, f := range m.Files {
+		if f.Path == "" || f.SHA256 == "" {
+			return fmt.Errorf("%w: invalid export file", ErrInvalidInput)
+		}
+	}
+	return nil
+}
+
+type ExportDecision struct {
+	VersionID      string         `json:"version_id"`
+	ManifestDigest string         `json:"manifest_digest"`
+	OwnerID        string         `json:"owner_id"`
+	Decision       DecisionStatus `json:"decision"`
+}
+
+func (d ExportDecision) Validate() error {
+	if d.VersionID == "" || d.ManifestDigest == "" || d.OwnerID == "" || !d.Decision.valid() {
+		return fmt.Errorf("%w: invalid export decision", ErrInvalidInput)
+	}
+	return nil
+}


## New file internal/modules/craft/web_contracts_test.go

--- /dev/null
+++ b/internal/modules/craft/web_contracts_test.go
@@ -0,0 +1,74 @@
+package craft
+
+import "testing"
+
+func TestWebContractFacts(t *testing.T) {
+	input := InputRecognition{Accepted: true, Understood: false, Reason: "unrecognized_extension"}
+	if err := input.Validate(); err != nil {
+		t.Fatal(err)
+	}
+	if err := (InputRecognition{Accepted: false, Understood: true}).Validate(); err == nil {
+		t.Fatal("understood input cannot be unaccepted")
+	}
+
+	evidence := WebCheckEvidence{Build: WebCheckPassed, Entry: WebCheckPassed, PreviewReachable: WebCheckPassed, PageLoaded: WebCheckNotRun}
+	if err := evidence.Validate(); err != nil {
+		t.Fatal(err)
+	}
+	if evidence.Ready() {
+		t.Fatal("page load has not passed")
+	}
+	evidence.PageLoaded = WebCheckPassed
+	if !evidence.Ready() {
+		t.Fatal("all four checks passed")
+	}
+	evidence.PageLoaded = CheckOutcome("maybe")
+	if err := evidence.Validate(); err == nil {
+		t.Fatal("unknown check outcome accepted")
+	}
+
+	for _, state := range []StopOutcomeStatus{StopRequested, StopConfirmed, StopUnknown} {
+		if err := (StopOutcome{Status: state, RunID: "run-1"}).Validate(); err != nil {
+			t.Fatal(err)
+		}
+	}
+	if err := (StopOutcome{Status: StopOutcomeStatus("maybe"), RunID: "run-1"}).Validate(); err == nil {
+		t.Fatal("unknown stop status accepted")
+	}
+	for _, state := range []WriterAcquireStatus{WriterAcquired, WriterConflict, WriterUnknown} {
+		if err := (WriterAcquireOutcome{Status: state, WorkspaceID: "ws-1"}).Validate(); err != nil {
+			t.Fatal(err)
+		}
+	}
+	if err := (WriterAcquireOutcome{Status: WriterAcquireStatus("maybe"), WorkspaceID: "ws-1"}).Validate(); err == nil {
+		t.Fatal("unknown writer status accepted")
+	}
+}
+
+func TestWebConsentAndBudgetContracts(t *testing.T) {
+	if err := (BudgetPause{RunID: "run-1", Reason: "exhausted", Limit: 100, Used: 100}).Validate(); err != nil {
+		t.Fatal(err)
+	}
+	if err := (BudgetPause{RunID: "run-1", Reason: "exhausted", Limit: -1}).Validate(); err == nil {
+		t.Fatal("negative budget accepted")
+	}
+	if err := (RestrictedContribution{VersionID: "v1", EvidenceDigest: "sha256:a", Restricted: true}).Validate(); err != nil {
+		t.Fatal(err)
+	}
+	if err := (ShareDecision{VersionID: "v1", EvidenceDigest: "sha256:a", OwnerID: "u1", Decision: DecisionApproved}).Validate(); err != nil {
+		t.Fatal(err)
+	}
+	if err := (ShareDecision{VersionID: "v1", EvidenceDigest: "sha256:a", OwnerID: "u1", Decision: DecisionStatus("maybe")}).Validate(); err == nil {
+		t.Fatal("unknown share decision accepted")
+	}
+	manifest := ExportManifest{VersionID: "v1", ManifestDigest: "sha256:m", Files: []ExportFile{{Path: "index.html", SHA256: "sha256:f", Restricted: false}}}
+	if err := manifest.Validate(); err != nil {
+		t.Fatal(err)
+	}
+	if err := (ExportDecision{VersionID: "v1", ManifestDigest: "sha256:m", OwnerID: "u1", Decision: DecisionRejected}).Validate(); err != nil {
+		t.Fatal(err)
+	}
+	if err := (ExportDecision{VersionID: "v1", ManifestDigest: "", OwnerID: "u1", Decision: DecisionApproved}).Validate(); err == nil {
+		t.Fatal("unbound export decision accepted")
+	}
+}


## New file packages/contracts/src/craft/web-artifact.test.ts

--- /dev/null
+++ b/packages/contracts/src/craft/web-artifact.test.ts
@@ -0,0 +1,58 @@
+import { test } from 'node:test';
+import assert from 'node:assert/strict';
+import {
+  parseCraftInputView,
+  parseCraftRunView,
+  parseCraftVersionView,
+  parseCraftInputRecognition,
+  parseCraftWebCheckEvidence,
+  parseCraftStopOutcome,
+  parseCraftWriterAcquireOutcome,
+  parseCraftBudgetPause,
+  parseCraftRestrictedContribution,
+  parseCraftShareDecision,
+  parseCraftExportManifest,
+  parseCraftExportDecision,
+} from './index.ts';
+
+const version = { id: 'v1', workspace_id: 'w1', run_id: 'r1', kind: 'web', files: [], checks: [] };
+
+test('input acceptance is separate from understanding and old responses still parse', () => {
+  assert.deepEqual(parseCraftInputRecognition({ accepted: true, understood: false, reason: 'unrecognized_extension' }), { accepted: true, understood: false, reason: 'unrecognized_extension' });
+  assert.throws(() => parseCraftInputRecognition({ accepted: false, understood: true, reason: '' }), /understood/);
+  assert.equal(parseCraftInputView({ ref: 'r', name: 'x', sha256: 'h', bytes: 1, citation_id: '' }).recognition, null);
+  assert.equal(parseCraftInputView({ ref: 'r', name: 'x', sha256: 'h', bytes: 1, citation_id: '', recognition: { accepted: true, understood: false, reason: 'opaque' } }).recognition?.understood, false);
+});
+
+test('version keeps four independent gates and rejects unknown statuses', () => {
+  const evidence = { build: 'passed', entry: 'passed', preview_reachable: 'passed', page_loaded: 'not_run' };
+  assert.deepEqual(parseCraftWebCheckEvidence(evidence), evidence);
+  assert.equal(parseCraftVersionView(version).web_evidence, null);
+  assert.equal(parseCraftVersionView({ ...version, web_evidence: evidence }).web_evidence?.page_loaded, 'not_run');
+  assert.throws(() => parseCraftWebCheckEvidence({ ...evidence, page_loaded: 'maybe' }), /page_loaded/);
+});
+
+test('stop and writer outcomes preserve unknown explicitly', () => {
+  assert.equal(parseCraftStopOutcome({ run_id: 'r1', status: 'unknown' }).status, 'unknown');
+  assert.equal(parseCraftWriterAcquireOutcome({ workspace_id: 'w1', status: 'unknown' }).status, 'unknown');
+  assert.throws(() => parseCraftStopOutcome({ run_id: 'r1', status: 'canceled' }), /status/);
+  assert.throws(() => parseCraftWriterAcquireOutcome({ workspace_id: 'w1', status: 'locked' }), /status/);
+});
+
+test('budget and restricted consent records bind immutable identities', () => {
+  assert.equal(parseCraftBudgetPause({ run_id: 'r1', reason: 'exhausted', limit: 100, used: 100 }).used, 100);
+  assert.throws(() => parseCraftBudgetPause({ run_id: 'r1', reason: 'exhausted', limit: -1, used: 0 }), /limit/);
+  assert.equal(parseCraftRestrictedContribution({ version_id: 'v1', evidence_digest: 'sha256:a', restricted: true }).restricted, true);
+  assert.equal(parseCraftShareDecision({ version_id: 'v1', evidence_digest: 'sha256:a', owner_id: 'u1', decision: 'approved' }).decision, 'approved');
+  assert.throws(() => parseCraftShareDecision({ version_id: 'v1', evidence_digest: 'sha256:a', owner_id: 'u1', decision: 'maybe' }), /decision/);
+  assert.equal(parseCraftExportManifest({ version_id: 'v1', manifest_digest: 'sha256:m', files: [{ path: 'index.html', sha256: 'sha256:f', restricted: false }] }).files.length, 1);
+  assert.equal(parseCraftExportDecision({ version_id: 'v1', manifest_digest: 'sha256:m', owner_id: 'u1', decision: 'rejected' }).decision, 'rejected');
+  assert.throws(() => parseCraftExportDecision({ version_id: 'v1', manifest_digest: '', owner_id: 'u1', decision: 'approved' }), /manifest_digest/);
+});
+
+test('run budget pause is a typed optional fact on the existing waiting state', () => {
+  const base = { run_id: 'r1', session_id: 's1', status: 'waiting_user', wait_reason: 'budget', revision: 1, epoch: 1, seq: 2, pending_id: 'p1' };
+  assert.equal(parseCraftRunView(base).budget_pause, null);
+  assert.equal(parseCraftRunView({ ...base, budget_pause: { run_id: 'r1', reason: 'exhausted', limit: 100, used: 100 } }).budget_pause?.used, 100);
+  assert.throws(() => parseCraftRunView({ ...base, budget_pause: { run_id: 'r1', reason: 'exhausted', limit: -1, used: 100 } }), /limit/);
+});


## New file packages/contracts/src/craft/web-artifact.ts

--- /dev/null
+++ b/packages/contracts/src/craft/web-artifact.ts
@@ -0,0 +1,79 @@
+/** Additive Craft web-artifact facts. These records describe server decisions;
+ * parsing them never grants permission or changes a Run. */
+export const CRAFT_WEB_CHECK_OUTCOMES = ['passed', 'failed', 'not_run'] as const;
+export type CraftWebCheckOutcome = (typeof CRAFT_WEB_CHECK_OUTCOMES)[number];
+export const CRAFT_STOP_OUTCOMES = ['requested', 'confirmed', 'unknown'] as const;
+export type CraftStopOutcomeStatus = (typeof CRAFT_STOP_OUTCOMES)[number];
+export const CRAFT_WRITER_ACQUIRE_OUTCOMES = ['acquired', 'conflict', 'unknown'] as const;
+export type CraftWriterAcquireStatus = (typeof CRAFT_WRITER_ACQUIRE_OUTCOMES)[number];
+export const CRAFT_DECISIONS = ['approved', 'rejected', 'unknown'] as const;
+export type CraftDecisionStatus = (typeof CRAFT_DECISIONS)[number];
+
+export interface CraftInputRecognition { accepted: boolean; understood: boolean; reason: string }
+export interface CraftWebCheckEvidence { build: CraftWebCheckOutcome; entry: CraftWebCheckOutcome; preview_reachable: CraftWebCheckOutcome; page_loaded: CraftWebCheckOutcome }
+export interface CraftStopOutcome { run_id: string; status: CraftStopOutcomeStatus }
+export interface CraftWriterAcquireOutcome { workspace_id: string; status: CraftWriterAcquireStatus }
+export interface CraftBudgetPause { run_id: string; reason: string; limit: number; used: number }
+export interface CraftRestrictedContribution { version_id: string; evidence_digest: string; restricted: boolean }
+export interface CraftShareDecision { version_id: string; evidence_digest: string; owner_id: string; decision: CraftDecisionStatus }
+export interface CraftExportFile { path: string; sha256: string; restricted: boolean }
+export interface CraftExportManifest { version_id: string; manifest_digest: string; files: CraftExportFile[] }
+export interface CraftExportDecision { version_id: string; manifest_digest: string; owner_id: string; decision: CraftDecisionStatus }
+
+function row(value: unknown, label: string): Record<string, unknown> {
+  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid ' + label);
+  return value as Record<string, unknown>;
+}
+function id(value: unknown, field: string): string {
+  if (typeof value !== 'string' || value.trim() === '') throw new Error('invalid ' + field);
+  return value;
+}
+function flag(value: unknown, field: string): boolean {
+  if (typeof value !== 'boolean') throw new Error('invalid ' + field);
+  return value;
+}
+function count(value: unknown, field: string): number {
+  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) throw new Error('invalid ' + field);
+  return value;
+}
+function choice<T extends string>(value: unknown, choices: readonly T[], field: string): T {
+  if (typeof value !== 'string' || !(choices as readonly string[]).includes(value)) throw new Error('invalid ' + field);
+  return value as T;
+}
+
+export function parseCraftInputRecognition(value: unknown): CraftInputRecognition {
+  const v = row(value, 'recognition');
+  const accepted = flag(v.accepted, 'accepted');
+  const understood = flag(v.understood, 'understood');
+  if (understood && !accepted) throw new Error('invalid understood');
+  const reason = typeof v.reason === 'string' ? v.reason : '';
+  if (accepted && !understood && reason === '') throw new Error('invalid reason');
+  return { accepted, understood, reason };
+}
+export function parseCraftWebCheckEvidence(value: unknown): CraftWebCheckEvidence {
+  const v = row(value, 'web evidence');
+  return { build: choice(v.build, CRAFT_WEB_CHECK_OUTCOMES, 'build'), entry: choice(v.entry, CRAFT_WEB_CHECK_OUTCOMES, 'entry'), preview_reachable: choice(v.preview_reachable, CRAFT_WEB_CHECK_OUTCOMES, 'preview_reachable'), page_loaded: choice(v.page_loaded, CRAFT_WEB_CHECK_OUTCOMES, 'page_loaded') };
+}
+export function parseCraftStopOutcome(value: unknown): CraftStopOutcome {
+  const v = row(value, 'stop outcome'); return { run_id: id(v.run_id, 'run_id'), status: choice(v.status, CRAFT_STOP_OUTCOMES, 'status') };
+}
+export function parseCraftWriterAcquireOutcome(value: unknown): CraftWriterAcquireOutcome {
+  const v = row(value, 'writer acquisition'); return { workspace_id: id(v.workspace_id, 'workspace_id'), status: choice(v.status, CRAFT_WRITER_ACQUIRE_OUTCOMES, 'status') };
+}
+export function parseCraftBudgetPause(value: unknown): CraftBudgetPause {
+  const v = row(value, 'budget pause'); return { run_id: id(v.run_id, 'run_id'), reason: id(v.reason, 'reason'), limit: count(v.limit, 'limit'), used: count(v.used, 'used') };
+}
+export function parseCraftRestrictedContribution(value: unknown): CraftRestrictedContribution {
+  const v = row(value, 'restricted contribution'); return { version_id: id(v.version_id, 'version_id'), evidence_digest: id(v.evidence_digest, 'evidence_digest'), restricted: flag(v.restricted, 'restricted') };
+}
+export function parseCraftShareDecision(value: unknown): CraftShareDecision {
+  const v = row(value, 'share decision'); return { version_id: id(v.version_id, 'version_id'), evidence_digest: id(v.evidence_digest, 'evidence_digest'), owner_id: id(v.owner_id, 'owner_id'), decision: choice(v.decision, CRAFT_DECISIONS, 'decision') };
+}
+export function parseCraftExportManifest(value: unknown): CraftExportManifest {
+  const v = row(value, 'export manifest');
+  if (!Array.isArray(v.files)) throw new Error('invalid files');
+  return { version_id: id(v.version_id, 'version_id'), manifest_digest: id(v.manifest_digest, 'manifest_digest'), files: v.files.map((file) => { const f = row(file, 'export file'); return { path: id(f.path, 'path'), sha256: id(f.sha256, 'sha256'), restricted: flag(f.restricted, 'restricted') }; }) };
+}
+export function parseCraftExportDecision(value: unknown): CraftExportDecision {
+  const v = row(value, 'export decision'); return { version_id: id(v.version_id, 'version_id'), manifest_digest: id(v.manifest_digest, 'manifest_digest'), owner_id: id(v.owner_id, 'owner_id'), decision: choice(v.decision, CRAFT_DECISIONS, 'decision') };
+}


## New file packages/views/src/craft/workbench-features.tsx

--- /dev/null
+++ b/packages/views/src/craft/workbench-features.tsx
@@ -0,0 +1,31 @@
+import type { ReactNode } from 'react';
+
+export type CraftWorkbenchSlot = 'header' | 'conversation' | 'aside';
+export interface CraftWorkbenchFeatureContext {
+  sessionId: string;
+  canWrite: boolean;
+  selectedVersionId: string | null;
+}
+export interface CraftWorkbenchFeature {
+  name: string;
+  slot: CraftWorkbenchSlot;
+  render(context: CraftWorkbenchFeatureContext): ReactNode;
+}
+
+const slots: readonly CraftWorkbenchSlot[] = ['header', 'conversation', 'aside'];
+
+// Construct once in the web adapter before rendering. The result is immutable
+// so feature registration cannot vary across re-renders or a resumed Task.
+export function createCraftWorkbenchFeatures(entries: readonly CraftWorkbenchFeature[]): readonly CraftWorkbenchFeature[] {
+  const names = new Set<string>();
+  const sorted = [...entries].sort((a, b) => a.name.localeCompare(b.name));
+  for (const entry of sorted) {
+    if (!/^[a-z][a-z0-9_-]*$/.test(entry.name)) throw new Error('invalid Craft feature name');
+    if (names.has(entry.name)) throw new Error('duplicate Craft feature name');
+    if (!slots.includes(entry.slot)) throw new Error('invalid Craft feature slot');
+    if (typeof entry.render !== 'function') throw new Error('Craft feature render unavailable');
+    names.add(entry.name);
+    Object.freeze(entry);
+  }
+  return Object.freeze(sorted);
+}
