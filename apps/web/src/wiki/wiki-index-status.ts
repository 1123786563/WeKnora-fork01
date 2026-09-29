import { useEffect, useRef, useState } from "react";
import type { WeKnoraClient } from "@weknora/api-client";

/**
 * KBW-4 — Wiki 索引状态轮询（对齐 Vue WikiBrowser.vue loadStats L2856-2892 与
 * KnowledgeBase.vue L119-176 的父层口径）：
 *   - 挂载时拉一次 GET /wiki/stats；
 *   - 仅当 is_active || pending_tasks > 0（索引中）才起 5s setInterval，
 *     回到空闲即停（Vue：「活跃时轮询，空闲时停掉定时器，避免无谓请求」）；
 *   - 从「轮询中 → 空闲」的完成沿触发 onIndexingSettled（Vue 完成分支重载
 *     页面列表/当前页/图谱）。
 * 请求走 client.request 原始通道而非 client.wiki.stats：api-client 的 stats()
 * 只保留 pages_by_type/pending_issues，丢掉了轮询判定所需的 pending_tasks /
 * is_active（packages/api-client 不在本修复的可触碰清单内）。
 */
export const WIKI_INDEX_STATUS_POLL_INTERVAL_MS = 5000;

export interface WikiIndexStats {
  pendingTasks: number;
  isActive: boolean;
  pendingIssues: number;
  totalPages: number;
  pagesByType: Record<string, number>;
}

export function wikiStatsPath(knowledgeBaseId: string): string {
  return `/api/v1/knowledgebase/${encodeURIComponent(knowledgeBaseId)}/wiki/stats`;
}

export function parseWikiIndexStats(row: unknown): WikiIndexStats {
  const pagesByType: Record<string, number> = {};
  if (typeof row === "object" && row !== null) {
    const record = row as Record<string, unknown>;
    if (typeof record.pages_by_type === "object" && record.pages_by_type !== null && !Array.isArray(record.pages_by_type)) {
      for (const [key, value] of Object.entries(record.pages_by_type as Record<string, unknown>)) {
        pagesByType[key] = typeof value === "number" ? value : 0;
      }
    }
  }
  if (typeof row !== "object" || row === null) {
    return { pendingTasks: 0, isActive: false, pendingIssues: 0, totalPages: 0, pagesByType };
  }
  const record = row as Record<string, unknown>;
  return {
    pendingTasks: typeof record.pending_tasks === "number" ? record.pending_tasks : 0,
    isActive: record.is_active === true,
    pendingIssues: typeof record.pending_issues === "number" ? record.pending_issues : 0,
    totalPages: typeof record.total_pages === "number" ? record.total_pages : 0,
    pagesByType,
  };
}

export interface WikiIndexStatusState {
  /** 最新一次 stats 响应；null = 尚未完成首次拉取（或请求失败——Vue 同口径静默）。 */
  stats: WikiIndexStats | null;
  /** Vue wikiIsIndexing（KnowledgeBase.vue L104）：isActive || pendingTasks > 0。 */
  indexing: boolean;
}

export function useWikiIndexStatus(
  client: WeKnoraClient,
  knowledgeBaseId: string,
  options?: { onIndexingSettled?: () => void },
): WikiIndexStatusState {
  const [stats, setStats] = useState<WikiIndexStats | null>(null);
  // ReturnType<typeof setInterval> 适配宿主全局（浏览器 number / Node Timeout），
  // 保持裸调全局 setInterval——测试用 node:test mock.timers 打桩全局计时器。
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  // Vue 以 statsTimer 是否存在判定「刚完成」；这里镜像为「见过索引中状态」。
  const sawIndexingRef = useRef(false);
  // 回调走 ref：调用方传内联闭包（引用 loadPages/load 等每渲染新建的函数）
  // 不会重启轮询 effect。
  const settledRef = useRef(options?.onIndexingSettled);
  settledRef.current = options?.onIndexingSettled;

  useEffect(() => {
    let active = true;
    sawIndexingRef.current = false;
    setStats(null);
    const stop = () => {
      if (timerRef.current !== null) {
        clearInterval(timerRef.current);
        timerRef.current = null;
      }
    };
    const load = async () => {
      try {
        const row = await client.request({ method: "GET", path: wikiStatsPath(knowledgeBaseId) });
        if (!active) return;
        const next = parseWikiIndexStats(row);
        setStats(next);
        if (next.isActive || next.pendingTasks > 0) {
          sawIndexingRef.current = true;
          if (timerRef.current === null) {
            timerRef.current = setInterval(() => { void load(); }, WIKI_INDEX_STATUS_POLL_INTERVAL_MS);
          }
        } else if (sawIndexingRef.current) {
          // Vue 完成分支（WikiBrowser.vue L2879-2890）：清定时器 + 重载内容。
          sawIndexingRef.current = false;
          stop();
          settledRef.current?.();
        }
      } catch {
        /* ignore — Vue loadStats 同口径 */
      }
    };
    void load();
    return () => {
      active = false;
      stop();
    };
  }, [client, knowledgeBaseId]);

  return { stats, indexing: stats !== null && (stats.isActive || stats.pendingTasks > 0) };
}
