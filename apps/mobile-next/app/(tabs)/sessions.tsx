import React, { useCallback, useEffect, useRef, useState } from "react";
import { FlatList, RefreshControl, StyleSheet, Text, View } from "react-native";
import { router } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { FilterChips, type FilterChip } from "@/components/FilterChips";
import { SearchField } from "@/components/SearchField";
import { StateView } from "@/components/StateView";
import { TaskCard } from "@/components/TaskCard";
import type { BadgeSpec } from "@/components/StatusBadge";
import type { SessionWire } from "@/contracts/auth";
import { formatRelative, runStatusLabel, runStatusTone } from "@/features/workbench/labels";

// M04 会话列表（02 规格）：搜索防抖 300ms、状态筛选、分页加载、下拉刷新。
// 说明（D-06）：session wire 无状态字段——进行中=已绑定 agent、待处理=存在 pending 交互（客户端聚合）、
// 已完成=其余会话；与 useOverview 的客户端聚合口径一致。

const PAGE_SIZE = 20;
const SEARCH_DEBOUNCE_MS = 300;

const FILTER_CHIPS: FilterChip[] = [
  { key: "all", label: "全部" },
  { key: "running", label: "进行中" },
  { key: "waiting", label: "待处理" },
  { key: "done", label: "已完成" },
];

type DerivedStatus = "running" | "waiting" | "done";

export default function SessionsScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();

  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [filter, setFilter] = useState("all");
  const [sessions, setSessions] = useState<SessionWire[]>([]);
  const [pendingIds, setPendingIds] = useState<Set<string>>(() => new Set());
  const [agentNames, setAgentNames] = useState<Record<string, string>>({});
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(true);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [errorKind, setErrorKind] = useState<null | "forbidden" | "error">(null);

  const busyRef = useRef(false);
  const genRef = useRef<number>(-1);

  // 300ms 防抖：输入停顿后才触发请求（useEffect + setTimeout）
  useEffect(() => {
    const t = setTimeout(() => setKeyword(keywordInput.trim()), SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [keywordInput]);

  const fetchSessions = useCallback(
    async (nextPage: number, kw: string, mode: "reset" | "more" | "refresh") => {
      if (busyRef.current) return;
      busyRef.current = true;
      const gen = app.scope.generation?.value ?? 0;
      genRef.current = gen;
      if (mode === "reset") {
        setLoading(true);
        setErrorKind(null);
      } else if (mode === "refresh") {
        setRefreshing(true);
      } else {
        setLoadingMore(true);
      }
      try {
        const res = await app.api.sessions({
          page: nextPage,
          page_size: PAGE_SIZE,
          keyword: kw.length > 0 ? kw : undefined,
        });
        // 迟到结果守卫：generation 变化丢弃，不进入新空间
        const currentGen = app.scope.generation?.value ?? 0;
        if (genRef.current !== currentGen || gen !== currentGen) return;
        setSessions((prev) => {
          if (mode !== "more") return res.sessions;
          const seen = new Set(prev.map((s) => s.id));
          return [...prev, ...res.sessions.filter((s) => !seen.has(s.id))];
        });
        setHasMore(res.sessions.length >= PAGE_SIZE);
        setPage(nextPage);
        setErrorKind(null);
      } catch (e) {
        const err = e as { kind?: string };
        if (err.kind === "forbidden") setErrorKind("forbidden");
        else setErrorKind("error");
        if (mode !== "more") setSessions([]);
      } finally {
        busyRef.current = false;
        setLoading(false);
        setLoadingMore(false);
        setRefreshing(false);
      }
    },
    [app],
  );

  // 待处理聚合：后端无全局 pending 端点，从最近会话的交互聚合（同 useOverview，D-04/D-06）
  const loadPending = useCallback(async () => {
    try {
      const res = await app.api.sessions({ page: 1, page_size: 10 });
      const ids = new Set<string>();
      for (const s of res.sessions.slice(0, 5)) {
        if (!s.id) continue;
        try {
          const list = await app.api.interactions(s.id);
          for (const i of list) {
            if (i.status === "pending") ids.add(s.id);
          }
        } catch {
          // 单会话交互读取失败不阻塞整体聚合
        }
      }
      setPendingIds(ids);
    } catch {
      setPendingIds(new Set());
    }
  }, [app]);

  // Agent 名映射（尽力而为：目录读取失败时回退到通用称呼）
  useEffect(() => {
    let alive = true;
    void app.api
      .agents()
      .then((list) => {
        if (!alive) return;
        const map: Record<string, string> = {};
        for (const a of list) map[a.id] = a.name;
        setAgentNames(map);
      })
      .catch(() => {
        // 目录不可用不阻塞会话列表
      });
    return () => {
      alive = false;
    };
  }, [app]);

  // 关键词变化 → 回到第一页重新加载
  useEffect(() => {
    void fetchSessions(1, keyword, "reset");
  }, [fetchSessions, keyword]);

  const deriveStatus = (s: SessionWire): DerivedStatus => {
    if (pendingIds.has(s.id)) return "waiting";
    return s.agent_id ? "running" : "done";
  };

  const badgeFor = (st: DerivedStatus): BadgeSpec => {
    if (st === "waiting") return { domain: "run", label: "待处理", tone: "attention" };
    if (st === "done") return { domain: "run", label: "已完成", tone: "success" };
    return { domain: "run", label: runStatusLabel("running"), tone: runStatusTone("running") };
  };

  const filtered = sessions.filter((s) => {
    if (filter === "all") return true;
    return deriveStatus(s) === filter;
  });

  const styles = StyleSheet.create({
    wrap: { flex: 1, backgroundColor: theme.c.bg },
    header: {
      paddingHorizontal: theme.space[20],
      paddingTop: insets.top + theme.space[12],
      paddingBottom: theme.space[12],
      gap: theme.space[12],
    },
    title: {
      fontSize: theme.type.title.fontSize,
      lineHeight: theme.type.title.lineHeight,
      fontWeight: theme.type.title.fontWeight as "700",
      color: theme.c.ink,
    },
    list: { paddingHorizontal: theme.space[20], paddingBottom: theme.space[32] },
    separator: { height: theme.space[12] },
    footer: { paddingVertical: theme.space[12] },
  });

  const renderEmpty = () => {
    if (loading) return <StateView state={{ kind: "loading" }} />;
    if (errorKind === "forbidden") return <StateView state={{ kind: "forbidden" }} />;
    if (errorKind === "error") {
      return <StateView state={{ kind: "error", retry: () => void fetchSessions(1, keyword, "reset") }} />;
    }
    if (sessions.length === 0) {
      return <StateView state={{ kind: "empty", title: "这里还没有任务", description: "从工作台新建任务后，会话会出现在这里。" }} />;
    }
    return <StateView state={{ kind: "empty", title: "没有匹配的会话", description: "换个关键词或筛选条件试试。" }} />;
  };

  return (
    <View style={styles.wrap}>
      <View style={styles.header}>
        <Text style={styles.title} accessibilityRole="header">
          会话
        </Text>
        <SearchField value={keywordInput} onChangeText={setKeywordInput} placeholder="搜索会话" testID="sessions-search" />
        <FilterChips
          chips={FILTER_CHIPS}
          selected={filter}
          onSelect={(k) => setFilter(k)}
          testID="sessions-filter"
        />
      </View>
      <FlatList
        data={filtered}
        keyExtractor={(s) => s.id}
        renderItem={({ item }) => (
          <TaskCard
            title={item.title}
            agentName={item.agent_id ? agentNames[item.agent_id] ?? "助手" : undefined}
            runBadge={badgeFor(deriveStatus(item))}
            meta={item.updated_at ? `更新于 ${formatRelative(item.updated_at)}` : undefined}
            onPress={() => router.push(`/sessions/${item.id}`)}
            testID={`session-card-${item.id}`}
          />
        )}
        ItemSeparatorComponent={() => <View style={styles.separator} />}
        ListEmptyComponent={renderEmpty}
        ListFooterComponent={loadingMore ? <View style={styles.footer}><StateView state={{ kind: "loading" }} /></View> : null}
        onEndReached={() => {
          if (hasMore && !loading && !loadingMore && !busyRef.current) {
            void fetchSessions(page + 1, keyword, "more");
          }
        }}
        onEndReachedThreshold={0.4}
        refreshControl={
          <RefreshControl
            refreshing={refreshing}
            onRefresh={() => {
              void fetchSessions(1, keyword, "refresh").then(() => void loadPending());
            }}
            tintColor={theme.c.brand}
            colors={[theme.c.brand]}
          />
        }
        contentContainerStyle={styles.list}
        keyboardShouldPersistTaps="handled"
      />
    </View>
  );
}
