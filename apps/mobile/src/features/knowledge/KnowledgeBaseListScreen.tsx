import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  ActivityIndicator,
  Modal,
  Pressable,
  SafeAreaView,
  Text,
  TextInput,
  View,
  FlatList,
} from "react-native";
import { useRouter } from "expo-router";
import * as SecureStore from "expo-secure-store";
import type { KnowledgeBase } from "@weknora/contracts";
import {
  filterByScope,
  groupKnowledgeBaseSections,
  type KnowledgeBaseScope,
} from "@weknora/domain/knowledge/list";
import { useMobileRuntime } from "../../runtime.tsx";
import { knowledgeHeaderLayout } from "./header-layout.ts";
import { signOutAndRedirect } from "./sign-out.ts";
import { canCreateKnowledgeBase, knowledgeBaseCountLabel, knowledgeListLabel } from "./list.ts";
import { applyUploadTaskEvent, createUploadTaskCleanups, subscribeToUploadEvents, summarizeUploadTasks, UPLOAD_REFRESH_DELAY_MS, type UploadTaskState } from "./upload-progress.ts";
import { addKbFavorite, createPinsGeneration, fetchKbFavoriteIds, readKbRecents, removeKbFavorite, touchKbRecent } from "./pins.ts";

const scopes: Array<{ key: KnowledgeBaseScope; labelKey: string }> = [
  { key: "all", labelKey: "common.all" },
  { key: "mine", labelKey: "knowledgeList.sections.mine" },
  { key: "favorites", labelKey: "knowledgeList.scope.favorites" },
  { key: "recents", labelKey: "knowledgeList.scope.recents" },
];

export function KnowledgeBaseListScreen() {
  const runtime = useMobileRuntime();
  const router = useRouter();
  const [items, setItems] = useState<KnowledgeBase[]>([]);
  const [mineItems, setMineItems] = useState<KnowledgeBase[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [scope, setScope] = useState<KnowledgeBaseScope>("all");
  const [favoriteIds, setFavoriteIds] = useState<Set<string>>(new Set());
  const [recentIds, setRecentIds] = useState<Set<string>>(new Set());
  const [uploadTasks, setUploadTasks] = useState<UploadTaskState[]>([]);
  const [createVisible, setCreateVisible] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [creating, setCreating] = useState(false);
  const loadGeneration = useRef(0);
  const activeWorkspace = runtime.workspaces.find(
    (workspace) => String(workspace.id) === runtime.tenantId,
  );
  const writable = canCreateKnowledgeBase(activeWorkspace?.role);

  const load = useCallback(async () => {
    const generation = ++loadGeneration.current;
    setLoading(true);
    setError("");
    try {
      const [next, mine] = await Promise.all([
        runtime.client.knowledgeBases.list(),
        runtime.client.knowledgeBases.list({ creator: "mine" }),
      ]);
      if (generation !== loadGeneration.current) return;
      setItems(next);
      setMineItems(mine);
      setFavoriteIds(
        (current) =>
          new Set([
            ...current,
            ...next
              .filter((item) => item.is_favorite === true)
              .map((item) => item.id),
          ]),
      );
    } catch (cause) {
      if (generation !== loadGeneration.current) return;
      setError(
        cause instanceof Error
          ? cause.message
          : knowledgeListLabel(runtime.locale, "knowledgeList.loadFailed"),
      );
    } finally {
      if (generation === loadGeneration.current) setLoading(false);
    }
  }, [runtime.client]);
  useEffect(() => {
    void load();
  }, [load]);

  // Upload progress mask — port of the Vue KnowledgeBaseList.vue listeners
  // (lines 1287-1290): apply every upload event to the task list, drop settled
  // tasks after the 10s cleanup delay, and debounce-refresh the list 800ms
  // after a batch finishes uploading.
  useEffect(() => {
    const cleanups = createUploadTaskCleanups((uploadId) => {
      setUploadTasks((current) => current.filter((task) => task.uploadId !== uploadId));
    });
    let refreshTimer: ReturnType<typeof setTimeout> | null = null;
    const unsubscribe = subscribeToUploadEvents((event) => {
      setUploadTasks((current) => applyUploadTaskEvent(current, event));
      if (event.type === "complete") cleanups.schedule(event.uploadId);
      if (event.type === "uploaded") {
        if (refreshTimer) clearTimeout(refreshTimer);
        refreshTimer = setTimeout(() => { void load(); }, UPLOAD_REFRESH_DELAY_MS);
      }
    });
    return () => {
      unsubscribe();
      cleanups.clear();
      if (refreshTimer) clearTimeout(refreshTimer);
    };
  }, [load]);

  const uploadSummaries = useMemo(
    () => summarizeUploadTasks(uploadTasks, (kbId) =>
      items.find((item) => item.id === kbId)?.name
      || knowledgeListLabel(runtime.locale, "knowledgeList.uploadProgress.unknownKb", { id: kbId })),
    [items, runtime.locale, uploadTasks],
  );

  // Persistent pins/recents — port of the Vue useResourcePins composable.
  // Favorites come from the DB-backed /user/favorites endpoints (they outlive
  // app restarts and sync across devices); recents are restored from the
  // device storage the runtime already uses (expo-secure-store), keyed per
  // user and tenant exactly like the Vue localStorage key.
  const tenantId = runtime.tenantId;
  // expo-secure-store adapter for the recents store — the async API the
  // runtime already uses for locale/workspace persistence.
  const recentsStorage = useMemo(() => ({
    getItem: (key: string) => SecureStore.getItemAsync(key),
    setItem: (key: string, value: string) => SecureStore.setItemAsync(key, value),
  }), []);
  const pinsGeneration = useMemo(() => createPinsGeneration(), []);
  const userId = runtime.userId;
  useEffect(() => {
    const generation = pinsGeneration.next();
    setFavoriteIds(new Set());
    setRecentIds(new Set());
    let active = true;
    void Promise.all([
      fetchKbFavoriteIds(runtime.client),
      readKbRecents(recentsStorage, userId, tenantId),
    ]).then(([ids, entries]) => {
      if (!active || !pinsGeneration.isCurrent(generation)) return;
      setFavoriteIds(ids);
      setRecentIds(new Set(entries.filter((entry) => entry.type === "kb").map((entry) => entry.id)));
    }).catch(() => undefined);
    return () => { active = false; };
  }, [pinsGeneration, recentsStorage, runtime.client, tenantId, userId]);

  const visibleItems = useMemo(() => {
    const source = scope === "mine" ? mineItems : items;
    return filterByScope(
      source,
      scope === "mine" ? "all" : scope,
      undefined,
      favoriteIds,
      recentIds,
    );
  }, [items, mineItems, scope, favoriteIds, recentIds]);
  const sections = useMemo(
    () => groupKnowledgeBaseSections(visibleItems, undefined),
    [visibleItems],
  );
  function openKnowledgeBase(item: KnowledgeBase) {
    setRecentIds((current) => new Set(current).add(item.id));
    // Persist the visit so recents survive an app restart (Vue touchRecent).
    void touchKbRecent(recentsStorage, runtime.userId, runtime.tenantId, item.id).catch(() => undefined);
    router.push(`/knowledge/${item.id}`);
  }
  function toggleFavorite(item: KnowledgeBase) {
    const wasFavorite = favoriteIds.has(item.id);
    // Optimistic like the Vue composable; silently roll back on failure (the
    // restored star is the error indication) and re-read server truth on load.
    setFavoriteIds((current) => {
      const next = new Set(current);
      if (next.has(item.id)) next.delete(item.id);
      else next.add(item.id);
      return next;
    });
    const restore = () => setFavoriteIds((current) => {
      const next = new Set(current);
      if (wasFavorite) next.add(item.id);
      else next.delete(item.id);
      return next;
    });
    const request = wasFavorite ? removeKbFavorite(runtime.client, item.id) : addKbFavorite(runtime.client, item.id);
    void request.catch(() => restore());
  }
  async function togglePin(item: KnowledgeBase) {
    try {
      const { is_pinned } = await runtime.client.knowledgeBases.togglePin(item.id);
      setItems((current) => current.map((row) => row.id === item.id ? { ...row, is_pinned } : row));
      setMineItems((current) => current.map((row) => row.id === item.id ? { ...row, is_pinned } : row));
    }
    catch (cause) {
      setError(cause instanceof Error ? cause.message : knowledgeListLabel(runtime.locale, "knowledgeList.pin.failed"));
    }
  }
  async function createKnowledgeBase() {
    const trimmed = name.trim();
    if (!trimmed) {
      setError(knowledgeListLabel(runtime.locale, "knowledgeEditor.messages.nameRequired"));
      return;
    }
    setCreating(true);
    setError("");
    try {
      await runtime.client.knowledgeBases.create({
        name: trimmed,
        description: description.trim() || undefined,
        type: "document",
      });
      setCreateVisible(false);
      setName("");
      setDescription("");
      await load();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : knowledgeListLabel(runtime.locale, "knowledgeEditor.messages.createFailed"),
      );
    } finally {
      setCreating(false);
    }
  }

  return (
    <SafeAreaView style={{ flex: 1, padding: 16 }}>
      <View style={knowledgeHeaderLayout.container}>
        <View
          style={{
            flexDirection: "row",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 12,
          }}
        >
          <Text accessibilityRole="header" style={knowledgeHeaderLayout.title}>
            {knowledgeListLabel(runtime.locale, "knowledgeBase.title")}
          </Text>
          {writable ? (
            <Pressable
              accessibilityRole="button"
              onPress={() => setCreateVisible(true)}
            >
              <Text style={knowledgeHeaderLayout.actionText}>+ {knowledgeListLabel(runtime.locale, "knowledgeList.create")}</Text>
            </Pressable>
          ) : null}
        </View>
        <View style={knowledgeHeaderLayout.actions}>
          <Pressable
            accessibilityRole="button"
            onPress={() => router.push(knowledgeHeaderLayout.chatRoute)}
          >
            <Text style={knowledgeHeaderLayout.actionText}>{knowledgeListLabel(runtime.locale, "menu.newChat")}</Text>
          </Pressable>
          <Pressable
            accessibilityRole="button"
            onPress={() => router.push("/workspace")}
          >
            <Text style={knowledgeHeaderLayout.actionText}>{knowledgeListLabel(runtime.locale, "menu.organizations")}</Text>
          </Pressable>
          <Pressable
            accessibilityRole="button"
            onPress={() => router.push("/management")}
          >
            <Text style={knowledgeHeaderLayout.actionText}>{knowledgeListLabel(runtime.locale, "menu.settings")}</Text>
          </Pressable>
          <Pressable
            accessibilityRole="button"
            onPress={() =>
              void signOutAndRedirect(runtime.logout, (path) =>
                router.replace(path),
              )
            }
          >
            <Text style={knowledgeHeaderLayout.actionText}>{knowledgeListLabel(runtime.locale, "menu.logout")}</Text>
          </Pressable>
        </View>
      </View>
      <View
        accessibilityRole="tablist"
        style={{
          flexDirection: "row",
          flexWrap: "wrap",
          gap: 8,
          marginBottom: 12,
        }}
      >
        {scopes.map((entry) => (
          <Pressable
            key={entry.key}
            accessibilityRole="tab"
            accessibilityState={{ selected: scope === entry.key }}
            onPress={() => setScope(entry.key)}
            style={{
              backgroundColor: scope === entry.key ? "#dbeafe" : "#f2f4f7",
              borderRadius: 16,
              paddingHorizontal: 12,
              paddingVertical: 7,
            }}
          >
            <Text>{knowledgeListLabel(runtime.locale, entry.labelKey)}</Text>
          </Pressable>
        ))}
      </View>
      {error ? (
        <Text
          accessibilityRole="alert"
          style={{ color: "#b42318", marginBottom: 12 }}
        >
          {error}
        </Text>
      ) : null}
      {uploadSummaries.length ? (
        <View
          accessibilityLabel={knowledgeListLabel(runtime.locale, "knowledgeList.uploadProgress.detail", { completed: uploadSummaries[0]?.completed ?? 0, total: uploadSummaries[0]?.total ?? 0 })}
          style={{ borderColor: "#d0d5dd", borderWidth: 1, borderRadius: 8, padding: 12, marginBottom: 12, gap: 10 }}
        >
          {uploadSummaries.map((summary) => {
            const done = summary.completed === summary.total;
            return (
              <View key={summary.kbId} style={{ gap: 3 }}>
                <Text style={{ fontWeight: "600" }}>
                  {done ? "✓" : "↑"}{" "}
                  {knowledgeListLabel(
                    runtime.locale,
                    done ? "knowledgeList.uploadProgress.completedTitle" : "knowledgeList.uploadProgress.uploadingTitle",
                    { name: summary.kbName },
                  )}
                </Text>
                <Text style={{ color: "#667085", fontSize: 12 }}>
                  {knowledgeListLabel(
                    runtime.locale,
                    done ? "knowledgeList.uploadProgress.completedDetail" : "knowledgeList.uploadProgress.detail",
                    done ? { total: summary.total } : { completed: summary.completed, total: summary.total },
                  )}
                </Text>
                <Text style={{ color: "#98a2b3", fontSize: 12 }}>
                  {knowledgeListLabel(
                    runtime.locale,
                    done ? "knowledgeList.uploadProgress.refreshing" : "knowledgeList.uploadProgress.keepPageOpen",
                  )}
                </Text>
                {summary.hasError ? (
                  <Text style={{ color: "#b42318", fontSize: 12 }}>
                    {knowledgeListLabel(runtime.locale, "knowledgeList.uploadProgress.errorTip")}
                  </Text>
                ) : null}
                <View style={{ height: 6, borderRadius: 3, backgroundColor: "#eaecf0" }}>
                  <View style={{ height: 6, borderRadius: 3, width: `${summary.progress}%`, backgroundColor: "#2864dc" }} />
                </View>
              </View>
            );
          })}
        </View>
      ) : null}
      {loading ? (
        <ActivityIndicator accessibilityLabel={knowledgeListLabel(runtime.locale, "common.loading")} />
      ) : visibleItems.length === 0 ? (
        <View style={{ alignItems: "center", paddingVertical: 36 }}>
          <Text style={{ color: "#667085", fontSize: 16 }}>
            {scope === "all" || scope === "mine"
              ? knowledgeListLabel(runtime.locale, "knowledgeList.empty.title")
              : scope === "favorites"
                ? knowledgeListLabel(runtime.locale, "knowledgeList.empty.favoritesTitle")
                : knowledgeListLabel(runtime.locale, "knowledgeList.empty.recentsTitle")}
          </Text>
          {writable && (scope === "all" || scope === "mine") ? (
            <Pressable
              onPress={() => setCreateVisible(true)}
              style={{
                marginTop: 14,
                backgroundColor: "#2864dc",
                borderRadius: 8,
                paddingHorizontal: 14,
                paddingVertical: 10,
              }}
            >
              <Text style={{ color: "#fff", fontWeight: "600" }}>
                {knowledgeListLabel(runtime.locale, "knowledgeList.create")}
              </Text>
            </Pressable>
          ) : null}
        </View>
      ) : (
        <FlatList
          data={
            sections.flatMap((section) => [
              { __section: section.labelKey, id: `section-${section.key}` },
              ...section.items,
            ]) as Array<KnowledgeBase & { __section?: string }>
          }
          keyExtractor={(item) => item.id}
          renderItem={({ item }) =>
            item.__section ? (
              <Text
                style={{
                  color: "#667085",
                  fontWeight: "700",
                  marginTop: 12,
                  marginBottom: 4,
                }}
              >
                {knowledgeListLabel(runtime.locale, item.__section)}
              </Text>
            ) : (
              <View
                style={{
                  flexDirection: "row",
                  alignItems: "center",
                  borderBottomColor: "#eaecf0",
                  borderBottomWidth: 1,
                  paddingVertical: 14,
                }}
              >
                <Pressable
                  accessibilityRole="button"
                  onPress={() => openKnowledgeBase(item)}
                  style={{ flex: 1 }}
                >
                  <Text style={{ fontWeight: "600", fontSize: 16 }}>
                    {item.name}
                  </Text>
                  <Text
                    style={{ color: "#667085", fontSize: 12, marginTop: 3 }}
                  >
                    {typeof item.description === "string" && item.description
                      ? item.description
                      : knowledgeListLabel(runtime.locale, "knowledgeBase.noDescription")}
                  </Text>
                  <Text
                    style={{ color: "#98a2b3", fontSize: 11, marginTop: 4 }}
                  >
                  {knowledgeBaseCountLabel(item, runtime.locale)}
                  </Text>
                </Pressable>
                <Pressable
                  accessibilityRole="button"
                  accessibilityLabel={`${knowledgeListLabel(runtime.locale, item.is_pinned === true ? "knowledgeList.pin.unpin" : "knowledgeList.pin.pin")} ${item.name}`}
                  onPress={() => void togglePin(item)}
                  style={{ padding: 10 }}
                >
                  <Text
                    style={{
                      color: item.is_pinned === true ? "#2864dc" : "#98a2b3",
                      fontSize: 20,
                    }}
                  >
                    {item.is_pinned === true ? "📌" : "📍"}
                  </Text>
                </Pressable>
                <Pressable
                  accessibilityRole="button"
                  accessibilityLabel={`${knowledgeListLabel(runtime.locale, favoriteIds.has(item.id) ? "knowledgeList.accessibility.unfavorite" : "knowledgeList.accessibility.favorite")} ${item.name}`}
                  onPress={() => toggleFavorite(item)}
                  style={{ padding: 10 }}
                >
                  <Text
                    style={{
                      color: favoriteIds.has(item.id) ? "#f59e0b" : "#98a2b3",
                      fontSize: 22,
                    }}
                  >
                    {favoriteIds.has(item.id) ? "★" : "☆"}
                  </Text>
                </Pressable>
              </View>
            )
          }
        />
      )}
      <Modal
        visible={createVisible}
        transparent
        animationType="slide"
        onRequestClose={() => setCreateVisible(false)}
      >
        <View
          style={{
            flex: 1,
            justifyContent: "flex-end",
            backgroundColor: "#0006",
          }}
        >
          <View
            style={{
              backgroundColor: "#fff",
              borderTopLeftRadius: 16,
              borderTopRightRadius: 16,
              padding: 20,
              gap: 10,
            }}
          >
            <Text style={{ fontSize: 20, fontWeight: "700" }}>
              {knowledgeListLabel(runtime.locale, "knowledgeList.create")}
            </Text>
            <TextInput
              accessibilityLabel={knowledgeListLabel(runtime.locale, "knowledgeBase.name")}
              value={name}
              onChangeText={setName}
              placeholder={knowledgeListLabel(runtime.locale, "knowledgeBase.name")}
              autoFocus
              style={{
                borderColor: "#d0d5dd",
                borderWidth: 1,
                borderRadius: 8,
                padding: 10,
              }}
            />
            <TextInput
              accessibilityLabel={knowledgeListLabel(runtime.locale, "knowledgeBase.description")}
              value={description}
              onChangeText={setDescription}
              placeholder={knowledgeListLabel(runtime.locale, "knowledgeBase.description")}
              multiline
              style={{
                borderColor: "#d0d5dd",
                borderWidth: 1,
                borderRadius: 8,
                padding: 10,
                minHeight: 70,
              }}
            />
            <View
              style={{
                flexDirection: "row",
                justifyContent: "flex-end",
                gap: 12,
              }}
            >
              <Pressable onPress={() => setCreateVisible(false)}>
                <Text style={{ color: "#667085", padding: 10 }}>{knowledgeListLabel(runtime.locale, "common.cancel")}</Text>
              </Pressable>
              <Pressable
                disabled={creating}
                onPress={() => void createKnowledgeBase()}
                style={{
                  backgroundColor: "#2864dc",
                  borderRadius: 8,
                  padding: 10,
                  opacity: creating ? 0.5 : 1,
                }}
              >
                <Text style={{ color: "#fff", fontWeight: "600" }}>
                  {creating ? knowledgeListLabel(runtime.locale, "common.loading") : knowledgeListLabel(runtime.locale, "knowledgeList.create")}
                </Text>
              </Pressable>
            </View>
          </View>
        </View>
      </Modal>
    </SafeAreaView>
  );
}
