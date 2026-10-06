# Commercial webhook anonymous authentication — Review Fix 1 Task 1 report

## Scope and checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-webhook-provider/WeKnora-fork01`
- Baseline and final HEAD: `6e1b2072a13a798e7ec25be44784e5242bd2f178` (no commit)
- Owned files: `internal/router/commercial_callback_public_test.go`, `internal/middleware/auth.go`.
- Initial tracked diff in owned files was empty (SHA-256 of empty scoped diff: `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`).
- Initial owned file hashes:
  - `internal/router/commercial_callback_public_test.go`: `6254d814ca693860c816c1ed18adc14ca5cb0169ba5677cc4eb9e24b437307bf`
  - `internal/middleware/auth.go`: `57dd24c4241dc022f08fa1c2e5e5f00898a1945ba28e73aac6cf3aa19d93841b`
- Previous provider Task 1 checkpoint remained unchanged before and after this task:
  - `internal/container/bootsmoke/boot_smoke_test.go`: `e56723e5f15c7953d39fa86c8ae799e8c73c461098017c779c15e7b78f00d23a`
  - `internal/container/container.go`: `51e083922d82feafe00237e68a8436ce27fdac49c2812af35d81b581848b7e21`
  - its scoped binary diff SHA-256 remains `47793c658a15852ca42e5cd577347c607ed859df236973b9d52cc96f1e009ca4`.

## Changes

- Added a real router test mounting `middleware.Auth` before `/api/v1` and registering the commercial webhook route with `handler.NewCommercialWebhookHandler(nil)`.
- Added the exact POST-only prefix exemption `"/api/v1/commercial/webhooks/*": {"POST"}` to `noAuthAPI`, documenting the raw-body HMAC boundary and direct `/api/v1` auth middleware.
- The new test asserts webhook POST returns handler fail-closed 503, while webhook GET and unrelated commercial POST both return 401.

## TDD and verification evidence

1. RED, before editing `internal/middleware/auth.go`:
   ```text
   go test ./internal/router -run '^TestCommercialWebhookRouteIsAnonymouslyReachable$' -count=1
   ```
   Result: failed as expected. `anonymous POST reaches fail-closed webhook handler` received `401` with `{"error":"Unauthorized: missing authentication"}` instead of 503. Exit status 1.
2. GREEN after adding the POST-only exemption:
   ```text
   go test ./internal/router -run 'TestProviderCallbackRouteIsAnonymouslyReachable|TestCommercialWebhookRouteIsAnonymouslyReachable' -count=1
   ```
   Result: `ok github.com/Tencent/WeKnora/internal/router 1.156s` (exit status 0). This includes the new positive test, GET 401 negative control, unrelated POST 401 negative control, and existing anonymous payment callback test.
3. Middleware whitelist tests:
   ```text
   go test ./internal/middleware -run TestIsNoAuthAPI -count=1
   ```
   Result: `ok github.com/Tencent/WeKnora/internal/middleware 0.718s` (exit status 0).
4. Scoped whitespace validation:
   ```text
   git diff --check -- internal/router/commercial_callback_public_test.go internal/middleware/auth.go
   ```
   Result: clean (exit status 0).

## Final diff and hashes

Final owned source file SHA-256:

```text
ecc95789875c1227e04e9ae85da4829be52ceabcddd48413f3ddbe307e86e810  internal/router/commercial_callback_public_test.go
500e6c5c33ad7eccf0d45e87bea3f3b63fa64add379bb663715545a8e62e588c  internal/middleware/auth.go
```

Owned binary diff SHA-256:

```text
f074659c36e7c8077462e7e58b19f8f734cc7d2eff8adad0e738c61a684cf5b3
```

Scoped diff:

```diff
--- a/internal/middleware/auth.go
+++ b/internal/middleware/auth.go
@@
 	"/api/v1/commercial/callbacks/*": {"POST"},
+	// Billing-authority webhooks use an HMAC signature over the raw request
+	// body as their authentication boundary. Auth wraps /api/v1 directly, so
+	// only POSTs on this webhook path bypass session/API-key authentication.
+	"/api/v1/commercial/webhooks/*": {"POST"},
--- a/internal/router/commercial_callback_public_test.go
+++ b/internal/router/commercial_callback_public_test.go
@@
 }
+
+func TestCommercialWebhookRouteIsAnonymouslyReachable(t *testing.T) {
+	gin.SetMode(gin.TestMode)
+	engine := gin.New()
+	engine.Use(middleware.Auth(nil, nil, nil, nil, nil))
+	v1 := engine.Group("/api/v1")
+	RegisterCommercialRoutes(v1, handler.NewCommercialHandler(nil), handler.NewCommercialWebhookHandler(nil))
+	// Anonymous webhook POST expects 503 from the nil handler; GET and unrelated
+	// commercial POSTs expect 401 from the real Auth middleware.
+}
```

No staging or commit was performed. Existing provider source changes and the prior report were not modified.

The diff above summarizes the test addition. The following is the complete final owned diff captured from Git:

```diff
diff --git a/internal/middleware/auth.go b/internal/middleware/auth.go
index a65bbd3fb..94431b625 100644
--- a/internal/middleware/auth.go
+++ b/internal/middleware/auth.go
@@ -78,6 +78,10 @@ var noAuthAPI = map[string][]string{
 	// (routes_commercial.go mounts the group under the authenticated /api/v1
 	// tree by design; the fail-closed handler rejects unsigned bodies).
 	"/api/v1/commercial/callbacks/*": {"POST"},
+	// Billing-authority webhooks use an HMAC signature over the raw request
+	// body as their authentication boundary. Auth wraps /api/v1 directly, so
+	// only POSTs on this webhook path bypass session/API-key authentication.
+	"/api/v1/commercial/webhooks/*": {"POST"},
 }
 
 // 检查请求是否在无需认证的API列表中
diff --git a/internal/router/commercial_callback_public_test.go b/internal/router/commercial_callback_public_test.go
index 7537c3c94..91dc9ef8c 100644
--- a/internal/router/commercial_callback_public_test.go
+++ b/internal/router/commercial_callback_public_test.go
@@ -58,3 +58,38 @@ func TestProviderCallbackRouteIsAnonymouslyReachable(t *testing.T) {
 		t.Fatalf("non-callback commercial route must still require auth, got %d: %s", w.Code, w.Body.String())
 	}
 }
+
+func TestCommercialWebhookRouteIsAnonymouslyReachable(t *testing.T) {
+	gin.SetMode(gin.TestMode)
+	engine := gin.New()
+	engine.Use(middleware.Auth(nil, nil, nil, nil, nil))
+	v1 := engine.Group("/api/v1")
+	RegisterCommercialRoutes(v1, handler.NewCommercialHandler(nil), handler.NewCommercialWebhookHandler(nil))
+
+	t.Run("anonymous POST reaches fail-closed webhook handler", func(t *testing.T) {
+		w := httptest.NewRecorder()
+		req := httptest.NewRequest(http.MethodPost, "/api/v1/commercial/webhooks/lago", strings.NewReader("body=1"))
+		engine.ServeHTTP(w, req)
+		if w.Code != http.StatusServiceUnavailable {
+			t.Fatalf("anonymous webhook POST must reach the nil handler and fail closed with 503, got %d: %s", w.Code, w.Body.String())
+		}
+	})
+
+	t.Run("webhook GET remains authenticated", func(t *testing.T) {
+		w := httptest.NewRecorder()
+		req := httptest.NewRequest(http.MethodGet, "/api/v1/commercial/webhooks/lago", nil)
+		engine.ServeHTTP(w, req)
+		if w.Code != http.StatusUnauthorized {
+			t.Fatalf("anonymous webhook GET must remain behind auth, got %d: %s", w.Code, w.Body.String())
+		}
+	})
+
+	t.Run("unrelated commercial POST remains authenticated", func(t *testing.T) {
+		w := httptest.NewRecorder()
+		req := httptest.NewRequest(http.MethodPost, "/api/v1/commercial/orders", strings.NewReader("{}"))
+		engine.ServeHTTP(w, req)
+		if w.Code != http.StatusUnauthorized {
+			t.Fatalf("anonymous unrelated commercial POST must remain behind auth, got %d: %s", w.Code, w.Body.String())
+		}
+	})
+}
```
