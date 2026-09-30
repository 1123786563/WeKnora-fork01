# ADR-0019：Career Desk 共享跨端业务状态

状态：为 Issue #140 架构移植采纳；实现前冻结接口。范围：此移植基线中新建 `@weknora/career-core` 及其 Web、Mini Program、Expo 消费者。

## 背景

批准的求职产品设计要求 Career Desk 统一管理跨端 `open/list/act/observe`、revision 冲突、pending intent 恢复、scope 改变和缓存失效。选定的 `issue30-sweep` BASE 中不存在 `packages/career-core`；不能把其他工作树的 fixture 误当作已存在的运行包或 API。多个客户端各自实现这些规则会使 unknown 写入、冲突和用户切换语义分叉。

## 决定

新建平台无关 `packages/career-core`，workspace 包名和导出为 `@weknora/career-core`。它只依赖 decoded contract types 和注入的端口，不执行 HTTP，也不导入 React Native、Taro、Web DOM 或 `packages/mobile-core`。职责分配如下：

- `packages/contracts` 拥有 decoded DTO；`packages/api-client` 拥有 HTTP transport 和响应解码。
- Career Desk 拥有跨端 open/list/act/observe、revision、durable pending-intent reconciliation 和 scope invalidation。
- Web、Mini Program、Mobile 负责呈现和平台适配；Mobile 将现有 scope lease 转成 CareerScope port。

首版接口在 Task 1 contract freeze 前验证并冻结：

```ts
type CareerScope = { deploymentOrigin: string; tenantId: string; actorId: string };
type CareerEnvelope<T> = { revision: string; value: T };
type CareerPage<T> = CareerEnvelope<readonly T[]> & { cursor?: string };
type CareerIntent<C> = { requestId: string; expectedRevision: string; command: C };
type CareerReceipt<T> =
  | { kind: "applied"; requestId: string; envelope: CareerEnvelope<T> }
  | { kind: "conflict"; requestId: string; envelope: CareerEnvelope<T> }
  | { kind: "unknown"; requestId: string }
  | { kind: "forbidden"; requestId: string };

interface CareerRemote<T, C> {
  open(scope: CareerScope, signal?: AbortSignal): Promise<CareerEnvelope<T>>;
  list(scope: CareerScope, cursor?: string, signal?: AbortSignal): Promise<CareerPage<T>>;
  act(scope: CareerScope, intent: CareerIntent<C>, signal?: AbortSignal): Promise<CareerReceipt<T>>;
  lookup(scope: CareerScope, requestId: string, signal?: AbortSignal): Promise<CareerReceipt<T>>;
  observe(scope: CareerScope, onRevision: (revision: string) => void): () => void;
}
interface CareerIntentStore<C> {
  save(scope: CareerScope, intent: CareerIntent<C>): Promise<void>;
  list(scope: CareerScope): Promise<readonly CareerIntent<C>[]>;
  remove(scope: CareerScope, requestId: string): Promise<void>;
}
interface CareerDesk<T, C> {
  open(scope: CareerScope): Promise<CareerEnvelope<T>>;
  list(cursor?: string): Promise<CareerPage<T>>;
  act(command: C): Promise<CareerReceipt<T>>;
  reconcilePending(): Promise<void>;
  observe(): void;
  snapshot(): CareerEnvelope<T> | undefined;
  dispose(): void;
}
```

Receipt/envelope types may be generic and adjusted for repository compileability, but the operations and invariants below are frozen. Persist the original request ID before sending a write. On scope change, abort observation, increment epoch, clear private projection/cursor, and reject late replies. Reconcile unknown outcomes only with the same request ID; conflict requires explicit rebase. `observe` is only a revision hint: refetch and decode before publishing authority. Receipts remain bound to request ID.

### Revision observer runtime contract

The runtime assembly supplies `careerObserver` in `WeKnoraClientOptions` when it has a revision-hint source. Its contract is `(scope, onRevision) => unsubscribe`: subscribe for the provided deployment/tenant/actor scope, deliver positive revision hints, and stop delivery when the returned function is called. A hint never carries authoritative Career data; Career Desk calls `open()` through CareerApi and publishes only a strictly decoded response. The current `HttpTransport` has no Career event endpoint or built-in revision stream, so the shared client does not invent a polling or SSE route. If no provider is configured, `CareerApi.observe()` throws `CareerObservationUnavailableError` immediately. Consumers must handle that explicit unsupported state by using an explicit `open()` refresh strategy or by disabling observation-dependent freshness behavior.

## Consequences

- Career-core behavioral tests run before Web/Mini/Mobile client tasks fork and cover request persistence/recovery, revision conflict, scope invalidation and late response rejection.
- Transport/authentication and DTO validation remain owned by their existing packages; no duplicate HTTP stack or trusted client tenant authority is introduced.
- The new package adds a workspace package and implementation work absent from BASE. It does not claim an existing fixture directory or package API.
- A future platform-specific adapter may wrap this seam but may not reimplement authority, revision, recovery or scope semantics.
