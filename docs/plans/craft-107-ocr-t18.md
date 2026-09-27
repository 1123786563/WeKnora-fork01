Review complete: 3 finding(s) across 1 selected item(s).

─── internal/application/repository/agent_run_events.go:79-81 ───
[bug · medium] 缺口规则只校验了返回页的第一行（rows[0].Seq > after+1），页内中部的洞仍然会静默跳投：例如保留窗口为 {1,2,3,5,6}、客户端 after=2
时，本页返回 [3,5,6]，rows[0].Seq==3 通过检查，seq 4 缺失却被当作正常事件序列下发。两个真实消费者都会把游标推进到页内最大 seq（agent_run.go 的 emit
将 after 置为 latest；workbench/notification.go 的投影器将 last 持久化为最大 seq），因此该缺口在后续分页中永远无法被追上——这正是 T18
要求必须返回 409 让客户端重载权威快照的场景，也与注释自身声明的威胁模型（"any store repair that removed a middle
event"）不符。建议在转换循环中校验整页连续性：AppendEvent 在 fence 锁下按 last.Seq+1 分配，正常运行时 seq 必然连续，全页连续性检查不会误报，仅
trim/repair 产生的洞会触发。

- 	if len(rows) > 0 && rows[0].Seq > after+1 {
+ 	out := make([]agentruntime.RunEvent, 0, len(rows))
+ 	prev := after
+ 	for _, r := range rows {
+ 		if r.Seq != prev+1 {
- 		return nil, agentruntime.ErrCursorExpired
+ 			return nil, agentruntime.ErrCursorExpired
+ 		}
+ 		prev = r.Seq
+ 		out = append(out, agentruntime.RunEvent{Seq: r.Seq, AttemptID: r.AttemptID, Type: r.EventType, Payload: json.RawMessage(r.Payload)})
  	}


─── internal/application/repository/agent_run_events.go:79-81 ───
[maintainability · low] 新增的页首行检查在语义上完全覆盖了上方第 63-66 行的 head 预检查：若 first.Seq > after+1，则 seq>?
查询返回的首行必然 ≥ first.Seq，同样 > after+1；若 first.Seq ≤ after+1，head 检查放行而由新检查裁决。ReadEvents 位于 SSE 每 250ms
一次的轮询路径（GetAgentRunEvents），每次调用都多付出一条 Take 查询。可考虑移除 head 预检查（或仅在返回空页时保留），减少每轮询周期的数据库往返；属非阻塞优化建议。



─── internal/application/repository/agent_run_events.go:79-81 ───
[bug · medium] 新增的页首缺口检查对持久化 checkpoint 消费者缺少恢复路径，且会引入队头阻塞：notification
projector（internal/modules/workbench/service/workbench/notification.go:61-64 ProjectAndCheckpoint）收到
ErrCursorExpired 时不推进 checkpoint 直接返回错误，而
NotificationWorker.RunOnce（notification_worker.go:47-49）遇到第一个错误即中止整页遍历，且 EventRunKeysPage 按
tenant_id/run_id 稳定字典序返回——结果是一旦某个 run 的保留窗口在 checkpoint 前方出现中部空洞（正是 trimRunEventsBeforeCutoff 保留
usage.*/approval.* 中部事件、删除其后事件的可复现场景），该 run 每 2 秒以相同 cursor 重复失败，字典序在其后的所有 run 的通知投影被永久饿死。SSE 客户端可凭
409 重载权威快照自愈，但该 checkpoint 消费者没有快照重载通道；且本变更前该场景是"跳过缺口并推进"（丢通知但自愈），变更后变为永久卡死。建议：在错误中携带可恢复的续读位置（如首个保留
seq），让 checkpoint 消费者能跳投推进而非无限重试，或同步为 projector/worker 增加 ErrCursorExpired 的处理（跳到保留窗口头部/trimmed 水位）。

  	if len(rows) > 0 && rows[0].Seq > after+1 {
- 		return nil, agentruntime.ErrCursorExpired
+ 		// SSE clients reload the snapshot on this sentinel, but checkpoint
+ 		// consumers (notification projector) cannot: expose the resumable
+ 		// position so they can advance past the gap instead of retrying the
+ 		// same expired cursor forever.
+ 		return nil, fmt.Errorf("%w: first_retained_seq=%d", agentruntime.ErrCursorExpired, rows[0].Seq)
  	}

