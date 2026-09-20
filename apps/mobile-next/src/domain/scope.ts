import type { CloudWorkspaceScope } from "@/cloud-workspace/CloudWorkspaceClient";

// Scope 隔离（详细设计 §3.2）：
// ScopeKey = canonicalOrigin + userId + tenantId；generation 单调递增；
// 切空间序：禁写 → generation+1 → abort 旧订阅 → 清可见数据 → 校验新空间 → 加载新数据。
// 迟到结果按 generation 丢弃，不进入新空间。

export interface ScopeKey {
  origin: string;
  userId: string;
  tenantId: string;
}

export function toCloudWorkspaceScope(scope: ScopeKey, generation: number): CloudWorkspaceScope {
  return {
    backend: scope.origin,
    accountId: scope.userId,
    tenantId: scope.tenantId,
    generation,
  };
}

export function sameScope(a: ScopeKey, b: ScopeKey): boolean {
  return a.origin === b.origin && a.userId === b.userId && a.tenantId === b.tenantId;
}

export function scopeCacheKey(s: ScopeKey): string {
  // 确定性元组序列化（组件分隔符转义），不用无转义拼接
  const esc = (v: string) => v.replace(/\\/g, "\\\\").replace(/\u0001/g, "\\s");
  return [esc(s.origin), esc(s.userId), esc(s.tenantId)].join("\u0001");
}

export interface Generation {
  value: number;
  scope: ScopeKey;
}

export class ScopeCoordinator {
  private gen = 0;
  private current: ScopeKey | null = null;
  private writesEnabled = true;
  private abortController: AbortController | null = null;
  private listeners = new Set<(g: Generation) => void>();

  get scope(): ScopeKey | null {
    return this.current;
  }

  get generation(): Generation | null {
    return this.current ? { value: this.gen, scope: this.current } : null;
  }

  get signal(): AbortSignal | null {
    return this.abortController?.signal ?? null;
  }

  get canWrite(): boolean {
    return this.writesEnabled;
  }

  onGenerationChange(fn: (g: Generation) => void): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  /** 迟到结果守卫：返回 true 表示结果属于当前 generation，可以进入 UI */
  accepts(g: Generation | null): boolean {
    if (!this.current || !g) return false;
    return g.value === this.gen && sameScope(g.scope, this.current);
  }

  /** 初始进入一个 scope（登录/冷启动后） */
  enter(scope: ScopeKey): Generation {
    this.current = scope;
    this.gen += 1;
    this.abortController = new AbortController();
    this.writesEnabled = true;
    const g = { value: this.gen, scope };
    for (const fn of this.listeners) fn(g);
    return g;
  }

  /** 切换空间：按设计顺序推进；返回新 generation。业务 Run 在服务端保持存活。 */
  switchTenant(tenantId: string): Generation {
    if (!this.current) throw new Error("切换空间前必须先 enter");
    // 1) 禁写
    this.writesEnabled = false;
    // 2) 推进 generation
    this.gen += 1;
    const scope: ScopeKey = { ...this.current, tenantId };
    this.current = scope;
    // 3) 关闭旧订阅（abort 所有旧请求与流）
    this.abortController?.abort();
    this.abortController = new AbortController();
    // 4) 清敏感可见数据由上层 store 执行（onGenerationChange 监听）
    // 5) 校验新空间 + 6) 加载新数据由上层执行；校验通过后 reEnableWrites
    const g = { value: this.gen, scope };
    for (const fn of this.listeners) fn(g);
    return g;
  }

  /** 新空间校验通过后恢复写入 */
  reEnableWrites(): void {
    this.writesEnabled = true;
  }

  /** 退出登录：清 scope，abort 全部 */
  reset(): void {
    this.writesEnabled = false;
    this.abortController?.abort();
    this.abortController = null;
    this.current = null;
  }
}
