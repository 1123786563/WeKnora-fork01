import type { CredentialStore, StoredCredential } from '@weknora/mobile-core';
import type { ValueStore } from '../core/intent.ts';
import { normalizeApiOrigin } from '../core/auth.ts';

export const CREDENTIAL_KEY_PREFIX = 'wk:auth:';
export function credentialKeyOf(origin: string): string { return `${CREDENTIAL_KEY_PREFIX}${origin}`; }

/**
 * 用时现读（上传/下载等平台直连通道专用）：只有 Runtime 写这个键（登录/轮换/清除），
 * 读取方永远拿到最新轮换后的凭据——本函数是「token 不出 Runtime」在小程序平台通道上的
 * 忠实落地：不缓存、不刷新、不解析过期。
 * 兼容收养旧 AuthCoordinator 形态 {kind:'bearer',accessToken,refreshToken}。
 */
export function readStoredCredential(store: ValueStore, origin: string): StoredCredential | undefined {
  const value = store.read(credentialKeyOf(origin));
  if (value === null || typeof value !== 'object') return undefined;
  const record = value as Record<string, unknown>;
  const token = typeof record.token === 'string' && record.token !== ''
    ? record.token
    : typeof record.accessToken === 'string' && record.accessToken !== '' ? record.accessToken : undefined;
  const refreshToken = typeof record.refreshToken === 'string' && record.refreshToken !== '' ? record.refreshToken : undefined;
  if (token === undefined || refreshToken === undefined) return undefined;
  return { token, refreshToken };
}

/**
 * 一次性收养旧登录态（D7 语义迁移）：同 origin 的大小写变体收养首个有效 bearer 后清除；
 * 变体键一律移除，canonical 仅在为空时写入（canonical 已有凭据则只清变体、不覆盖）。
 * normalizeApiOrigin 不归一空白，故比较前先对键内 origin 折叠首尾空白（最终审查修复 F4）：
 * 尾随空白等畸形变体与大小写变体同语义——收养+移除，不再原样残留本机。
 */
export function adoptLegacyCredentials(store: ValueStore, origin: string): void {
  const canonical = credentialKeyOf(origin);
  const normalizedOrigin = normalizeApiOrigin(origin);
  for (const key of store.keys?.() ?? []) {
    if (!key.startsWith(CREDENTIAL_KEY_PREFIX) || key === canonical) continue;
    const keyOrigin = key.slice(CREDENTIAL_KEY_PREFIX.length);
    if (normalizeApiOrigin(keyOrigin.trim()) !== normalizedOrigin) continue;
    const adopted = readStoredCredential(store, keyOrigin);
    if (adopted && readStoredCredential(store, origin) === undefined) {
      store.write(canonical, { token: adopted.token, refreshToken: adopted.refreshToken });
    }
    store.remove(key);
  }
}

export function createTaroCredentialStore(store: ValueStore, origin: string): CredentialStore {
  adoptLegacyCredentials(store, origin);
  return {
    async read(deployment) { return readStoredCredential(store, deployment); },
    async write(deployment, credential) { store.write(credentialKeyOf(deployment), { token: credential.token, refreshToken: credential.refreshToken }); },
    async clear(deployment) { store.remove(credentialKeyOf(deployment)); },
  };
}
