/**
 * 不可变成果查看领域模型（MX-024 / M14）。
 * 冻结规则：
 * - 下载/预览前重新授权（每次访问都取新授权的不可变版本）；签名 URL 仅在单次访问内存中
 *   使用，**绝不持久化**（persistentSignedUrls 恒空）；
 * - 过期 URL→重新授权获取新版本；权限撤销→清除内容并提示（不残留旧缓存引用）；
 * - artifact 与不可变 version 关联来源（source run）；工作目录文件需先导入为 artifact；
 * - 分享前确认外流范围；缓存文件按策略清理（会话内临时，不落长期目录）。
 */

export interface ArtifactVersion {
  artifactId: string;
  version: number;
  kind: 'text' | 'image' | 'table' | 'file';
  title: string;
  sourceRunId: string;
  contentDigest: string;
}

export type ArtifactAccess =
  | { status: 'authorized'; signedUrl: string; expiresAt: string; version: ArtifactVersion }
  | { status: 'expired' }
  | { status: 'revoked' }
  | { status: 'not_found' };

export interface ArtifactAccessPort {
  /** 每次访问重新授权：返回该次访问专用的短时签名 URL（不落盘）。 */
  authorize(artifactId: string, version: number): Promise<ArtifactAccess>;
}

export interface ShareOutcome {
  shared: boolean;
  scopeDescription: string;
}

export function createArtifactViewer(ports: { access: ArtifactAccessPort }) {
  const sessionCache = new Map<string, { url: string; expiresAt: string; fetchedAt: string }>();
  let shareCount = 0;

  const key = (artifactId: string, version: number) => `${artifactId}@v${version}`;

  return {
    /**
     * 预览/下载：每次都重新授权（禁止复用会话外的旧签名）。
     * - expired → 返回 expired（调用方重新走 authorize——即再次调用本方法）；
     * - revoked/not_found → 清除该条目一切本地引用（缓存+内容）。
     */
    async open(artifactId: string, version: number): Promise<ArtifactAccess> {
      const access = await ports.access.authorize(artifactId, version);
      if (access.status === 'authorized') {
        sessionCache.set(key(artifactId, version), { url: access.signedUrl, expiresAt: access.expiresAt, fetchedAt: new Date().toISOString() });
        return access;
      }
      // 撤权/过期/不存在：清本地引用（不残留可被回填的旧签名）
      sessionCache.delete(key(artifactId, version));
      return access;
    },
    /** 会话内缓存（仅内存；无持久化通道——persistentSignedUrls 结构性为空）。 */
    persistentSignedUrls(): string[] {
      return [];
    },
    /** 系统分享：须先经用户确认外流范围；每次确认记一次。 */
    async share(version: ArtifactVersion, confirmScope: boolean): Promise<ShareOutcome> {
      if (!confirmScope) return { shared: false, scopeDescription: '用户未确认外流范围，未分享' };
      shareCount += 1;
      return {
        shared: true,
        scopeDescription: `分享「${version.title}」v${version.version}（不可变版本，来源任务 ${version.sourceRunId}）——接收方将获得该版本内容的访问授权`,
      };
    },
    /** 缓存策略清理：会话结束清空（调用方在离开页面/切空间时触发）。 */
    clearSessionCache(): void {
      sessionCache.clear();
    },
    sessionCacheSize(): number {
      return sessionCache.size;
    },
    observedShareCount(): number {
      return shareCount;
    },
  };
}

export type ArtifactViewer = ReturnType<typeof createArtifactViewer>;
