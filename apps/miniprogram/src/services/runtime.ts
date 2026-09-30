import '../platform/polyfills.ts';
import Taro from '@tarojs/taro';
import { createWeKnoraClient, createExecutionsApi, createServerSentEventParser, parseChatEvent, buildChatStreamRequest } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileResourceRemote } from '@weknora/api-client/mobile/resources';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createMobileRuntime } from '@weknora/mobile-core';
import type { MobileRuntime } from '@weknora/mobile-core';
import type { ChatStreamEvent } from '@weknora/contracts';
import type { ClientRequest, HttpRequest, HttpResult, NativeMultipartFileRequest } from '@weknora/api-client';
import { normalizeApiOrigin } from '../core/auth.ts';
import { storage, clearPrivateCache } from '../platform/storage.ts';
import { createWeappTransport, type WeappNetwork } from '../platform/transport.ts';
import { createAuthorizedRequestChannel, createAuthorizedStreamChannel } from '../platform/authorized-channels.ts';
import { createTaroCredentialStore, readStoredCredential } from '../platform/credential-store.ts';
import { createTaroSessionFacade, type TaroSessionFacade } from './session.ts';

// baseURL、可信来源校验、auth 存储 key 必须使用同一个 host 大小写归一化后的 origin，
// 否则同一 host 的不同大小写构建之间会话孤立，旧变体 key 的 token 会残留本机。
const origin = normalizeApiOrigin(__API_ORIGIN__);
export const network: WeappNetwork = {
  request: options => Taro.request(options as Parameters<typeof Taro.request>[0]),
  uploadFile: options => Taro.uploadFile(options as Parameters<typeof Taro.uploadFile>[0]),
};
const native = createWeappTransport(network);

/** 未鉴权 ClientRequest 通道（Runtime remoteFor 与授权 REST 通道共用；Bearer 由调用方叠加）。 */
const plainRequests = new Map<string, (input: ClientRequest) => Promise<unknown>>();
function plainRequestFor(origin: string): (input: ClientRequest) => Promise<unknown> {
  let request = plainRequests.get(origin);
  if (!request) {
    request = createWeKnoraClient({ baseURL: origin, transport: { send: (r: HttpRequest) => native.send(r) } }).request;
    plainRequests.set(origin, request);
  }
  return request;
}

/**
 * MobileRuntime 是本小程序唯一会话编排器（issue #68 AC2 replace-dont-layer）：
 * 凭据只有 Runtime 一个写者；深模块（Task Office / Resource Shelf / Task Material）
 * 的 lease 与授权通道全部由它铸造。平台 Adapter 见 platform/*。
 */
export const runtime: MobileRuntime = createMobileRuntime({
  credentialStore: createTaroCredentialStore(storage, origin),
  clientVersion: CLIENT_PROTOCOL_VERSION,
  remoteFor(deployment) { return createMobileRuntimeRemote({ origin: deployment, request: plainRequestFor(deployment) }); },
  authorizedTransport(deployment) { return createAuthorizedRequestChannel(plainRequestFor(deployment)); },
  authorizedStream(deployment) { return createAuthorizedStreamChannel(network, deployment); },
  resourceShelf: { remoteFor(deployment) { return createMobileResourceRemote({ origin: deployment, request: plainRequestFor(deployment) }); } },
});

// wx.login 薄适配：仅此一处触达 Taro 登录 API，facade 保持纯 Node 可测。
export const auth: TaroSessionFacade = createTaroSessionFacade({ runtime, origin, storage, wxCode: async () => (await Taro.login()).code });

/**
 * 平台直连通道（multipart 上传/二进制下载）取 token 的唯一入口：
 * 先走一次授权 GET（触发 Runtime 的 refresh-once 并把轮换落盘），再「用时现读」。
 * 永不缓存 token、永不再造第二条刷新路径。
 * 消费方：client 的 sendBinary/sendMultipartFile 与 platform/files.ts 的受保护下载——
 * 两条直连通道都必须经此铸造 token，不得绕行 auth.credential() 现读。
 */
export async function currentBearerToken(): Promise<string> {
  await runtime.authorizedRequest({ method: 'GET', path: '/api/v1/auth/me' });
  const stored = readStoredCredential(storage, origin);
  if (!stored) throw new Error('AUTH_REQUIRED');
  return stored.token;
}

/** 授权 JSON 通道上的应用客户端：send 走 authorizedRequest（401 刷新重放由 Runtime 负责；
 *  非 2xx 已以 ApiError 抛出，因此外壳状态为名义 200——真实语义在 Runtime 内）。 */
const authorizedTransport = {
  async send(r: HttpRequest): Promise<HttpResult> {
    const path = r.url.startsWith(origin) ? r.url.slice(origin.length) : r.url;
    const body = await runtime.authorizedRequest({ method: r.method, path, headers: r.headers, body: r.body, signal: r.signal });
    return { status: 200, headers: {}, body };
  },
  async sendBinary(r: HttpRequest): Promise<HttpResult> {
    const token = await currentBearerToken();
    return native.sendBinary({ ...r, headers: { ...r.headers, authorization: `Bearer ${token}` } });
  },
  async sendMultipartFile(r: NativeMultipartFileRequest): Promise<HttpResult> {
    const token = await currentBearerToken();
    return native.sendMultipartFile({ ...r, headers: { ...r.headers, authorization: `Bearer ${token}` } } as NativeMultipartFileRequest);
  },
};

export const client = createWeKnoraClient({ baseURL: origin, transport: authorizedTransport });
export const executions = createExecutionsApi(client.request);

export async function stream(input: HttpRequest, onText: (text: string) => void): Promise<void> {
  const path = input.url.startsWith(origin) ? input.url.slice(origin.length) : input.url;
  await runtime.authorizedEventStream({ method: input.method, path, headers: input.headers, body: input.body, signal: input.signal }, onText);
}

export async function chatStream(options: Parameters<typeof buildChatStreamRequest>[0], onEvent: (event: ChatStreamEvent) => void): Promise<void> {
  const req = buildChatStreamRequest(options), parser = createServerSentEventParser(frame => onEvent(parseChatEvent(frame)));
  await stream({ ...req, url: origin + req.path, headers: req.headers ?? {} }, text => parser.push(text));
  parser.finish();
}

export const apiOrigin = origin;

export async function logout(): Promise<void> {
  // 远端吊销尽力而为（Runtime signOut 只清本地）；网络失败不阻塞本地登出。
  try { await runtime.authorizedRequest({ method: 'POST', path: '/api/v1/auth/logout' }); } catch { /* silent: local logout always proceeds */ }
  await runtime.signOut();
  clearPrivateCache();
}

export function stopSubscriptions(): void { auth.abortSubscriptions(); }
