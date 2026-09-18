// MX-024 probe · 成果签名 URL 生命周期观察器
// frozen 场景：artifact-version1 × expired-url-then-permission-revoked。
// 真实 createArtifactViewer：v1 打开授权→URL 过期→重授权→随后权限撤销——
// 全程零分享（未确认外流范围）、签名 URL 零持久化（persistentSignedUrls 恒空）且撤权后缓存清空。
import { createArtifactViewer, type ArtifactAccess, type ArtifactAccessPort } from '../../../apps/mobile/sources/weknora/resources/artifact-viewer.ts';

export interface ProbeInput {
  fixture: string;
  fault: string;
}

export interface Observation {
  shareCount: number;
  persistentSignedUrls: string[];
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'artifact-version1' || input.fault !== 'expired-url-then-permission-revoked') {
    throw new Error(`unsupported fixture/fault: ${input.fixture}/${input.fault}`);
  }
  const version = {
    artifactId: 'art-1', version: 1, kind: 'text' as const, title: '季度分析报告', sourceRunId: 'run-9', contentDigest: 'sha256:vvv',
  };
  // 授权端口脚本（只替换网络边界）：首轮授权→过期→再授权→撤权
  let calls = 0;
  const port: ArtifactAccessPort = {
    authorize: async (): Promise<ArtifactAccess> => {
      calls += 1;
      if (calls === 1) {
        return { status: 'authorized', signedUrl: 'https://files.example/sig-1', expiresAt: '2026-09-18T08:00:30Z', version };
      }
      if (calls === 2) return { status: 'expired' };
      if (calls === 3) {
        return { status: 'authorized', signedUrl: 'https://files.example/sig-2', expiresAt: '2026-09-18T08:05:00Z', version };
      }
      return { status: 'revoked' };
    },
  };
  const viewer = createArtifactViewer({ access: port });

  // 1) 首次打开（授权）
  const first = await viewer.open('art-1', 1);
  if (first.status !== 'authorized') throw new Error('precondition: first open must be authorized');
  // 2) URL 过期 → open 返回 expired，本地引用清除
  const expired = await viewer.open('art-1', 1);
  if (expired.status !== 'expired') throw new Error(`second open must observe expiry, got ${expired.status}`);
  // 3) 重新授权重开（新签名，旧的不再复用）
  const reopened = await viewer.open('art-1', 1);
  if (reopened.status !== 'authorized' || (openedUrl(reopened) === 'https://files.example/sig-1')) {
    throw new Error('re-authorization must mint a fresh signed url');
  }
  // 4) 权限撤销 → 状态 revoked 且会话缓存清空
  const revoked = await viewer.open('art-1', 1);
  if (revoked.status !== 'revoked') throw new Error('revocation must surface');
  if (viewer.sessionCacheSize() !== 0) throw new Error('revocation must clear all local references');

  // 5) 未确认外流范围 → 零分享
  const declined = await viewer.share(version, false);
  if (declined.shared) throw new Error('share without scope confirmation must not happen');

  // 6) 签名 URL 持久化通道结构性为空
  const persistentSignedUrls = viewer.persistentSignedUrls();
  if (persistentSignedUrls.length !== 0) throw new Error('signed urls must never persist');

  return { shareCount: viewer.observedShareCount(), persistentSignedUrls };
}

function openedUrl(access: ArtifactAccess): string {
  return access.status === 'authorized' ? access.signedUrl : '';
}
