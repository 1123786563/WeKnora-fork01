export interface ForegroundSyncLifecycle {
  subscribe(listener: (state: 'active' | 'background') => void): () => void;
}

export interface ForegroundSyncPorts {
  lifecycle: ForegroundSyncLifecycle;
  /** 每次前台事件时解析当前授权 scope 的权威同步动作；未授权/无 scope 返回 undefined（跳过）。 */
  resolveSync(): (() => Promise<void>) | undefined;
}

export interface ForegroundSyncLoop {
  start(): () => void;
}

/**
 * 前台权威同步环（#67 AC2）：推送网关关闭/无推送凭据时，回到前台即向权威服务端重新
 * 拉取（通知 Inbox page() 权威重投影——module-seams §5.3：推送只触发重新同步，权威
 * 状态永远在服务端）。
 * - 单飞合并：同步进行中收到新的 active 事件不并发，只记一次待执行；
 * - 失败包含：同步失败只吞掉（下次前台再试），绝不 unhandled rejection；
 * - scope 每次事件重新解析：切租户/换部署/登出后的旧闭包不再被引用。
 */
export function createForegroundSyncLoop(ports: ForegroundSyncPorts): ForegroundSyncLoop {
  let running = false;
  let followUpPending = false;
  let started = false;
  const runOnce = async (): Promise<void> => {
    // running 必须在同步段置位：连续 active 事件在首个微任务前就能被合并为 follow-up。
    running = true;
    try {
      // resolveSync/sync 落到微任务之后：前台事件同步发布方（AppState）不被阻塞，
      // 事件返回后才就绪的闭包（如同步动作的解析上下文）在此时已生效。
      await Promise.resolve();
      const sync = ports.resolveSync();
      if (sync !== undefined) await sync();
    } catch {
      // 失败包含：下一次前台事件重试
    } finally {
      running = false;
      if (followUpPending) {
        followUpPending = false;
        void runOnce();
      }
    }
  };
  return {
    start() {
      if (started) return () => undefined;
      started = true;
      const unsubscribe = ports.lifecycle.subscribe((state) => {
        if (state !== 'active') return;
        if (running) {
          followUpPending = true;
          return;
        }
        void runOnce();
      });
      return () => { unsubscribe(); };
    },
  };
}
