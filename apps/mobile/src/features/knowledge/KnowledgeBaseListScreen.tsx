import { useCallback, useEffect, useMemo, useState } from "react";
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
import type { KnowledgeBase } from "@weknora/contracts";
import {
  filterByScope,
  groupKnowledgeBaseSections,
  type KnowledgeBaseScope,
} from "@weknora/domain/knowledge/list";
import { useMobileRuntime } from "../../runtime.tsx";
import { knowledgeHeaderLayout } from "./header-layout.ts";
import { signOutAndRedirect } from "./sign-out.ts";
import { canCreateKnowledgeBase, knowledgeBaseCountLabel } from "./list.ts";

const scopes: Array<{ key: KnowledgeBaseScope; label: string }> = [
  { key: "all", label: "All" },
  { key: "mine", label: "Mine" },
  { key: "favorites", label: "Favorites" },
  { key: "recents", label: "Recent" },
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
  const [createVisible, setCreateVisible] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [creating, setCreating] = useState(false);
  const activeWorkspace = runtime.workspaces.find(
    (workspace) => String(workspace.id) === runtime.tenantId,
  );
  const writable = canCreateKnowledgeBase(activeWorkspace?.role);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [next, mine] = await Promise.all([
        runtime.client.knowledgeBases.list(),
        runtime.client.knowledgeBases.list({ creator: "mine" }),
      ]);
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
      setError(
        cause instanceof Error
          ? cause.message
          : "Unable to load knowledge bases",
      );
    } finally {
      setLoading(false);
    }
  }, [runtime.client]);
  useEffect(() => {
    void load();
  }, [load]);

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
    router.push(`/knowledge/${item.id}`);
  }
  function toggleFavorite(item: KnowledgeBase) {
    setFavoriteIds((current) => {
      const next = new Set(current);
      if (next.has(item.id)) next.delete(item.id);
      else next.add(item.id);
      return next;
    });
  }
  async function createKnowledgeBase() {
    const trimmed = name.trim();
    if (!trimmed) {
      setError("Knowledge base name is required");
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
          : "Unable to create knowledge base",
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
            Knowledge bases
          </Text>
          {writable ? (
            <Pressable
              accessibilityRole="button"
              onPress={() => setCreateVisible(true)}
            >
              <Text style={knowledgeHeaderLayout.actionText}>+ Create</Text>
            </Pressable>
          ) : null}
        </View>
        <View style={knowledgeHeaderLayout.actions}>
          <Pressable
            accessibilityRole="button"
            onPress={() => router.push(knowledgeHeaderLayout.chatRoute)}
          >
            <Text style={knowledgeHeaderLayout.actionText}>Chat</Text>
          </Pressable>
          <Pressable
            accessibilityRole="button"
            onPress={() => router.push("/workspace")}
          >
            <Text style={knowledgeHeaderLayout.actionText}>Workspace</Text>
          </Pressable>
          <Pressable
            accessibilityRole="button"
            onPress={() => router.push("/management")}
          >
            <Text style={knowledgeHeaderLayout.actionText}>Manage</Text>
          </Pressable>
          <Pressable
            accessibilityRole="button"
            onPress={() =>
              void signOutAndRedirect(runtime.logout, (path) =>
                router.replace(path),
              )
            }
          >
            <Text style={knowledgeHeaderLayout.actionText}>Sign out</Text>
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
            <Text>{entry.label}</Text>
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
      {loading ? (
        <ActivityIndicator accessibilityLabel="Loading knowledge bases" />
      ) : visibleItems.length === 0 ? (
        <View style={{ alignItems: "center", paddingVertical: 36 }}>
          <Text style={{ color: "#667085", fontSize: 16 }}>
            {scope === "all" || scope === "mine"
              ? "No knowledge bases available."
              : `No ${scope} knowledge bases.`}
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
                Create knowledge base
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
                {item.__section.split(".").pop()}
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
                      : "No description"}
                  </Text>
                  <Text
                    style={{ color: "#98a2b3", fontSize: 11, marginTop: 4 }}
                  >
                    {knowledgeBaseCountLabel(item)}
                  </Text>
                </Pressable>
                <Pressable
                  accessibilityRole="button"
                  accessibilityLabel={`${favoriteIds.has(item.id) ? "Unfavorite" : "Favorite"} ${item.name}`}
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
              Create knowledge base
            </Text>
            <TextInput
              accessibilityLabel="Knowledge base name"
              value={name}
              onChangeText={setName}
              placeholder="Name"
              autoFocus
              style={{
                borderColor: "#d0d5dd",
                borderWidth: 1,
                borderRadius: 8,
                padding: 10,
              }}
            />
            <TextInput
              accessibilityLabel="Knowledge base description"
              value={description}
              onChangeText={setDescription}
              placeholder="Description (optional)"
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
                <Text style={{ color: "#667085", padding: 10 }}>Cancel</Text>
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
                  {creating ? "Creating…" : "Create"}
                </Text>
              </Pressable>
            </View>
          </View>
        </View>
      </Modal>
    </SafeAreaView>
  );
}
