// 组合根（composition root）：装配 platform + api + domain，向 UI 提供 commands。
// 页面只消费 context；不直接导入后端 token 或 fetch（02 规格各页"组件与接线"要求）。
// 注意：本文件不得放在 src/app/ 下（expo-router 会将 src/app 作为路由根扫描）。
import React, { createContext, useContext, useEffect, useMemo, useRef, useState } from "react";
import * as Linking from "expo-linking";
import { HttpClient, setAllowLocalHttpForDev } from "@/api/http";
import { WeKnoraApi } from "@/api/weknora";
import { ScopeCoordinator, scopeCacheKey, type ScopeKey } from "@/domain/scope";
import { InMemoryStore, type MobileStore } from "@/platform/store";
import { SqliteStore, secureCreds } from "@/platform/native";
import { AuthController, type AppIdentity, type AuthStage } from "@/features/auth/AuthController";
import { SubmissionService } from "@/features/workbench/submit/SubmissionService";
import { ChatService } from "@/features/conversations/chat/ChatService";
import { AttachmentUploader } from "@/features/workbench/attachments/AttachmentUploader";
import { handleDeepLink } from "@/features/notifications/deeplink/DeepLinkController";

export interface AppHostConfig {
  /** 默认受信服务器（用户可在 M01 覆盖） */
  defaultOrigin: string;
  /** dev 模式：允许 http/本地地址（expo go 本地联调） */
  allowLocalHttp?: boolean;
  /** 视觉对照/演示数据显式开关（EXPO_PUBLIC_VISUAL_FIXTURE=1，dev only；fixture 数字仅隔离测试用） */
  visualFixture?: boolean;
  /** 测试注入：内存 store + 原生凭证替身 */
  __testOverrides?: {
    store?: MobileStore;
    credentials?: typeof secureCreds;
  };
}

export interface AppHost {
  stage: AuthStage;
  identity: AppIdentity | null;
  scope: ScopeCoordinator;
  api: WeKnoraApi;
  store: MobileStore;
  auth: AuthController;
  submissions: SubmissionService;
  origin: string;
  setOrigin(origin: string): Promise<void>;
  refreshIdentity(): Promise<void>;
  switchSpace(tenantId: string): Promise<void>;
  logout(): Promise<void>;
  scopeKey(): string;
  /** 聊天发送与流式消费（M07） */
  chat: ChatService;
  /** 附件上传器工厂（M05 每次编辑会话一个实例，items 有状态） */
  makeAttachmentUploader(): AttachmentUploader;
  /** 视觉验证/演示 fixture 显式开关（非生产默认） */
  visualFixture: boolean;
  setVisualFixture(on: boolean): void;
}

const AppContext = createContext<AppHost | null>(null);

/** expo-router 惰性注入（jest 环境 AppProvider 不渲染，避免模块副作用） */
const routerPush = (pathname: string, params: Record<string, string>): void => {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const router = require("expo-router").router as { push: (h: { pathname: string; params?: Record<string, string> }) => void };
  router.push({ pathname, params });
};

export function AppProvider({ children, config }: { children: React.ReactNode; config: AppHostConfig }) {
  const [stage, setStage] = useState<AuthStage>({ kind: "booting" });
  const [identity, setIdentity] = useState<AppIdentity | null>(null);
  const [origin, setOriginState] = useState(config.defaultOrigin);
  const [visualFixture, setVisualFixtureState] = useState(config.visualFixture ?? false);
  const [reloadKey, setReloadKey] = useState(0);

  if (config.allowLocalHttp) setAllowLocalHttpForDev(true);

  const hostRef = useRef<AppHost | null>(null);
  if (!hostRef.current) {
    const store = config.__testOverrides?.store ?? new SqliteStore();
    const credentials = config.__testOverrides?.credentials ?? secureCreds;
    const scope = new ScopeCoordinator();
    // token 缓存：ChatService/上传等同步 headers 场景使用；每次凭证写入后刷新
    let cachedToken = "";
    const http = new HttpClient({
      getOrigin: () => hostRef.current?.origin ?? config.defaultOrigin,
      getTenantId: () => scope.scope?.tenantId ?? null,
      credentials: {
        read: async () => {
          const c = await credentials.read();
          cachedToken = c?.access ?? "";
          return c ? { access: c.access, refresh: c.refresh } : null;
        },
        write: async (v) => {
          const c = await credentials.read();
          cachedToken = v.access;
          await credentials.write({
            origin: hostRef.current?.origin ?? config.defaultOrigin,
            access: v.access,
            refresh: v.refresh,
            userId: c?.userId ?? "",
            tenantId: c?.tenantId ?? null,
          });
        },
        clear: () => {
          cachedToken = "";
          return credentials.clear();
        },
      },
      onAuthExpired: () => setStage({ kind: "login" }),
    });
    const api = new WeKnoraApi(http);
    const scopeKey = () => {
      const s = scope.scope;
      return s ? scopeCacheKey(s) : "__no_scope__";
    };
    const auth = new AuthController({
      api,
      scope,
      store,
      credentials,
      getOrigin: () => hostRef.current?.origin ?? config.defaultOrigin,
      validateOrigin: (o) => {
        if (!/^https?:\/\//.test(o)) throw new Error("服务器地址无效");
        if (!o.startsWith("https://") && !config.allowLocalHttp) throw new Error("生产环境仅支持 https 服务器");
      },
      onStage: (s) => setStage(s),
      onIdentity: (i) => setIdentity(i),
    });
    const submissions = new SubmissionService({
      api,
      store,
      scopeKey,
      canWrite: () => scope.canWrite,
      newRequestId: () => `req_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 10)}`,
      lookup: async (requestId) => {
        try {
          const r = await api.lookupRequest(requestId);
          return { runId: r?.run_id ?? null, status: r?.status ?? null };
        } catch {
          return { runId: null, status: null };
        }
      },
    });
    const chat = new ChatService({
      buildUrl: (sessionId) => `${hostRef.current?.origin ?? config.defaultOrigin}/api/v1/agent-chat/${encodeURIComponent(sessionId)}`,
      headers: () => {
        const h: Record<string, string> = {};
        if (cachedToken) h.Authorization = `Bearer ${cachedToken}`;
        const tenant = scope.scope?.tenantId;
        if (tenant) h["X-Tenant-ID"] = tenant;
        return h;
      },
    });
    const makeAttachmentUploader = () =>
      new AttachmentUploader({
        transport: {
          upload: (sessionId, file, agentId, signal) => api.uploadAttachment(sessionId, file, agentId, signal),
          get: (sessionId, attachmentId, signal) => api.getAttachment(sessionId, attachmentId, signal),
        },
      });
    hostRef.current = {
      stage,
      identity,
      scope,
      api,
      store,
      auth,
      submissions,
      origin: config.defaultOrigin,
      setOrigin: async (o) => {
        setOriginState(o);
        if (hostRef.current) hostRef.current.origin = o;
        setReloadKey((k) => k + 1);
      },
      refreshIdentity: async () => {
        await auth.refreshMemberships();
      },
      switchSpace: async (tenantId) => {
        await auth.switchSpace(tenantId);
      },
      logout: async () => {
        await auth.logout();
        setReloadKey((k) => k + 1);
      },
      scopeKey,
      chat,
      makeAttachmentUploader,
      visualFixture: false,
      setVisualFixture: (on: boolean) => {
        setVisualFixtureState(on);
        if (hostRef.current) hostRef.current.visualFixture = on;
      },
    };
  }
  const host = hostRef.current;
  host.stage = stage;
  host.identity = identity;
  host.origin = origin;
  host.visualFixture = visualFixture;

  useEffect(() => {
    // SqliteStore.open 在原生环境异步初始化；内存实现跳过
    const storeAny = host.store as unknown as { open?: () => Promise<void> };
    storeAny.open?.().catch(() => {});
    void host.auth.bootstrap();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [reloadKey]);

  // RW-028 深链接线：通知/外链 → 解析（危险参数剥离）→ 认证检查 → 导航。
  // defer_to_login：登录流程完成后由 gate 落位（深链不跳过授权）。
  useEffect(() => {
    const applyUrl = (url: string) => {
      const decision = handleDeepLink(url, {
        stage: stage.kind,
        currentTenantId: host.scope.scope?.tenantId ?? null,
        resourceTenantId: null, // 资源归属空间未知：页面内重新授权（详细设计 §9）
      });
      if (decision.kind === "navigate") {
        routerPush(decision.pathname, decision.routeParams);
      } else if (decision.kind === "defer_to_login") {
        routerPush("/login", {});
      } else if (decision.kind === "pick_space_first") {
        routerPush("/spaces", {});
      }
    };
    const sub = Linking.addEventListener("url", ({ url }) => applyUrl(url));
    const initial = Linking.getInitialURL();
    if (initial && typeof initial.then === "function") {
      initial.then((u) => {
        if (u && stage.kind !== "ready") return; // 未认证冷启动深链：gate 处理认证后落位首页
        if (u) applyUrl(u);
      });
    }
    return () => sub.remove();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stage.kind, reloadKey]);

  const value = useMemo<AppHost>(() => ({ ...host }), [host, stage, identity, origin, visualFixture]);
  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}

export function useApp(): AppHost {
  const ctx = useContext(AppContext);
  if (!ctx) throw new Error("useApp 必须在 AppProvider 内使用");
  return ctx;
}

export type { ScopeKey };
export { InMemoryStore };
