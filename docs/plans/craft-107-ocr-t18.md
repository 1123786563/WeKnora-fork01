Review complete: 1 finding(s) across 2 selected item(s).

─── internal/modules/workbench/service/workbench/notification.go:89-94 ───
[bug · high] 恢复循环的 prefix 分支锚点错误，多洞窗口下确定性活锁（零进度），重新引入本修复声称消除的饥饿。

根因：HolePositions 返回的 contiguousBefore/resumeAfter 是相对于「产生该错误的那次 ReadEvents 的游标」的。当 readErr
来自跳读（ReadEvents(key, resume, limit)，锚在 resume > after）时，contiguous > after 并不意味着 (after, contiguous]
区间从 after 起连续——原始洞 (after+1..resume) 仍在。此时从 after 重读前缀必然在首页行就撞回原洞，得到与最初完全相同的错误 {after,
resume}，下一轮又跳读、又得到同样的第二洞错误……循环在两个错误间确定性交替。

复现（保留行 {40, 50..101}——ApplyEventRetention 保留 usage.*/approval.*/run_completed
等受保护族，前缀裁剪后原地存活，多洞是设计的正常产物；通知 checkpoint 落后于裁剪水位，after=10，limit=256）：
1. ReadEvents(10,256) → 首行 40≠11 → err{contiguousBefore:10, resumeAfter:39} → contiguous==after →
跳读；
2. ReadEvents(39,256) → 40✓ 后 50≠41 → err{40,49} → contiguous(40)>after(10) → prefix；
3. ReadEvents(10,30) → 首行 40≠11 → err{10,39}（与第 1 步相同）→ 跳读 → 回到第 2 步，永久交替。

后果：烧完 1024 次迭代（每次 1–2 条 DB 查询，每 2s tick 约 2000 次查询）后返回 ErrCursorExpired，checkpoint 停在
10；NotificationWorker.RunOnce 收到错误即 return，中止整个 run 扫描——该 run 及字典序靠后的所有 run 的通知投影被永久饿死，与注释「every
call makes strictly forward progress」相悖。

修复：前缀重读必须锚定在产生 readErr 的那次读取的游标上（用 root 跟踪：初始 root=after；跳读失败后 root=resume；prefix 读取失败后 root
不变），成功时返回 (root, prefix) 使 checkpoint 推进到洞沿，从而恢复每次迭代严格前向推进。

- 		if contiguous > after {
- 			// The prefix up to the hole's edge is contiguous and
- 			// projectable: return exactly it (a bounded read of
- 			// contiguous-after rows can never reach the hole) and let the
- 			// checkpoint advance to the edge; the NEXT pass crosses.
- 			prefix, err := p.runs.ReadEvents(ctx, key, after, int(contiguous-after))
+ 	root := after // cursor the current readErr is anchored at
+ 	for attempt := 0; attempt < maxCursorHoleSkips; attempt++ {
+ 		contiguous, resume, ok := repository.HolePositions(readErr)
+ 		if !ok {
+ 			first, err := p.runs.FirstEventSeq(ctx, key)
+ 			if err != nil {
+ 				return after, nil, err
+ 			}
+ 			if first == 0 {
+ 				return after, nil, nil
+ 			}
+ 			contiguous, resume = after, first-1
+ 		}
+ 		if contiguous > root {
+ 			// contiguousBefore is relative to the READ that produced the
+ 			// error: re-read the prefix anchored at that read's cursor,
+ 			// never at the original after once a jump has happened.
+ 			prefix, err := p.runs.ReadEvents(ctx, key, root, int(contiguous-root))
+ 			if err == nil {
+ 				return root, prefix, nil
+ 			}
+ 			if !errors.Is(err, agentruntime.ErrCursorExpired) {
+ 				return after, nil, err
+ 			}
+ 			readErr = err // anchored at root; root unchanged
+ 			continue
+ 		}
+ 		events, err := p.runs.ReadEvents(ctx, key, resume, limit)
+ 		if err == nil {
+ 			return resume, events, nil
+ 		}
+ 		if !errors.Is(err, agentruntime.ErrCursorExpired) {
+ 			return after, nil, err
+ 		}
+ 		readErr = err // anchored at resume
+ 		root = resume
+ 	}


LLM retry report summary: 2 of 19 requests affected -- 1 request failed, 1 request recovered after retry

Review planning (1 request):
- internal/application/repository/agent_run_events.go,internal/modules/workbench/service/workbench/notification.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed

Core review (1 request):
- internal/application/repository/agent_run_events.go,internal/modules/workbench/service/workbench/notification.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
