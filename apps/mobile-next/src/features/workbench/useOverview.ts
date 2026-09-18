// M03 工作台聚合（RW-011，decisions D-04：后端无 /workbench/overview → 客户端聚合读模型）。
// 数据全部来自真实 API；visualFixture 为视觉对照显式开关（非生产默认，不含真实业务语义）。
import { useCallback, useEffect, useRef, useState } from "react";
import { useApp } from "@/host/AppProvider";
import type { RunViewWire, InteractionWire } from "@/contracts/workbench";
import type { SessionWire } from "@/contracts/auth";

export interface OverviewData {
  counts: { running: number; waitingUser: number; completed: number };
  inProgress: Array<{ runId: string; sessionId: string; title: string; status: string; asOf: string | null }>;
  pendingInteractions: Array<{ id: string; runId: string; kind: string; title: string }>;
  recentSessions: SessionWire[];
  asOf: string;
}

export type OverviewState =
  | { kind: "loading" }
  | { kind: "ready"; data: OverviewData; stale: boolean }
  | { kind: "error"; message: string; retry: () => void }
  | { kind: "forbidden" }
  | { kind: "offline" };

const VISUAL_FIXTURE: OverviewData = {
  counts: { running: 2, waitingUser: 1, completed: 8 },
  inProgress: [
    { runId: "run_demo_1", sessionId: "s1", title: "整理产品反馈", status: "running", asOf: null },
  ],
  pendingInteractions: [
    { id: "it_demo_1", runId: "run_demo_1", kind: "tool_approval", title: "发布客户反馈摘要" },
  ],
  recentSessions: [],
  asOf: new Date().toISOString(),
};

export function useOverview(): OverviewState & { refresh: () => void } {
  const app = useApp();
  const [state, setState] = useState<OverviewState>({ kind: "loading" });
  const genRef = useRef<number>(0);

  const load = useCallback(async () => {
    if (app.visualFixture) {
      setState({ kind: "ready", data: VISUAL_FIXTURE, stale: false });
      return;
    }
    const gen = app.scope.generation?.value ?? 0;
    genRef.current = gen;
    if (state.kind !== "loading") setState({ kind: "loading" });
    try {
      // 客户端聚合：会话列表 + 未决交互并发读取（单 scope；cursor/as_of 由响应携带）
      const [sessions, interactions] = await Promise.all([
        app.api.sessions({ page: 1, page_size: 20 }),
        loadPendingInteractions(app),
      ]);
      // 迟到结果守卫：generation 变化丢弃
      const currentGen = app.scope.generation?.value ?? 0;
      if (genRef.current !== currentGen || gen !== currentGen) return;
      const data: OverviewData = {
        counts: {
          running: sessions.sessions.filter((s) => !!s.agent_id).length,
          waitingUser: interactions.length,
          completed: 0,
        },
        inProgress: interactions.map((i) => ({
          runId: i.run_id,
          sessionId: "",
          title: i.title,
          status: "waiting_user",
          asOf: i.created_at,
        })),
        pendingInteractions: interactions.map((i) => ({ id: i.id, runId: i.run_id, kind: i.kind, title: i.title })),
        recentSessions: sessions.sessions.slice(0, 5),
        asOf: new Date().toISOString(),
      };
      setState({ kind: "ready", data, stale: false });
    } catch (e) {
      const err = e as { kind?: string; message?: string };
      if (err.kind === "forbidden") {
        setState({ kind: "forbidden" });
        return;
      }
      if (err.kind === "network" || err.kind === "aborted") {
        setState({ kind: "offline" });
        return;
      }
      setState({ kind: "error", message: err.message ?? "暂时无法连接", retry: load });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [app.visualFixture, app.scope.generation?.value]);

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [load]);

  return { ...(state as OverviewState), refresh: () => void load() };
}

async function loadPendingInteractions(app: ReturnType<typeof useApp>): Promise<InteractionWire[]> {
  // 后端无全局 pending 端点：从最近会话的 run 交互聚合（D-04/D-06 差异记录）
  try {
    const sessions = await app.api.sessions({ page: 1, page_size: 10 });
    const all: InteractionWire[] = [];
    for (const s of sessions.sessions.slice(0, 5)) {
      if (!s.id) continue;
      try {
        const list = await app.api.interactions(s.id);
        all.push(...list.filter((i) => i.status === "pending"));
      } catch {
        // 单会话交互读取失败不阻塞整体聚合
      }
    }
    return all;
  } catch {
    return [];
  }
}

export type { RunViewWire };
