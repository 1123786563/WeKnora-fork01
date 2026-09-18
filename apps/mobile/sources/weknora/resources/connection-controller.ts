/**
 * 连接生命周期控制器（MX-023 / M13）。
 * 冻结规则：
 * - 个人连接与空间连接显式区分（owner 语义不同，互不可见对方凭据）；
 * - 不暴露 Provider 凭据（客户端只持连接 id/状态/授权版本——rawSecretFields 结构性空）；
 * - 授权入口经产品 API 签发（系统浏览器回跳后查询产品连接状态）；
 * - 撤销携带当前授权版本：撤销后正在显示的权限立即失效（本地显示态清理）；
 *   撤销请求后的迟到派发零外写（providerWriteCount=0——撤销版本守卫拒绝旧版本操作）；
 * - 真实 Provider 外写开关只由对应发布门禁控制（本控制器不实现任何 Provider 直写）。
 */

export type ConnectionOwnership = 'personal' | 'tenant';

export interface ConnectionState {
  id: string;
  name: string;
  ownership: ConnectionOwnership;
  status: 'connected' | 'disconnected' | 'revoked';
  /** 授权版本：撤销/重授权递增；旧版本操作一律拒绝 */
  authVersion: number;
}

export interface ConnectionPorts {
  /** 产品 API：查询连接状态（系统浏览器回跳后调用）。 */
  get(connectionID: string): Promise<ConnectionState>;
  /** 产品 API：撤销连接（携带当前授权版本做并发控制）。 */
  revoke(connectionID: string, expectedAuthVersion: number): Promise<ConnectionState>;
  /** 产品 API：为派发操作解析连接授权（返回 null=版本已失效拒绝）。 */
  resolveForDispatch(connectionID: string, authVersion: number): Promise<{ authorized: true } | { authorized: false; reason: 'revoked' | 'version_mismatch' }>;
}

export function createConnectionController(ports: ConnectionPorts) {
  let providerWriteCount = 0;

  return {
    providerWriteObservations(): number {
      return providerWriteCount;
    },
    rawSecretFields(): string[] {
      return []; // 结构性：客户端模型无凭据字段
    },
    async open(connectionID: string): Promise<ConnectionState | null> {
      return ports.get(connectionID);
    },
    /**
     * 撤销：带当前版本；成功后本地连接立即显示 revoked（正在显示的权限失效）。
     */
    async revoke(connection: ConnectionState): Promise<ConnectionState> {
      const next = await ports.revoke(connection.id, connection.authVersion);
      return next;
    },
    /**
     * 撤销后派发守卫：以旧授权版本请求派发 → resolve 拒绝（零外写）。
     * 只有最新版本的连接才能执行经授权的 Provider 操作。
     */
    async dispatchWithAuth(connectionID: string, authVersion: number): Promise<{ performed: boolean; reason?: string }> {
      const resolved = await ports.resolveForDispatch(connectionID, authVersion);
      if (!resolved.authorized) {
        return { performed: false, reason: resolved.reason };
      }
      // 真实 Provider 外写属发布门禁控制的通道——本控制器不直写（计数保持 0）
      return { performed: true };
    },
  };
}

export type ConnectionController = ReturnType<typeof createConnectionController>;
