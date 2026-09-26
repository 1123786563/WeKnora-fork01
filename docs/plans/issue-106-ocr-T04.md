Review complete: 2 finding(s) across 2 selected item(s).

─── examples/plugins/jira-todo-mcp/oauth.go:547-549 ───
[bug · low] randomToken() 在 s.mu.Lock() 与 s.mu.Unlock() 之间调用（本函数无 defer 解锁）。randomToken 在
crypto/rand 不可用时按设计 fail-closed panic——此前临界区内没有任何可 panic 的调用，这次改动首次引入：panic 会让 s.mu 永久保持锁定，而该锁同时被
/register、/token、/authorize 及 /mcp 每请求的 lookupSession 依赖（见 validateAuthorizationRequest 注释），等于整个
OAuth 面与已授权工具调用全部死锁，比 handler 崩溃更糟。nonce 生成不依赖临界区内任何状态，建议移到加锁之前（与其他 randomToken 调用点，如 clientID/code
均在锁外生成的纪律一致）。

+ 	// nonce 生成不依赖临界区状态，且 randomToken 可能 panic——
+ 	// 必须在锁外生成，避免 panic 永久持有 s.mu。
  	formNonce := randomToken()
- 	s.pendingAuths[state] = pendingAuth{Request: req, ExpiresAt: now.Add(pendingAuthTTL), FormNonce: formNonce}
- 	s.mu.Unlock()
+ 	now := time.Now()
+ 	s.mu.Lock()
+ 	s.maybeSweepLocked(now)


─── examples/plugins/jira-todo-mcp/main.go:225-227 ───
[maintainability · low] 该启动期校验漏掉了同一类「静默永不命中」的死条目：形如 "host:"（尾随冒号）的条目 net.SplitHostPort 会成功返回空
port（不报错），通过本校验；但任何合法 URI 的 parsed.Host 都不可能是 "host:"（url.Parse 对 "http://host:/" 报 invalid
port），handleRegister 的 entry == host / entry == bareHost 也都命不中——与 :80/:443
完全同类的静默失效。若它是名单里唯一条目，结果是所有注册被拒而运营者无从得知原因。建议对空 port 一并 fail-closed。

- 		if port == "80" || port == "443" {
- 			return fmt.Errorf("AllowedRedirectHosts entry %q pins a default port that URIs normally elide (url.Parse does not materialize it); use the bare host instead", entry)
+ 		if port == "" || port == "80" || port == "443" {
+ 			return fmt.Errorf("AllowedRedirectHosts entry %q can never match a redirect_uri host (empty or default port that URIs elide); use the bare host instead", entry)
  		}


LLM retry report summary: 1 of 12 requests affected -- 1 request failed

Core review (1 request):
- examples/plugins/jira-todo-mcp/main.go,examples/plugins/jira-todo-mcp/oauth.go: timed out -> failed

Per-attempt detail: --format json (retry_report).
