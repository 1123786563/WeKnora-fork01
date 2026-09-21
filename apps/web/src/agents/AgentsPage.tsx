/**
 * agents 列表页 —— TDesign 同构迁移（Task 9 pilot）。
 *
 * 事实源：frontend/src/views/agent/AgentList.vue（template/DOM/类名 1:1 复刻）
 * + frontend/src/components/{ListSpaceSidebar,AgentAvatar,SpaceAvatar,
 * ResourceOriginBadge}.vue。组件从 tdesign-react 具名导入（playbook §1），
 * 图标用 tdesign-icons-react 的 Icon（= Vue 端 `Icon as TIcon`，本地 sprite
 * `<use>` 渲染，与 Vue 端同源同字形）；样式平移在 agents.td.css。
 * 纯逻辑仍在 list.ts / state.ts / api.ts（本文件不重复实现）。
 * wk-* / data-* 测试 hook 按迁移前锚点保留（playbook §2.4）。
 */
import { useCallback, useEffect, useMemo, useState, type CSSProperties } from 'react';
import type { AgentConfiguration, WeKnoraClient } from '@weknora/api-client';
import { Button, Dialog, Loading, Popup, Skeleton, Tag, Tooltip } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { MessagePlugin } from 'tdesign-react';
import { formatMessage, type Locale } from '@weknora/i18n';
import { usePreferredLocale } from '../locale.ts';
import { navigate } from '../platform/navigation.ts';
import {
  contextualGuideMessage,
  markContextualGuideDone,
  openContextualGuide,
} from '@weknora/views/guides/contextual-guides';
import './agents.td.css';
/* agents.css 只剩 configuration/ConfigurationPage 仍在消费的共享段
 * （wk-agent-section-header / count，CSS retention criteria），随本页模块
 * 一并加载，待 configuration 域迁移时清理（playbook §4.3）。 */
import './agents.css';
import { AgentEditorModal } from './AgentEditorModal.tsx';
import { makeEditorT } from './agent-editor.ts';
import {
  agentSectionLabelKey,
  avatarGradient,
  avatarLetter,
  buildAllViewRows,
  buildMineViewRows,
  buildPinIndex,
  buildSpaceViewRows,
  canManageAgent,
  cardActions,
  chatNavigationPath,
  cornerBadge,
  expertSourceId,
  featureBadges,
  hydratePinnedCards,
  kbScope,
  mcpScope,
  FEATURE_BADGE_TITLE_KEYS,
  opensEditorOnCardClick,
  readAgentRecents,
  readFavoriteIds,
  SECTION_ICON_KEYS,
  sectionize,
  sharedAgentsFromRecords,
  sharedCountByOrg,
  toggleFavoriteId,
  touchAgentRecent,
  writeAgentRecents,
  writeFavoriteIds,
  type AgentCardAction,
  type AgentCardModel,
  type AgentSectionView,
  type AgentViewer,
  type PinEntry,
  type SharedAgentSummary,
} from './list.ts';

type Translate = (key: string, values?: Record<string, string | number>) => string;

export interface AgentsPageData {
  ownAgents: AgentConfiguration[];
  disabledOwnIds: string[];
  organizations: Array<{ id: string; name: string; avatar?: string }>;
  sharedAgents: SharedAgentSummary[];
}

export interface AgentEditDeepLink {
  editId: string;
  section: string;
  highlight?: string;
  sourceTenantId?: string;
}

const AGENT_EDITOR_SECTIONS = new Set(['basic', 'prompts', 'model', 'conversation', 'knowledge', 'retrieval', 'websearch', 'tools', 'skills']);
const AGENT_HIGHLIGHT_SECTIONS: Record<string, string> = { summary_model: 'model', rerank_model: 'model', allowed_tools: 'tools' };

export function parseAgentEditDeepLink(search: string): AgentEditDeepLink | null {
  const params = new URLSearchParams(search);
  const editId = params.get('edit')?.trim();
  if (!editId) return null;
  const requestedSection = params.get('section')?.trim();
  const highlight = params.get('highlight')?.trim() || undefined;
  const section = requestedSection || (highlight ? AGENT_HIGHLIGHT_SECTIONS[highlight] : undefined) || 'basic';
  return {
    editId,
    section: section === 'sandbox' ? 'skills' : AGENT_EDITOR_SECTIONS.has(section) ? section : 'basic',
    highlight,
    sourceTenantId: params.get('sourceTenantId')?.trim() || undefined,
  };
}

export function resolveAgentEditTarget(ownAgents: AgentCardModel[], sharedAgents: AgentCardModel[], editId: string, sourceTenantId?: string): AgentCardModel | null {
  const own = ownAgents.find((agent) => agent.id === editId);
  if (own) return own;
  if (!sourceTenantId) return null;
  return sharedAgents.find((agent) => agent.id === editId && String(agent.sourceTenantId) === sourceTenantId) ?? null;
}

/** Parallel fetch behind the Vue chatResources.fetchAgentsForList + orgStore duo. */
export async function loadAgentsPageData(client: WeKnoraClient): Promise<AgentsPageData> {
  const [agents, organizations, sharedRecords] = await Promise.all([
    client.configuration.agents.listWithState({ creator: 'all' }),
    client.identity.organizations.list(),
    client.identity.organizations.agentShares.listShared(),
  ]);
  return {
    ownAgents: agents.items,
    disabledOwnIds: agents.disabledOwnAgentIds,
    organizations: organizations.items.map((org) => ({ id: String(org.id), name: String(org.name ?? '') })),
    sharedAgents: sharedAgentsFromRecords(sharedRecords),
  };
}

// --- 图片资源（Vue @/assets/img 内联副本；src URL 不参与像素对比） --------------

/* frontend/src/assets/img/more.png —— 卡片三点按钮（32×32 PNG）。 */
const MORE_PNG = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACAAAAAgBAMAAACBVGfHAAAAD1BMVEUAAAAwMTMwMDMwMjIwMTPbLw9bAAAABHRSTlMA3llYOk1BewAAABxJREFUKM9jGGnAUAiJAAERRwSBXUBRCIkYYQAAnNMDYY7Uun8AAAAASUVORK5CYII=';
/* frontend/src/assets/img/circle.png —— 删除确认弹窗警示图（48×48 PNG）。 */
const CIRCLE_PNG = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAMAAABg3Am1AAAACXBIWXMAACE4AAAhOAFFljFgAAAABGdBTUEAALGPC/xhBQAAAPZQTFRFAAAA91BQ+lBQ9FBQ/1BQ/FBQ+FBQ/FJS+lBQ91BQ/FFR/FBQ+VBQ+FBQ+lNT+FBQ+1JS+VBQ+lBQ+FBQ/FBQ+VBQ91BQ+lNT+1FR+1BQ+FBQ+VFR+VBQ+lFR+VFR91BQ+lFR+VFR+FFR+1JS+1FR+VFR+VBQ+lBQ+FBQ+VFR+FFR+1FR+lFR+lBQ+lFR+lBQ+lFR+VFR+VBQ+lFR+lJS+lFR+VFR+lFR+VFR+lFR+VFR+lFR+VFR+lFR+lFR+lFR+lFR+lFR+lFR+lFR+lFR+lFR+lFR+VFR+lFR+lFR+lFR+lFR+lFR+lFR+lFR8WpGUQAAAFF0Uk5TAAQFBQcJCRISEhcXFxcZGRwcJCQqKysxMjIyOTk8PEBDQ0NGR0dHS0tOYGp/f4CAg4ODh42VlZycn5+mp62ur7O0tbi/wc7V2Njc3+bt8fT4NJ1opQAAAAFiS0dEUg1gLZAAAAFuSURBVHjaxdXJUsJAFIXhAwYNIjIGEQQBB1AQFQWR2TDEgQTu+7+MVUmFSpoMDRu+/V91e3MahxGKJCUpGQmCi1B9ncwXirKYjxu3AnyclD40slD7BREeLmSVGMthGm7OP1fkYN2Ow1H2i1xM89gWqGjkSisHwKr8kIfvMhh5jTxpWdjEp+RDjsGqQ3ZPQIPserDIrP2DVQ4b4RH5BySLMBWXPIFagmlAPAF1AYOg8gWaAMMd8QVUhaHJG7xAFxzzBpOQHpzNeYP5qR6kFrzBIqkHksIbKNLOwc4n7ffo0IQ3GAeha9GWWbM5oy0NGGrE6QYGQSMu6mY4u8SlD1NJJcavovwRY1mASZSJ8SBJj8QYitjIsaOaAa7ZiU3DoucftGEVY456q9ffmUFOwCbrN5VX+48x/9yz8lNyIV/CUaKzJgerXgxuMqMlMVQ5Bw/h4kAlC61bEuFDuH8eGx/7pFUTwOUompKkVPQYB/EPlK2oyxaXjlIAAAAASUVORK5CYII=';
/* frontend/src/assets/img/organization-green.svg —— 共享来源空间徽标（20×20）。 */
function OrgGreenIcon() {
  return (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M10 10C8.8 7.5 7.8 3.8 4.8 3.8C2.2 3.8 0.8 6.8 0.8 10C0.8 13.2 2.2 16.2 4.8 16.2C7.8 16.2 8.8 12.5 10 10C11.2 7.5 12.5 5.5 14.5 5.5C16.5 5.5 18 7.5 18 10C18 12.5 16.5 14.5 14.5 14.5C12.5 14.5 11.2 12.5 10 10Z" stroke="#07C05F" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
    </svg>
  );
}
/* frontend/src/assets/img/upload.svg —— 空状态插画（162×162，10KB；内联副本）。 */
const UPLOAD_SVG = 'data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMTYyIiBoZWlnaHQ9IjE2MiIgdmlld0JveD0iMCAwIDE2MiAxNjIiIGZpbGw9Im5vbmUiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyI+CjxnIGZpbHRlcj0idXJsKCNmaWx0ZXIwX2RfNjAyMl81MTczMSkiPgo8cGF0aCBkPSJNMzYuODc1IDc4TDIwIDExMS43NVYxMzMuMDQ3QzIwIDE0MC43NiAyNi4yNTI2IDE0Ny4wMTMgMzMuOTY1NSAxNDcuMDEzSDgwLjc1SDEyNy41MzRDMTM1LjI0NyAxNDcuMDEzIDE0MS41IDE0MC43NiAxNDEuNSAxMzMuMDQ3VjExMS43NUwxMjQuNjI1IDc4SDgwLjc1SDM2Ljg3NVoiIGZpbGw9InVybCgjcGFpbnQwX2xpbmVhcl82MDIyXzUxNzMxKSIvPgo8L2c+CjxwYXRoIGQ9Ik0zNy4xMjUgMTExLjM3NVY3Ny42MjVMMjAuMjUgMTExLjM3NUgzNy4xMjVaIiBmaWxsPSJ1cmwoI3BhaW50MV9saW5lYXJfNjAyMl81MTczMSkiLz4KPHBhdGggZD0iTTEyNSAxMTEuNzVWNzhMMTQxLjg3NSAxMTEuNzVIMTI1WiIgZmlsbD0idXJsKCNwYWludDJfbGluZWFyXzYwMjJfNTE3MzEpIi8+CjxwYXRoIGQ9Ik03Ny45ODY0IDEwOC42MjdMNjYuMjc0IDkzLjc0MzZDNjUuNDAyOSA5Mi42MzY1IDY2LjE5MTUgOTEuMDEyNSA2Ny42MDAyIDkxLjAxMjVINzIuNjM1QzczLjU2NyA5MS4wMTI1IDc0LjMyOTIgOTAuMjYyNSA3NC4yMDExIDg5LjMzOTRDNzIuNjc1NCA3OC4zNTAzIDU2Ljg4MDUgNTkuNDM1NSAzMy4xMDA3IDUwLjg1NDlDMzIuMTcyOSA1MC41MjAxIDMyLjQwNjcgNDguOTM3NSAzMy4zOTMgNDguOTM3NUgxMjUuMjMyQzEyNi4yMTggNDguOTM3NSAxMjYuNDUyIDUwLjUyMDEgMTI1LjUyNCA1MC44NTQ5QzEwMS43NDQgNTkuNDM1NSA4NS45NDk2IDc4LjM1MDMgODQuNDIzOSA4OS4zMzk0Qzg0LjI5NTcgOTAuMjYyNSA4NS4wNTggOTEuMDEyNSA4NS45OSA5MS4wMTI1SDkxLjAyNDhDOTIuNDMzNSA5MS4wMTI1IDkzLjIyMjEgOTIuNjM2NSA5Mi4zNTEgOTMuNzQzNkw4MC42Mzg2IDEwOC42MjdDNzkuOTYzIDEwOS40ODYgNzguNjYyIDEwOS40ODYgNzcuOTg2NCAxMDguNjI3WiIgZmlsbD0idXJsKCNwYWludDNfbGluZWFyXzYwMjJfNTE3MzEpIi8+CjxwYXRoIGZpbGwtcnVsZT0iZXZlbm9kZCIgY2xpcC1ydWxlPSJldmVub2RkIiBkPSJNNjcuNjA0IDExMS4zNzVIMjAuMjVWMTMzLjEzOEMyMC4yNSAxNDAuNTk0IDI2LjI5NDIgMTQ2LjYzOCAzMy43NSAxNDYuNjM4SDEyOC4yNUMxMzUuNzA2IDE0Ni42MzggMTQxLjc1IDE0MC41OTQgMTQxLjc1IDEzMy4xMzhWMTExLjM3NUg5NC4zOTUxQzkzLjU2NDcgMTE4LjAzNCA4Ny44ODM5IDEyMy4xODggODAuOTk5NSAxMjMuMTg4Qzc0LjExNTIgMTIzLjE4OCA2OC40MzQ0IDExOC4wMzQgNjcuNjA0IDExMS4zNzVaIiBmaWxsPSJ1cmwoI3BhaW50NF9saW5lYXJfNjAyMl81MTczMSkiLz4KPHBhdGggZD0iTTQ2LjkzNjYgMTguNTQzNkM0Ni43NDA4IDE4LjE0MTEgNDYuNzEyOSAxNy42Nzc0IDQ2Ljg1OTEgMTcuMjU0NEw0OS4xMTAyIDEwLjczNzVDNDkuODcxIDguNTM1MiA1Mi4yNzI5IDcuMzY2NjEgNTQuNDc1MiA4LjEyNzM0TDY4LjgzMDQgMTMuMDg2MUM3MS4wMzI2IDEzLjg0NjggNzIuMjAxMiAxNi4yNDg4IDcxLjQ0MDUgMTguNDUxMUw2Ny41ODM3IDI5LjYxNjJDNjYuODIyOSAzMS44MTg1IDY0LjQyMSAzMi45ODcxIDYyLjIxODcgMzIuMjI2M0w1Mi41MTE3IDI4Ljg3MzJDNTIuMDg4NyAyOC43MjcxIDUxLjc0MTEgMjguNDE4OSA1MS41NDUzIDI4LjAxNjVMNDYuOTM2NiAxOC41NDM2WiIgZmlsbD0idXJsKCNwYWludDVfbGluZWFyXzYwMjJfNTE3MzEpIi8+CjxtYXNrIGlkPSJtYXNrMF82MDIyXzUxNzMxIiBzdHlsZT0ibWFzay10eXBlOmFscGhhIiBtYXNrVW5pdHM9InVzZXJTcGFjZU9uVXNlIiB4PSI0NiIgeT0iNyIgd2lkdGg9IjI2IiBoZWlnaHQ9IjI2Ij4KPHBhdGggZD0iTTQ2LjkzNjYgMTguNTQzNkM0Ni43NDA4IDE4LjE0MTEgNDYuNzEyOSAxNy42Nzc0IDQ2Ljg1OTEgMTcuMjU0NEw0OS4xMTAyIDEwLjczNzVDNDkuODcxIDguNTM1MiA1Mi4yNzI5IDcuMzY2NjEgNTQuNDc1MiA4LjEyNzM0TDY4LjgzMDQgMTMuMDg2MUM3MS4wMzI2IDEzLjg0NjggNzIuMjAxMiAxNi4yNDg4IDcxLjQ0MDUgMTguNDUxMUw2Ny41ODM3IDI5LjYxNjJDNjYuODIyOSAzMS44MTg1IDY0LjQyMSAzMi45ODcxIDYyLjIxODcgMzIuMjI2M0w1Mi41MTE3IDI4Ljg3MzJDNTIuMDg4NyAyOC43MjcxIDUxLjc0MTEgMjguNDE4OSA1MS41NDUzIDI4LjAxNjVMNDYuOTM2NiAxOC41NDM2WiIgZmlsbD0iI0Q5RDlEOSIvPgo8L21hc2s+CjxnIG1hc2s9InVybCgjbWFzazBfNjAyMl81MTczMSkiPgo8cGF0aCBkPSJNNDMuODc2IDI1Ljg5MDFMNDYuNjMwOCAxNy45MTVMNTEuNDE1OSAxOS41NjhDNTMuMTc3NyAyMC4xNzY1IDU0LjExMjYgMjIuMDk4MSA1My41MDQgMjMuODU5OUw1MS44NTExIDI4LjY0NUw0My44NzYgMjUuODkwMVoiIGZpbGw9IiNCNUVDQ0YiLz4KPC9nPgo8cGF0aCBkPSJNODkuNTU3MiAxNi40Mzg1Qzg5LjY2NjUgMTYuMTE4MSA4OS44OTg3IDE1Ljg1NDMgOTAuMjAyNSAxNS43MDUxTDk0Ljg4MzIgMTMuNDA2NkM5Ni40NjQ5IDEyLjYyOTkgOTguMzc2OCAxMy4yODI1IDk5LjE1MzUgMTQuODY0MkwxMDQuMjE3IDI1LjE3NDZDMTA0Ljk5MyAyNi43NTYzIDEwNC4zNDEgMjguNjY4MiAxMDIuNzU5IDI5LjQ0NUw5NC43Mzk4IDMzLjM4MjlDOTMuMTU4MSAzNC4xNTk2IDkxLjI0NjIgMzMuNTA3MSA5MC40Njk1IDMxLjkyNTNMODcuMDQ1OCAyNC45NTM1Qzg2Ljg5NjYgMjQuNjQ5NiA4Ni44NzQyIDI0LjI5OSA4Ni45ODM2IDIzLjk3ODZMODkuNTU3MiAxNi40Mzg1WiIgZmlsbD0idXJsKCNwYWludDZfbGluZWFyXzYwMjJfNTE3MzEpIi8+CjxtYXNrIGlkPSJtYXNrMV82MDIyXzUxNzMxIiBzdHlsZT0ibWFzay10eXBlOmFscGhhIiBtYXNrVW5pdHM9InVzZXJTcGFjZU9uVXNlIiB4PSI4NiIgeT0iMTMiIHdpZHRoPSIxOSIgaGVpZ2h0PSIyMSI+CjxwYXRoIGQ9Ik04OS41NTcyIDE2LjQzODVDODkuNjY2NSAxNi4xMTgxIDg5Ljg5ODcgMTUuODU0MyA5MC4yMDI1IDE1LjcwNTFMOTQuODgzMiAxMy40MDY2Qzk2LjQ2NDkgMTIuNjI5OSA5OC4zNzY4IDEzLjI4MjUgOTkuMTUzNSAxNC44NjQyTDEwNC4yMTcgMjUuMTc0NkMxMDQuOTkzIDI2Ljc1NjMgMTA0LjM0MSAyOC42NjgyIDEwMi43NTkgMjkuNDQ1TDk0LjczOTggMzMuMzgyOUM5My4xNTgxIDM0LjE1OTYgOTEuMjQ2MiAzMy41MDcxIDkwLjQ2OTUgMzEuOTI1M0w4Ny4wNDU4IDI0Ljk1MzVDODYuODk2NiAyNC42NDk2IDg2Ljg3NDIgMjQuMjk5IDg2Ljk4MzYgMjMuOTc4Nkw4OS41NTcyIDE2LjQzODVaIiBmaWxsPSIjRDlEOUQ5Ii8+CjwvbWFzaz4KPGcgbWFzaz0idXJsKCNtYXNrMV82MDIyXzUxNzMxKSI+CjxwYXRoIGQ9Ik04NCAxOC43NTFMODkuNzI4IDE1LjkzODJMOTEuNDE1NyAxOS4zNzVDOTIuMDM3MSAyMC42NDAzIDkxLjUxNSAyMi4xNjk5IDkwLjI0OTYgMjIuNzkxM0w4Ni44MTI4IDI0LjQ3OUw4NCAxOC43NTFaIiBmaWxsPSIjRTdFN0U3Ii8+CjwvZz4KPHBhdGggZD0iTTQ2LjM3MzQgNTcuMjI4OUM0Ni4yNTAyIDU3LjYxMjUgNDUuOTc5NiA1Ny45MzE1IDQ1LjYyMTMgNTguMTE1N0w0MC4xMDAxIDYwLjk1MzJDMzguMjM0NCA2MS45MTIxIDM1Ljk0NDUgNjEuMTc2OSAzNC45ODU3IDU5LjMxMTFMMjguNzM1NCA0Ny4xNDk0QzI3Ljc3NjYgNDUuMjgzNiAyOC41MTE4IDQyLjk5MzggMzAuMzc3NSA0Mi4wMzQ5TDM5LjgzNjYgMzcuMTczNkM0MS43MDI0IDM2LjIxNDggNDMuOTkyMiAzNi45NSA0NC45NTExIDM4LjgxNTdMNDkuMTc3NSA0Ny4wMzk1QzQ5LjM2MTcgNDcuMzk3OSA0OS4zOTU5IDQ3LjgxNDcgNDkuMjcyOCA0OC4xOTg0TDQ2LjM3MzQgNTcuMjI4OVoiIGZpbGw9InVybCgjcGFpbnQ3X2xpbmVhcl82MDIyXzUxNzMxKSIvPgo8bWFzayBpZD0ibWFzazJfNjAyMl81MTczMSIgc3R5bGU9Im1hc2stdHlwZTphbHBoYSIgbWFza1VuaXRzPSJ1c2VyU3BhY2VPblVzZSIgeD0iMjgiIHk9IjM2IiB3aWR0aD0iMjIiIGhlaWdodD0iMjYiPgo8cGF0aCBkPSJNNDYuMzczNCA1Ny4yMjg5QzQ2LjI1MDIgNTcuNjEyNSA0NS45Nzk2IDU3LjkzMTUgNDUuNjIxMyA1OC4xMTU3TDQwLjEwMDEgNjAuOTUzMkMzOC4yMzQ0IDYxLjkxMjEgMzUuOTQ0NSA2MS4xNzY5IDM0Ljk4NTcgNTkuMzExMUwyOC43MzU0IDQ3LjE0OTRDMjcuNzc2NiA0NS4yODM2IDI4LjUxMTggNDIuOTkzOCAzMC4zNzc1IDQyLjAzNDlMMzkuODM2NiAzNy4xNzM2QzQxLjcwMjQgMzYuMjE0OCA0My45OTIyIDM2Ljk1IDQ0Ljk1MTEgMzguODE1N0w0OS4xNzc1IDQ3LjAzOTVDNDkuMzYxNyA0Ny4zOTc5IDQ5LjM5NTkgNDcuODE0NyA0OS4yNzI4IDQ4LjE5ODRMNDYuMzczNCA1Ny4yMjg5WiIgZmlsbD0iI0Q5RDlEOSIvPgo8L21hc2s+CjxnIG1hc2s9InVybCgjbWFzazJfNjAyMl81MTczMSkiPgo8cGF0aCBkPSJNNTIuOTM3NSA1NC4zNTU3TDQ2LjE4MSA1Ny44MjgxTDQ0LjA5NzYgNTMuNzc0MkM0My4zMzA1IDUyLjI4MTYgNDMuOTE4NiA1MC40NDk3IDQ1LjQxMTIgNDkuNjgyNkw0OS40NjUxIDQ3LjU5OTJMNTIuOTM3NSA1NC4zNTU3WiIgZmlsbD0iI0U3RTdFNyIvPgo8L2c+CjxwYXRoIGQ9Ik0xMjAuODggMzcuMzg5MkMxMjEuMjAzIDM3LjQ3NTggMTIxLjQ3OSAzNy42ODcyIDEyMS42NDYgMzcuOTc2OUwxMjQuMjIzIDQyLjQzOTlDMTI1LjA5MyA0My45NDgxIDEyNC41NzcgNDUuODc2NiAxMjMuMDY5IDQ2Ljc0NzRMMTEzLjIzOCA1Mi40MjMzQzExMS43MjkgNTMuMjk0IDEwOS44MDEgNTIuNzc3MyAxMDguOTMgNTEuMjY5MUwxMDQuNTE1IDQzLjYyMjhDMTAzLjY0NSA0Mi4xMTQ2IDEwNC4xNjEgNDAuMTg2MSAxMDUuNjcgMzkuMzE1M0wxMTIuMzE3IDM1LjQ3NzNDMTEyLjYwNyAzNS4zMSAxMTIuOTUxIDM1LjI2NDcgMTEzLjI3NCAzNS4zNTEzTDEyMC44OCAzNy4zODkyWiIgZmlsbD0idXJsKCNwYWludDhfbGluZWFyXzYwMjJfNTE3MzEpIi8+CjxtYXNrIGlkPSJtYXNrM182MDIyXzUxNzMxIiBzdHlsZT0ibWFzay10eXBlOmFscGhhIiBtYXNrVW5pdHM9InVzZXJTcGFjZU9uVXNlIiB4PSIxMDQiIHk9IjM1IiB3aWR0aD0iMjEiIGhlaWdodD0iMTgiPgo8cGF0aCBkPSJNMTIwLjg4IDM3LjM4OTJDMTIxLjIwMyAzNy40NzU4IDEyMS40NzkgMzcuNjg3MiAxMjEuNjQ2IDM3Ljk3NjlMMTI0LjIyMyA0Mi40Mzk5QzEyNS4wOTMgNDMuOTQ4MSAxMjQuNTc3IDQ1Ljg3NjYgMTIzLjA2OSA0Ni43NDc0TDExMy4yMzggNTIuNDIzM0MxMTEuNzI5IDUzLjI5NCAxMDkuODAxIDUyLjc3NzMgMTA4LjkzIDUxLjI2OTFMMTA0LjUxNSA0My42MjI4QzEwMy42NDUgNDIuMTE0NiAxMDQuMTYxIDQwLjE4NjEgMTA1LjY3IDM5LjMxNTNMMTEyLjMxNyAzNS40NzczQzExMi42MDcgMzUuMzEgMTEyLjk1MSAzNS4yNjQ3IDExMy4yNzQgMzUuMzUxM0wxMjAuODggMzcuMzg5MloiIGZpbGw9IiNEOUQ5RDkiLz4KPC9tYXNrPgo8ZyBtYXNrPSJ1cmwoI21hc2szXzYwMjJfNTE3MzEpIj4KPHBhdGggZD0iTTExOC4yMzEgMzIuMDYyN0wxMjEuMzg1IDM3LjUyNDRMMTE4LjEwOCAzOS40MTY0QzExNi45MDEgNDAuMTEzIDExNS4zNTggMzkuNjk5NiAxMTQuNjYyIDM4LjQ5M0wxMTIuNzcgMzUuMjE2TDExOC4yMzEgMzIuMDYyN1oiIGZpbGw9IiNCNUVDQ0YiLz4KPC9nPgo8cGF0aCBkPSJNNzMuMzQ4MyA0NS4wOTg0QzczLjM0NzggNDQuODQ2OCA3My40NDczIDQ0LjYwNTMgNzMuNjI0OCA0NC40MjdMNzYuMzYwMyA0MS42ODA1Qzc3LjI4NDcgNDAuNzUyNCA3OC43ODY0IDQwLjc0OTQgNzkuNzE0NSA0MS42NzM4TDg1Ljc2NDMgNDcuNjk5M0M4Ni42OTI0IDQ4LjYyMzcgODYuNjk1NSA1MC4xMjU1IDg1Ljc3MTEgNTEuMDUzNkw4MS4wODQ1IDU1Ljc1OUM4MC4xNjAxIDU2LjY4NzEgNzguNjU4NCA1Ni42OTAxIDc3LjczMDMgNTUuNzY1N0w3My42Mzk0IDUxLjY5MTJDNzMuNDYxMSA1MS41MTM3IDczLjM2MDcgNTEuMjcyNiA3My4zNjAyIDUxLjAyMUw3My4zNDgzIDQ1LjA5ODRaIiBmaWxsPSJ1cmwoI3BhaW50OV9saW5lYXJfNjAyMl81MTczMSkiLz4KPG1hc2sgaWQ9Im1hc2s0XzYwMjJfNTE3MzEiIHN0eWxlPSJtYXNrLXR5cGU6YWxwaGEiIG1hc2tVbml0cz0idXNlclNwYWNlT25Vc2UiIHg9IjczIiB5PSI0MCIgd2lkdGg9IjE0IiBoZWlnaHQ9IjE3Ij4KPHBhdGggZD0iTTczLjM0ODMgNDUuMDk4NEM3My4zNDc4IDQ0Ljg0NjggNzMuNDQ3MyA0NC42MDUzIDczLjYyNDggNDQuNDI3TDc2LjM2MDMgNDEuNjgwNUM3Ny4yODQ3IDQwLjc1MjQgNzguNzg2NCA0MC43NDk0IDc5LjcxNDUgNDEuNjczOEw4NS43NjQzIDQ3LjY5OTNDODYuNjkyNCA0OC42MjM3IDg2LjY5NTUgNTAuMTI1NSA4NS43NzExIDUxLjA1MzZMODEuMDg0NSA1NS43NTlDODAuMTYwMSA1Ni42ODcxIDc4LjY1ODQgNTYuNjkwMSA3Ny43MzAzIDU1Ljc2NTdMNzMuNjM5NCA1MS42OTEyQzczLjQ2MTEgNTEuNTEzNyA3My4zNjA3IDUxLjI3MjYgNzMuMzYwMiA1MS4wMjFMNzMuMzQ4MyA0NS4wOTg0WiIgZmlsbD0iI0Q5RDlEOSIvPgo8L21hc2s+CjxnIG1hc2s9InVybCgjbWFzazRfNjAyMl81MTczMSkiPgo8cGF0aCBkPSJNNzAgNDguMDY2NEw3My4zNDc1IDQ0LjcwNTRMNzUuMzY0MSA0Ni43MTM5Qzc2LjEwNjYgNDcuNDUzNCA3Ni4xMDkgNDguNjU0OCA3NS4zNjk1IDQ5LjM5NzNMNzMuMzYxIDUxLjQxMzlMNzAgNDguMDY2NFoiIGZpbGw9IiMwN0MwNUYiLz4KPC9nPgo8cGF0aCBkPSJNMTA2LjEzOCAxMjAuMTAzQzEwNi4xMzggMTE4Ljk0NiAxMDcuMDc2IDExOC4wMDkgMTA4LjIzMyAxMTguMDA5SDExMy44MTlDMTE0Ljk3NiAxMTguMDA5IDExNS45MTQgMTE4Ljk0NiAxMTUuOTE0IDEyMC4xMDNWMTIwLjEwM0MxMTUuOTE0IDEyMS4yNiAxMTQuOTc2IDEyMi4xOTggMTEzLjgxOSAxMjIuMTk4SDEwOC4yMzNDMTA3LjA3NiAxMjIuMTk4IDEwNi4xMzggMTIxLjI2IDEwNi4xMzggMTIwLjEwM1YxMjAuMTAzWiIgZmlsbD0iIzE0ODVFRSIvPgo8cGF0aCBkPSJNMTIyLjg5NiAxMjAuMTAzQzEyMi44OTYgMTE4Ljk0NiAxMjMuODM0IDExOC4wMDkgMTI0Ljk5MSAxMTguMDA5SDEzMC41NzhDMTMxLjczNCAxMTguMDA5IDEzMi42NzIgMTE4Ljk0NiAxMzIuNjcyIDEyMC4xMDNWMTIwLjEwM0MxMzIuNjcyIDEyMS4yNiAxMzEuNzM0IDEyMi4xOTggMTMwLjU3OCAxMjIuMTk4SDEyNC45OTFDMTIzLjgzNCAxMjIuMTk4IDEyMi44OTYgMTIxLjI2IDEyMi44OTYgMTIwLjEwM1YxMjAuMTAzWiIgZmlsbD0iIzA3QzA1RiIvPgo8cmVjdCB4PSIxMDYuMTM4IiB5PSIxMTcuMzEiIHdpZHRoPSI5Ljc3NTg2IiBoZWlnaHQ9IjQuMTg5NjYiIHJ4PSIyLjA5NDgzIiBmaWxsPSIjNDM5REYxIi8+CjxyZWN0IHg9IjEyMi44OTYiIHk9IjExNy4zMSIgd2lkdGg9IjkuNzc1ODYiIGhlaWdodD0iNC4xODk2NiIgcng9IjIuMDk0ODMiIGZpbGw9IiMzOUNEODAiLz4KPGRlZnM+CjxmaWx0ZXIgaWQ9ImZpbHRlcjBfZF82MDIyXzUxNzMxIiB4PSIxNC40MTM4IiB5PSI3NS4yMDY5IiB3aWR0aD0iMTMyLjY3MiIgaGVpZ2h0PSI4MC4xODU0IiBmaWx0ZXJVbml0cz0idXNlclNwYWNlT25Vc2UiIGNvbG9yLWludGVycG9sYXRpb24tZmlsdGVycz0ic1JHQiI+CjxmZUZsb29kIGZsb29kLW9wYWNpdHk9IjAiIHJlc3VsdD0iQmFja2dyb3VuZEltYWdlRml4Ii8+CjxmZUNvbG9yTWF0cml4IGluPSJTb3VyY2VBbHBoYSIgdHlwZT0ibWF0cml4IiB2YWx1ZXM9IjAgMCAwIDAgMCAwIDAgMCAwIDAgMCAwIDAgMCAwIDAgMCAwIDEyNyAwIiByZXN1bHQ9ImhhcmRBbHBoYSIvPgo8ZmVPZmZzZXQgZHk9IjIuNzkzMSIvPgo8ZmVHYXVzc2lhbkJsdXIgc3RkRGV2aWF0aW9uPSIyLjc5MzEiLz4KPGZlQ29tcG9zaXRlIGluMj0iaGFyZEFscGhhIiBvcGVyYXRvcj0ib3V0Ii8+CjxmZUNvbG9yTWF0cml4IHR5cGU9Im1hdHJpeCIgdmFsdWVzPSIwIDAgMCAwIDAuMTkyNjkxIDAgMCAwIDAgMC4xOTI2OTEgMCAwIDAgMCAwLjE5MjY5MSAwIDAgMCAwLjEgMCIvPgo8ZmVCbGVuZCBtb2RlPSJub3JtYWwiIGluMj0iQmFja2dyb3VuZEltYWdlRml4IiByZXN1bHQ9ImVmZmVjdDFfZHJvcFNoYWRvd182MDIyXzUxNzMxIi8+CjxmZUJsZW5kIG1vZGU9Im5vcm1hbCIgaW49IlNvdXJjZUdyYXBoaWMiIGluMj0iZWZmZWN0MV9kcm9wU2hhZG93XzYwMjJfNTE3MzEiIHJlc3VsdD0ic2hhcGUiLz4KPC9maWx0ZXI+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQwX2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iODAuNzUiIHkxPSI3OCIgeDI9IjgwLjc1IiB5Mj0iMTMyIiBncmFkaWVudFVuaXRzPSJ1c2VyU3BhY2VPblVzZSI+CjxzdG9wIHN0b3AtY29sb3I9IiNFNEY5RUUiLz4KPHN0b3Agb2Zmc2V0PSIxIiBzdG9wLWNvbG9yPSIjOUVERUJEIi8+CjwvbGluZWFyR3JhZGllbnQ+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQxX2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iMjguNjg3NSIgeTE9Ijc3LjYyNSIgeDI9IjI4LjY4NzUiIHkyPSIxMTEuMzc1IiBncmFkaWVudFVuaXRzPSJ1c2VyU3BhY2VPblVzZSI+CjxzdG9wIHN0b3AtY29sb3I9IiNEQkZBRTkiLz4KPHN0b3Agb2Zmc2V0PSIxIiBzdG9wLWNvbG9yPSIjMkNEODdFIi8+CjwvbGluZWFyR3JhZGllbnQ+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQyX2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iMTMzLjQzOCIgeTE9Ijc4IiB4Mj0iMTMzLjQzOCIgeTI9IjExMS43NSIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjREJGQUU5Ii8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0iIzJDRDg3RSIvPgo8L2xpbmVhckdyYWRpZW50Pgo8bGluZWFyR3JhZGllbnQgaWQ9InBhaW50M19saW5lYXJfNjAyMl81MTczMSIgeDE9Ijc5LjMxMjUiIHkxPSIxMDYuMzEyIiB4Mj0iNzkuMzEyNSIgeTI9IjQ4LjkzNzUiIGdyYWRpZW50VW5pdHM9InVzZXJTcGFjZU9uVXNlIj4KPHN0b3Agc3RvcC1jb2xvcj0iIzgzQzFGQSIvPgo8c3RvcCBvZmZzZXQ9IjEiIHN0b3AtY29sb3I9IiM4M0MxRkEiIHN0b3Atb3BhY2l0eT0iMCIvPgo8L2xpbmVhckdyYWRpZW50Pgo8bGluZWFyR3JhZGllbnQgaWQ9InBhaW50NF9saW5lYXJfNjAyMl81MTczMSIgeDE9IjgxIiB5MT0iMTExLjM3NSIgeDI9IjgxIiB5Mj0iMTQ2LjYzOCIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjRjNGRkY3Ii8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0id2hpdGUiLz4KPC9saW5lYXJHcmFkaWVudD4KPGxpbmVhckdyYWRpZW50IGlkPSJwYWludDVfbGluZWFyXzYwMjJfNTE3MzEiIHgxPSI0Ny4xODE4IiB5MT0iMTYuMzIiIHgyPSI2OS41MTIxIiB5Mj0iMjQuMDMzNiIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjMDdDMDVGIi8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0iIzA3QzA1RiIgc3RvcC1vcGFjaXR5PSIwIi8+CjwvbGluZWFyR3JhZGllbnQ+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQ2X2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iOTAuODczNiIgeTE9IjE1LjM3NTYiIHgyPSI5OC43NDk0IiB5Mj0iMzEuNDEzOSIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjQzVDNUM1Ii8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0iI0M1QzVDNSIgc3RvcC1vcGFjaXR5PSIwIi8+CjwvbGluZWFyR3JhZGllbnQ+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQ3X2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iNDQuODI5NyIgeTE9IjU4LjUyMjUiIHgyPSIzNS4xMDcxIiB5Mj0iMzkuNjA0MyIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjQzVDNUM1Ii8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0iI0M1QzVDNSIgc3RvcC1vcGFjaXR5PSIwIi8+CjwvbGluZWFyR3JhZGllbnQ+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQ4X2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iMTIxLjczMiIgeTE9IjM4LjEyNjIiIHgyPSIxMDUuOTc0IiB5Mj0iNDUuODI0NCIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjNTRFODlBIi8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0iIzA3QzA1RiIgc3RvcC1vcGFjaXR5PSIwLjEiLz4KPC9saW5lYXJHcmFkaWVudD4KPGxpbmVhckdyYWRpZW50IGlkPSJwYWludDlfbGluZWFyXzYwMjJfNTE3MzEiIHgxPSI3My42NTYyIiB5MT0iNDQuMzk1NSIgeDI9IjgyLjI0NDYiIHkyPSI1NC4zNzMxIiBncmFkaWVudFVuaXRzPSJ1c2VyU3BhY2VPblVzZSI+CjxzdG9wIHN0b3AtY29sb3I9IiM2RUUxQTUiLz4KPHN0b3Agb2Zmc2V0PSIxIiBzdG9wLWNvbG9yPSIjNkVFMUE1IiBzdG9wLW9wYWNpdHk9IjAiLz4KPC9saW5lYXJHcmFkaWVudD4KPC9kZWZzPgo8L3N2Zz4K';

/* 三星闪光装饰（Vue 模板内联 svg，AgentList.vue:15-31）。 */
function SparklesIcon({ size }: { size: number }) {
  return (
    <svg className="sparkles-icon" width={size} height={size} viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M10 3L10.8 6.2C10.9 6.7 11.3 7.1 11.8 7.2L15 8L11.8 8.8C11.3 8.9 10.9 9.3 10.8 9.8L10 13L9.2 9.8C9.1 9.3 8.7 8.9 8.2 8.8L5 8L8.2 7.2C8.7 7.1 9.1 6.7 9.2 6.2L10 3Z" fill="currentColor" stroke="currentColor" strokeWidth="0.8" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M15.5 4L15.8 5.2C15.85 5.45 16.05 5.65 16.3 5.7L17.5 6L16.3 6.3C16.05 6.35 15.85 6.55 15.8 6.8L15.5 8L15.2 6.8C15.15 6.55 14.95 6.35 14.7 6.3L13.5 6L14.7 5.7C14.95 5.65 15.15 5.45 15.2 5.2L15.5 4Z" fill="currentColor" stroke="currentColor" strokeWidth="0.6" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M4.5 13L4.8 14.2C4.85 14.45 5.05 14.65 5.3 14.7L6.5 15L5.3 15.3C5.05 15.35 4.85 15.55 4.8 15.8L4.5 17L4.2 15.8C4.15 15.55 3.95 15.35 3.7 15.3L2.5 15L3.7 14.7C3.95 14.65 4.15 14.45 4.2 14.2L4.5 13Z" fill="currentColor" stroke="currentColor" strokeWidth="0.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

/* 卡片装饰双星（Vue 模板内联 svg，AgentList.vue:163-178）。 */
function CardDecoration() {
  return (
    <div className="card-decoration" aria-hidden="true">
      <svg className="star-icon" width="24" height="24" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M10 3L10.8 6.2C10.9 6.7 11.3 7.1 11.8 7.2L15 8L11.8 8.8C11.3 8.9 10.9 9.3 10.8 9.8L10 13L9.2 9.8C9.1 9.3 8.7 8.9 8.2 8.8L5 8L8.2 7.2C8.7 7.1 9.1 6.7 9.2 6.2L10 3Z" stroke="currentColor" strokeWidth="0.8" strokeLinecap="round" strokeLinejoin="round" fill="currentColor" fillOpacity="0.15" />
      </svg>
      <svg className="star-icon small" width="14" height="14" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M10 3L10.8 6.2C10.9 6.7 11.3 7.1 11.8 7.2L15 8L11.8 8.8C11.3 8.9 10.9 9.3 10.8 9.8L10 13L9.2 9.8C9.1 9.3 8.7 8.9 8.2 8.8L5 8L8.2 7.2C8.7 7.1 9.1 6.7 9.2 6.2L10 3Z" stroke="currentColor" strokeWidth="0.8" strokeLinecap="round" strokeLinejoin="round" fill="currentColor" fillOpacity="0.15" />
      </svg>
    </div>
  );
}

/* 网络搜索能力徽章内联 svg（AgentList.vue:266-271）。 */
function WebSearchBadgeIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
      <circle cx="8" cy="8" r="6" stroke="currentColor" strokeWidth="1.2" fill="none" />
      <ellipse cx="8" cy="8" rx="2.5" ry="6" stroke="currentColor" strokeWidth="1.2" fill="none" />
      <line x1="2" y1="6" x2="14" y2="6" stroke="currentColor" strokeWidth="1.2" />
      <line x1="2" y1="10" x2="14" y2="10" stroke="currentColor" strokeWidth="1.2" />
    </svg>
  );
}

// --- AgentAvatar.vue 端口（name 哈希渐变 + 首字母 + 双星装饰） ---------------------

export function AgentAvatar({ name, size = 'medium' }: { name: string; size?: 'small' | 'medium' | 'large' }) {
  const g = avatarGradient(name || '');
  return (
    <div
      className={`agent-avatar${size === 'small' ? ' agent-avatar-small' : size === 'large' ? ' agent-avatar-large' : ''}`}
      style={{ background: `linear-gradient(135deg, ${g.from} 0%, ${g.to} 100%)` }}
    >
      <svg className="agent-sparkles" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
        <path d="M24 5L24.4 6.6C24.45 6.85 24.65 7.05 24.9 7.1L26.5 7.5L24.9 7.9C24.65 7.95 24.45 8.15 24.4 8.4L24 10L23.6 8.4C23.55 8.15 23.35 7.95 23.1 7.9L21.5 7.5L23.1 7.1C23.35 7.05 23.55 6.85 23.6 6.6L24 5Z" fill="rgba(255,255,255,0.6)" />
        <path d="M7 22L7.4 23.6C7.45 23.85 7.65 24.05 7.9 24.1L9.5 24.5L7.9 24.9C7.65 24.95 7.45 25.15 7.4 25.4L7 27L6.6 25.4C6.55 25.15 6.35 24.95 6.1 24.9L4.5 24.5L6.1 24.1C6.35 24.05 6.55 23.85 6.6 23.6L7 22Z" fill="rgba(255,255,255,0.5)" />
      </svg>
      <span className="agent-avatar-letter" style={{ textShadow: `0 1px 2px ${g.to}80, 0 0 8px ${g.from}30` }}>{avatarLetter(name || '')}</span>
    </div>
  );
}

// --- SpaceAvatar.vue 端口（ListSpaceSidebar 组织条目用） ---------------------------

const SPACE_GRADIENTS: ReadonlyArray<{ from: string; to: string }> = [
  { from: '#07c05f', to: '#059669' },
  { from: '#11998e', to: '#38ef7d' },
  { from: '#43e97b', to: '#38f9d7' },
  { from: '#02aab0', to: '#00cdac' },
  { from: '#36d1dc', to: '#5b86e5' },
  { from: '#4facfe', to: '#00f2fe' },
  { from: '#667eea', to: '#764ba2' },
  { from: '#4776e6', to: '#8e54e9' },
  { from: '#56ab2f', to: '#a8e063' },
  { from: '#00b09b', to: '#96c93d' },
  { from: '#5ee7df', to: '#b490ca' },
  { from: '#614385', to: '#516395' },
];

function spaceGradient(name: string) {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    const char = name.charCodeAt(i);
    hash = ((hash << 5) - hash) + char;
    hash = hash & hash;
  }
  return SPACE_GRADIENTS[Math.abs(hash) % SPACE_GRADIENTS.length]!;
}

export function SpaceAvatar({ name, size = 'medium' }: { name: string; size?: 'small' | 'medium' | 'large' }) {
  const g = spaceGradient(name || '');
  const first = (name || '').trim().charAt(0);
  const letter = !first ? '?' : /[a-zA-Z]/.test(first) ? first.toUpperCase() : first;
  return (
    <div
      className={`space-avatar${size === 'small' ? ' space-avatar-small' : size === 'large' ? ' space-avatar-large' : ''}`}
      style={{ background: `linear-gradient(135deg, ${g.from} 0%, ${g.to} 100%)` }}
    >
      <svg className="space-avatar-decoration" viewBox="0 0 56 40" preserveAspectRatio="xMaxYMax meet" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
        <circle cx="10" cy="12" r="4" stroke="currentColor" strokeWidth="1.5" fill="none" opacity="0.5" />
        <circle cx="28" cy="8" r="5" stroke="currentColor" strokeWidth="1.8" fill="none" opacity="0.7" />
        <circle cx="46" cy="14" r="4" stroke="currentColor" strokeWidth="1.5" fill="none" opacity="0.5" />
        <path d="M14 13 L24 10 M32 10 L42 13" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" opacity="0.4" />
        <circle cx="28" cy="28" r="6" stroke="currentColor" strokeWidth="1.2" fill="none" opacity="0.35" />
        <path d="M28 14 L28 22 M20 18 L26 24 M36 18 L30 24" stroke="currentColor" strokeWidth="1" strokeLinecap="round" opacity="0.3" />
      </svg>
      <span className="space-avatar-letter" style={{ textShadow: `0 1px 2px ${g.to}80, 0 0 8px ${g.from}30` }}>{letter}</span>
    </div>
  );
}

// --- ResourceOriginBadge.vue 端口 ---------------------------------------------------

const ORIGIN_ICON: Record<string, string> = { mine: 'user', tenant: 'usergroup', creator: 'user', space: 'building', shared: 'share' };

export function ResourceOriginBadge({ variant, creatorName, tenantName }: {
  variant: 'mine' | 'tenant' | 'creator' | 'space' | 'shared';
  creatorName?: string;
  tenantName?: string;
}) {
  const text = variant === 'mine' ? formatMessage('zh-CN', 'resourceOrigin.mine')
    : variant === 'creator' ? (creatorName || formatMessage('zh-CN', 'resourceOrigin.tenant'))
      : variant === 'space' ? (tenantName || formatMessage('zh-CN', 'resourceOrigin.space'))
        : variant === 'shared' ? (tenantName || formatMessage('zh-CN', 'resourceOrigin.shared'))
          : (tenantName || formatMessage('zh-CN', 'resourceOrigin.tenant'));
  return (
    <span className={`resource-origin-badge origin-${variant}`}>
      <TIcon name={ORIGIN_ICON[variant] ?? 'usergroup'} size="12px" className="badge-icon" />
      <span className="badge-text">{text}</span>
    </span>
  );
}

// --- 左侧空间栏（ListSpaceSidebar.vue collapsed 条带 1:1） -------------------------

export interface AgentRailItem {
  key: string;
  label: string;
  icon: 'layers' | 'star' | 'history' | 'workspace' | 'space';
  avatarName?: string;
  count: number | undefined;
  active: boolean;
}

const RAIL_ICON_NAME: Record<AgentRailItem['icon'], string> = {
  layers: 'layers',
  star: 'star',
  history: 'history',
  /* Vue workspace 条目是 t-icon system-sum（ListSpaceSidebar.vue:31）。 */
  workspace: 'system-sum',
  space: 'layers',
};

/* ListSpaceSidebar truncateLabel：折叠条标签 ~4 个 CJK 字符宽度。 */
function truncateLabel(text: string, max = 4): string {
  return !text ? '' : text.length > max ? `${text.slice(0, max)}…` : text;
}

export function AgentRail({ t, items, onSelect }: { t: Translate; items: AgentRailItem[]; onSelect: (key: string) => void }) {
  const tooltipText = (name: string, count?: number) => (count === undefined ? name : `${name} (${count})`);
  const orgItems = items.filter((item) => item.icon === 'space');
  const baseItems = items.filter((item) => item.icon !== 'space');
  return (
    <div className="list-space-sidebar">
      <div className="icon-strip">
        {baseItems.map((item) => (
          <Tooltip key={item.key} content={tooltipText(item.label, item.count)} placement="right" showArrow={false}>
            <div
              className={`icon-item-labeled${item.key === 'mine' ? ' workspace-item' : ''}${item.active ? ' active' : ''}`}
              data-space-key={item.key}
              onClick={() => onSelect(item.key)}
            >
              <TIcon name={RAIL_ICON_NAME[item.icon]} size="16px" />
              <span className="icon-label">{item.label}</span>
            </div>
          </Tooltip>
        ))}
        {orgItems.length > 0 ? <div className="icon-strip-divider" /> : null}
        {orgItems.map((item) => (
          <Tooltip key={item.key} content={tooltipText(item.label, item.count)} placement="right" showArrow={false}>
            <div
              className={`icon-item-labeled${item.active ? ' active' : ''}`}
              data-space-key={item.key}
              onClick={() => onSelect(item.key)}
            >
              <SpaceAvatar name={item.avatarName ?? item.label} size="small" />
              <span className="icon-label">{truncateLabel(item.label)}</span>
            </div>
          </Tooltip>
        ))}
      </div>
      <div className="resize-handle" aria-hidden="true">
        <div className="resize-handle-line" />
      </div>
    </div>
  );
}

// --- 分组标题（.agent-section-header） ----------------------------------------------

const SECTION_SUBICONS: Partial<Record<string, string>> = { sharedEditable: 'edit-1', sharedReadonly: 'browse' };

function AgentSectionHeader({ section, t, viewer, collapsed, onToggle }: {
  section: AgentSectionView; t: Translate; viewer: AgentViewer; collapsed: boolean; onToggle: (key: string) => void;
}) {
  const subIcon = SECTION_SUBICONS[section.key];
  return (
    <div
      className="agent-section-header"
      role="button"
      tabIndex={0}
      data-agent-section={section.key}
      onClick={() => onToggle(section.key)}
      onKeyDown={(event) => {
        if (event.key === 'Enter') { event.preventDefault(); onToggle(section.key); }
        if (event.key === ' ') { event.preventDefault(); onToggle(section.key); }
      }}
    >
      <TIcon name={SECTION_ICON_KEYS[section.key]} size="14px" />
      {subIcon ? <TIcon name={subIcon} size="12px" className="agent-section-subicon" /> : null}
      <span>{t(agentSectionLabelKey(section.key, viewer))}</span>
      <span className="agent-section-count">{section.count}</span>
      {/* Vue：t-icon 直接携带 agent-section-toggle 类（无包装 span，避免多余行框） */}
      <TIcon className="agent-section-toggle" name={collapsed ? 'chevron-right' : 'chevron-down'} size="14px" data-agent-section-toggle={section.key} />
    </div>
  );
}

// --- 卡片 ----------------------------------------------------------------------------

const ACTION_META: Record<AgentCardAction, { icon: string; labelKey: string }> = {
  edit: { icon: 'edit', labelKey: 'common.edit' },
  copy: { icon: 'file-copy', labelKey: 'common.copy' },
  toggle: { icon: 'poweroff', labelKey: 'agent.disable' },
  delete: { icon: 'delete', labelKey: 'common.delete' },
};

function featureBadgeIcon(badge: string): { icon: string; size: string } | null {
  if (badge === 'webSearch') return null;
  const modeBadge = badge === 'modeNormal' || badge === 'modeAgent' || badge === 'modePlain';
  const icon = badge === 'modeAgent' ? 'control-platform' : badge === 'knowledge' ? 'folder'
    : badge === 'mcp' ? 'extension' : badge === 'multiTurn' ? 'chat-bubble' : 'chat';
  return { icon, size: modeBadge ? '14px' : '16px' };
}

export function AgentCard({ agent, t, viewer, favorited, menuOpen, hidden = false, onOpen, onToggleFavorite, onToggleMenu, onMenuAction }: {
  agent: AgentCardModel;
  t: Translate;
  viewer: AgentViewer;
  favorited: boolean;
  menuOpen: boolean;
  /** Vue v-show="!isAgentRowHidden(agent)"：折叠组保留 DOM 仅隐藏（§3.2）。 */
  hidden?: boolean;
  onOpen: (agent: AgentCardModel) => void;
  onToggleFavorite: (id: string) => void;
  onToggleMenu: (id: string | null) => void;
  onMenuAction: (action: AgentCardAction, agent: AgentCardModel) => void;
}) {
  /* Vue :class 绑定逐字复刻：mode class 只在值精确匹配时出现（builtin 快速问答
   * 的 agent_mode 为空串 → 无 mode class → 白底）。 */
  const rawMode = agent.config?.agent_mode;
  const mode = rawMode === 'smart-reasoning' ? 'agent' : 'normal';
  const modeClass = rawMode === 'quick-answer' ? ' agent-mode-normal' : rawMode === 'smart-reasoning' ? ' agent-mode-agent' : '';
  const badges = featureBadges(agent);
  const expertBadgeId = expertSourceId(agent.config);
  const actions = cardActions(agent, viewer);
  const badge = cornerBadge(agent, viewer.userId);
  const modeTitleKey = agent.config?.agent_mode === 'smart-reasoning' ? 'agent.mode.agent' : 'agent.mode.normal';
  /* 空间视角卡片（sharedByMe / 空间 tab）右下只有徽章，无来源 pill（Vue :546-635）。 */
  const showSourcePill = !agent.isMine && !agent.sharedByMe;
  return (
    <div
      className={`agent-card${agent.is_builtin ? ' is-builtin' : ''}${modeClass}${agent.isMine ? '' : ' shared-agent-card'} wk-agent-card`}
      data-agent-id={agent.id}
      style={hidden ? { display: 'none' } : undefined}
      onClick={() => onOpen(agent)}
    >
      <CardDecoration />
      <button
        type="button"
        className={`agent-favorite-star${favorited ? ' is-favorited' : ''}`}
        aria-pressed={favorited ? 'true' : 'false'}
        aria-label={t('common.favorite')}
        onClick={(event) => { event.stopPropagation(); onToggleFavorite(agent.id); }}
      >
        <TIcon name={favorited ? 'star-filled' : 'star'} size="14px" />
      </button>
      <div className="card-header">
        <div className="card-header-left">
          {agent.is_builtin ? (
            <div className={`builtin-avatar ${mode === 'agent' ? 'agent' : 'normal'}`}>
              <TIcon name={mode === 'agent' ? 'control-platform' : 'chat'} size="18px" />
            </div>
          ) : agent.avatar ? (
            <div className="builtin-avatar agent-emoji">{agent.avatar}</div>
          ) : (
            <AgentAvatar name={agent.name} size="small" />
          )}
          <span className="card-title" title={agent.name}>{agent.name}</span>
        </div>
        {actions.length > 0 ? (
          <Popup
            visible={menuOpen}
            trigger="hover"
            overlayClassName="card-more-popup"
            destroyOnClose
            placement="bottom-right"
            onVisibleChange={(visible) => { if (!visible) onToggleMenu(null); }}
            content={(
              <div className="popup-menu">
                {actions.map((action) => (
                  <div
                    key={action}
                    className={`popup-menu-item${action === 'delete' ? ' delete' : ''}`}
                    data-action={action}
                    onClick={() => onMenuAction(action, agent)}
                  >
                    <TIcon className="menu-icon" name={ACTION_META[action]!.icon} />
                    <span>{t(action === 'toggle' && agent.disabledByMe ? 'agent.enable' : ACTION_META[action]!.labelKey)}</span>
                  </div>
                ))}
              </div>
            )}
          >
            <div
              className={`more-wrap${menuOpen ? ' active-more' : ''}`}
              aria-label={t('agent.manageAgents')}
              aria-haspopup="menu"
              aria-expanded={menuOpen ? 'true' : 'false'}
              onClick={(event) => { event.stopPropagation(); onToggleMenu(menuOpen ? null : agent.id); }}
            >
              <img className="more-icon" src={MORE_PNG} alt="" />
            </div>
          </Popup>
        ) : null}
      </div>
      <div className="card-content">
        <div className="card-description">{agent.description || t('agent.noDescription')}</div>
      </div>
      <div className="card-bottom">
        <div className="bottom-left">
          <div className="feature-badges">
            {agent.disabledByMe ? <Tag theme="default" size="small" className="disabled-badge">{t('agent.disabled')}</Tag> : null}
            {badges.map((key) => {
              const iconDef = featureBadgeIcon(key);
              const badgeTitle = key === 'modeNormal' || key === 'modeAgent' || key === 'modePlain' ? t(modeTitleKey) : t(FEATURE_BADGE_TITLE_KEYS[key]!);
              return (
                <Tooltip key={key} content={badgeTitle} placement="top">
                  <div
                    className={`feature-badge${key === 'modeNormal' ? ' mode-normal' : key === 'modeAgent' ? ' mode-agent' : key === 'webSearch' ? ' web-search' : key === 'knowledge' ? ' knowledge' : key === 'mcp' ? ' mcp' : key === 'multiTurn' ? ' multi-turn' : ''}`}
                    data-feature-badge={key}
                  >
                    {iconDef ? <TIcon name={iconDef.icon} size={iconDef.size} /> : <WebSearchBadgeIcon />}
                  </div>
                </Tooltip>
              );
            })}
          </div>
        </div>
        {showSourcePill ? (
          <div className="card-bottom-source wk-agent-card-source">
            <OrgGreenIcon />
            <span className="org-source-text">{agent.orgName}</span>
          </div>
        ) : badge ? (
          badge.kind === 'builtin' ? (
            <div className="builtin-badge">
              <TIcon name="lock-on" size="12px" />
              <span>{t('agent.builtin')}</span>
            </div>
          ) : (
            <ResourceOriginBadge variant="creator" creatorName={badge.name} />
          )
        ) : expertBadgeId ? (
          /* Octop M2 溯源徽章（React 侧增量，Vue 无对应分支；样式走平移 CSS 同款 pill） */
          <div className="builtin-badge" data-agent-expert-badge>{t('experts.badge', { expertId: expertBadgeId })}</div>
        ) : null}
      </div>
    </div>
  );
}

// --- 共享详情抽屉（AgentList.vue shared-detail-drawer unscoped 块 1:1） ---------------

function scopeText(scope: ReturnType<typeof kbScope>, t: Translate, allKey: string, selectedKey: string, noneKey: string): string {
  if (scope.kind === 'all') return t(allKey);
  if (scope.kind === 'selected') return t(selectedKey, { count: scope.count });
  return t(noneKey);
}

export function AgentDetailDrawer({ kind, agent, t, onClose, onUseInChat }: {
  kind: 'shared';
  agent: AgentCardModel;
  t: Translate;
  onClose: () => void;
  onUseInChat: (agent: AgentCardModel) => void;
}) {
  const config = agent.config;
  const usesKb = config ? config.kb_selection_mode !== 'none' && config.kb_selection_mode !== undefined : false;
  return (
    <div className="shared-detail-drawer-overlay" onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}>
      <div className="shared-detail-drawer">
        <div className="shared-detail-drawer-header">
          <h3 className="shared-detail-drawer-title">{t('agent.detail.title')}</h3>
          <button type="button" className="shared-detail-drawer-close" aria-label={t('general.close')} onClick={onClose}>
            <TIcon name="close" />
          </button>
        </div>
        <div className="shared-detail-drawer-body">
          <div className="shared-detail-row">
            <span className="shared-detail-label">{t('agent.editor.name')}</span>
            <span className="shared-detail-value">{agent.name}</span>
          </div>
          <div className="shared-detail-row">
            <span className="shared-detail-label">{t('knowledgeList.detail.sourceOrg')}</span>
            <span className="shared-detail-value shared-detail-org">
              <OrgGreenIcon />
              <span>{agent.orgName}</span>
            </span>
          </div>
          <div className="shared-detail-row">
            <span className="shared-detail-label">{t('knowledgeList.detail.myPermission')}</span>
            <span className="shared-detail-value">{t('organization.share.permissionReadonly')}</span>
          </div>
          {config ? (
            <>
              <div className="shared-detail-section-title">{t('agent.shareScope.title')}</div>
              <div className="shared-detail-row">
                <span className="shared-detail-label">{t('agent.shareScope.knowledgeBase')}</span>
                <span className="shared-detail-value">{scopeText(kbScope(config), t, 'agent.shareScope.kbAll', 'agent.shareScope.kbSelected', 'agent.shareScope.kbNone')}</span>
              </div>
              <div className="shared-detail-row">
                <span className="shared-detail-label">{t('agent.shareScope.chatModel')}</span>
                <span className="shared-detail-value">{config.model_id ? t('agent.shareScope.modelConfigured') : t('agent.shareScope.modelNotSet')}</span>
              </div>
              {usesKb ? (
                <div className="shared-detail-row">
                  <span className="shared-detail-label">{t('agent.shareScope.rerankModel')}</span>
                  <span className="shared-detail-value">{config.rerank_model_id ? t('agent.shareScope.modelConfigured') : t('agent.shareScope.modelNotSet')}</span>
                </div>
              ) : null}
              <div className="shared-detail-row">
                <span className="shared-detail-label">{t('agent.shareScope.webSearch')}</span>
                <span className="shared-detail-value">{config.web_search_enabled ? t('agent.shareScope.enabled') : t('agent.shareScope.disabled')}</span>
              </div>
              <div className="shared-detail-row">
                <span className="shared-detail-label">{t('agent.shareScope.mcp')}</span>
                <span className="shared-detail-value">{scopeText(mcpScope(config), t, 'agent.shareScope.mcpAll', 'agent.shareScope.mcpSelected', 'agent.shareScope.mcpNone')}</span>
              </div>
            </>
          ) : null}
        </div>
        <div className="shared-detail-drawer-footer">
          <Button theme="primary" block onClick={() => onUseInChat(agent)}>{t('agent.detail.useInChat')}</Button>
        </div>
      </div>
    </div>
  );
}

// --- 删除确认（AgentList.vue del-agent-dialog 1:1） -----------------------------------

export function AgentDeleteDialog({ agent, t, busy, onConfirm, onCancel }: {
  agent: AgentCardModel | null; t: Translate; busy: boolean; onConfirm: () => void; onCancel: () => void;
}) {
  return (
    <Dialog
      visible={agent !== null}
      dialogClassName="del-agent-dialog"
      closeBtn={false}
      cancelBtn={null}
      confirmBtn={null}
      onClose={onCancel}
    >
      {agent ? (
        <div className="circle-wrap">
          <div className="dialog-header">
            <img className="circle-img" src={CIRCLE_PNG} alt="" />
            <span className="circle-title">{t('agent.delete.confirmTitle')}</span>
          </div>
          <span className="del-circle-txt">{t('agent.delete.confirmMessage', { name: agent.name })}</span>
          <div className="circle-btn">
            <span className="circle-btn-txt" onClick={onCancel}>{t('common.cancel')}</span>
            <span className="circle-btn-txt confirm" onClick={() => { if (!busy) onConfirm(); }}>{busy ? t('common.loading') : t('agent.delete.confirmButton')}</span>
          </div>
        </div>
      ) : null}
    </Dialog>
  );
}

// --- 页面视图 --------------------------------------------------------------------------

export interface AgentsPageViewProps {
  t: Translate;
  editorT: Translate;
  client: WeKnoraClient;
  viewer: AgentViewer;
  loading: boolean;
  space: string;
  /** Space tab fetch in flight (Vue AgentList.vue spaceAgentsLoading). */
  spaceLoading?: boolean;
  rail: AgentRailItem[];
  sections: AgentSectionView[];
  flatCards: AgentCardModel[];
  isSectioned: boolean;
  favorites: ReadonlySet<string>;
  openMenuId: string | null;
  error: string | null;
  notice: string | null;
  drawer: { kind: 'shared'; agent: AgentCardModel } | null;
  editor: { mode: 'create' | 'edit'; agent: AgentCardModel | null; initialSection?: string; initialHighlight?: string; readOnly?: boolean } | null;
  deleteTarget: AgentCardModel | null;
  deleting: boolean;
  collapsedSections: ReadonlySet<string>;
  canCreate: boolean;
  onSpaceChange: (key: string) => void;
  onOpenCard: (agent: AgentCardModel) => void;
  onToggleFavorite: (id: string) => void;
  onToggleMenu: (id: string | null) => void;
  onMenuAction: (action: AgentCardAction, agent: AgentCardModel) => void;
  onCreate: () => void;
  onCloseDrawer: () => void;
  onUseInChat: (agent: AgentCardModel) => void;
  onCloseEditor: () => void;
  onEditorSaved: (agent: Record<string, unknown>, mode: 'create' | 'edit') => void;
  onDeleteConfirm: () => void;
  onDeleteCancel: () => void;
  onToggleSection: (key: string) => void;
}

/* 骨架屏（Vue t-skeleton rowCol 三段，AgentList.vue:44-60）。 */
function SkeletonCards() {
  return (
    <div className="agent-card-wrap" aria-busy="true">
      {Array.from({ length: 6 }, (_, index) => (
        <div className="agent-card agent-card-skeleton" key={`skel-${index}`}>
          <div className="card-header">
            <div className="card-header-left">
              <Skeleton animation="gradient" rowCol={[[{ width: '32px', height: '32px', type: 'circle' }, { width: '40%', height: '18px' }]]} />
            </div>
          </div>
          <div className="card-content">
            <Skeleton animation="gradient" rowCol={[{ width: '100%', height: '14px' }, { width: '70%', height: '14px' }]} />
          </div>
          <div className="card-bottom">
            <Skeleton animation="gradient" rowCol={[[{ width: '60px', height: '22px', type: 'rect' }, { width: '60px', height: '22px', type: 'rect' }]]} />
          </div>
        </div>
      ))}
    </div>
  );
}

function EmptyState({ t, space, canCreate, onCreate }: { t: Translate; space: string; canCreate: boolean; onCreate: () => void }) {
  if (space === 'favorites') {
    return (
      <div className="empty-state">
        <TIcon name="star" size="48px" className="empty-icon" />
        <span className="empty-txt">{t('agent.empty.favoritesTitle')}</span>
        <span className="empty-desc">{t('agent.empty.favoritesDescription')}</span>
      </div>
    );
  }
  if (space === 'recents') {
    return (
      <div className="empty-state">
        <TIcon name="history" size="48px" className="empty-icon" />
        <span className="empty-txt">{t('agent.empty.recentsTitle')}</span>
        <span className="empty-desc">{t('agent.empty.recentsDescription')}</span>
      </div>
    );
  }
  if (space === 'all' || space === 'mine') {
    return (
      <div className="empty-state">
        <img className="empty-img" src={UPLOAD_SVG} alt="" />
        <span className="empty-txt">{t('agent.empty.title')}</span>
        <span className="empty-desc">{t('agent.empty.description')}</span>
        {canCreate ? (
          <Button className="agent-create-btn empty-state-btn" data-guide="agent-list-create" onClick={onCreate} icon={<span className="btn-icon-wrapper"><SparklesIcon size={18} /></span>}>
            <span>{t('agent.createAgent')}</span>
          </Button>
        ) : null}
      </div>
    );
  }
  return (
    <div className="empty-state">
      <img className="empty-img" src={UPLOAD_SVG} alt="" />
      <span className="empty-txt">{t('agent.empty.sharedTitle')}</span>
      <span className="empty-desc">{t('agent.empty.sharedDescription')}</span>
    </div>
  );
}

function AgentSection({ section, t, viewer, collapsed, onToggle, cardProps }: {
  section: AgentSectionView; t: Translate; viewer: AgentViewer; collapsed: boolean;
  onToggle: (key: string) => void;
  cardProps: (agent: AgentCardModel) => Parameters<typeof AgentCard>[0];
}) {
  return (
    <>
      <AgentSectionHeader section={section} t={t} viewer={viewer} collapsed={collapsed} onToggle={onToggle} />
      {section.cards.map((agent) => (
        <AgentCard key={agent.id} {...cardProps(agent)} hidden={collapsed} />
      ))}
    </>
  );
}

export function AgentsPageView(props: AgentsPageViewProps) {
  const { t, editorT, viewer, loading, space, spaceLoading, rail, sections, flatCards, isSectioned, favorites, openMenuId, drawer, editor, deleteTarget, deleting, collapsedSections, canCreate } = props;
  const cardProps = (agent: AgentCardModel) => ({
    agent, t, viewer,
    favorited: favorites.has(agent.id),
    menuOpen: openMenuId === agent.id,
    onOpen: props.onOpenCard,
    onToggleFavorite: props.onToggleFavorite,
    onToggleMenu: props.onToggleMenu,
    onMenuAction: props.onMenuAction,
  });
  const hasCards = isSectioned ? sections.length > 0 : flatCards.length > 0;
  const showSkeleton = loading && flatCards.length === 0 && sections.length === 0;
  return (
    <div className="agent-list-container">
      <AgentRail t={t} items={rail} onSelect={props.onSpaceChange} />
      <div className="agent-list-content">
        <div className="header" style={{ '--wails-draggable': 'drag' } as CSSProperties}>
          <div className="header-title" style={{ '--wails-draggable': 'drag' } as CSSProperties}>
            <div className="title-row" style={{ '--wails-draggable': 'drag' } as CSSProperties}>
              <h2 style={{ '--wails-draggable': 'drag' } as CSSProperties}>{t('agent.title')}</h2>
              {canCreate ? (
                <Tooltip content={t('agent.createAgent')} placement="bottom">
                  <Button
                    variant="text"
                    theme="default"
                    size="small"
                    className="header-action-btn"
                    data-guide="agent-list-create"
                    style={{ '--wails-draggable': 'no-drag' } as CSSProperties}
                    onClick={props.onCreate}
                    icon={<span className="btn-icon-wrapper"><SparklesIcon size={19} /></span>}
                  />
                </Tooltip>
              ) : null}
            </div>
            <p className="header-subtitle" style={{ '--wails-draggable': 'drag' } as CSSProperties}>{t('agent.subtitle')}</p>
          </div>
        </div>
        <div className="agent-list-main">
          {showSkeleton ? <SkeletonCards /> : null}
          {!loading && !spaceLoading && hasCards ? (
            <div className="agent-card-wrap">
              {isSectioned
                ? sections.map((section) => (
                  <AgentSection key={section.key} section={section} t={t} viewer={viewer} collapsed={collapsedSections.has(section.key)} onToggle={props.onToggleSection} cardProps={cardProps} />
                ))
                : flatCards.map((agent) => <AgentCard key={agent.id} {...cardProps(agent)} />)}
            </div>
          ) : null}
          {spaceLoading ? (
            <div className="agent-list-main-loading" data-agent-space-loading="true">
              <Loading size="medium" text="" />
            </div>
          ) : null}
          {!loading && !spaceLoading && !hasCards ? <EmptyState t={t} space={space} canCreate={canCreate} onCreate={props.onCreate} /> : null}
        </div>
      </div>
      {drawer ? <AgentDetailDrawer kind={drawer.kind} agent={drawer.agent} t={t} onClose={props.onCloseDrawer} onUseInChat={props.onUseInChat} /> : null}
      {editor ? <AgentEditorModal open mode={editor.mode} agent={editor.agent} initialSection={editor.initialSection} initialHighlightField={editor.initialHighlight} readOnly={editor.readOnly} client={props.client} t={editorT} onClose={props.onCloseEditor} onSaved={props.onEditorSaved} /> : null}
      <AgentDeleteDialog agent={deleteTarget} t={t} busy={deleting} onConfirm={props.onDeleteConfirm} onCancel={props.onDeleteCancel} />
    </div>
  );
}

// --- 空间 URL 状态（Vue useListUrlState ?scope=） ------------------------------------

const RESERVED_SCOPES = new Set(['all', 'mine', 'favorites', 'recents']);

function readSpaceFromUrl(): string | null {
  const value = new URLSearchParams(window.location.search).get('scope');
  return value && value.trim() !== '' ? value : null;
}

function writeSpaceToUrl(space: string): void {
  const url = new URL(window.location.href);
  if (space === 'all' || !space) url.searchParams.delete('scope');
  else url.searchParams.set('scope', space);
  window.history.replaceState({}, '', url);
}

function isSectionedView(space: string): boolean {
  return space === 'all' || space === 'mine' || (space !== '' && space !== 'favorites' && space !== 'recents');
}

// The Vue rail labels live under listSpaceSidebar.*, which packages/i18n does
// not carry yet (reported gap). formatMessage echoes unknown keys, so try the
// real key first and fall back to the Vue literals until the keys land.
const RAIL_LABEL_FALLBACK: Record<'recents' | 'workspace', Partial<Record<Locale, string>>> = {
  recents: { 'zh-CN': '最近', 'en-US': 'Recent', 'ja-JP': '最近', 'ko-KR': '최근', 'ru-RU': 'Недавние' },
  workspace: { 'zh-CN': '本空间', 'en-US': 'Workspace', 'ja-JP': 'ワークスペース', 'ko-KR': '워크스페이스', 'ru-RU': 'Пространство' },
};

function railLabel(t: Translate, key: string, kind: 'recents' | 'workspace', locale: Locale): string {
  const resolved = t(key);
  return resolved === key ? RAIL_LABEL_FALLBACK[kind][locale] ?? resolved : resolved;
}

function membershipRoleOf(memberships: unknown, tenantId: string | null): string {
  if (!Array.isArray(memberships)) return 'viewer';
  for (const item of memberships) {
    if (!item || typeof item !== 'object') continue;
    const row = item as Record<string, unknown>;
    const id = row.tenant_id ?? row.tenantId;
    const role = row.role;
    if (tenantId !== null && String(id) === tenantId && typeof role === 'string') return role;
  }
  return 'viewer';
}

interface AgentsPageProps { client: WeKnoraClient; tenantId?: string | number | null }

// Vue's useTenantModelReadiness counts KnowledgeQA models as chat models.
export function hasAgentChatModel(models: ReadonlyArray<{ type?: unknown }>): boolean {
  return models.some((model) => model.type === 'KnowledgeQA');
}

export function AgentsPage({ client, tenantId }: AgentsPageProps) {
  const locale = usePreferredLocale();
  const t = useCallback<Translate>((key, values) => formatMessage(locale, key, values), [locale]);
  // agentEditor.* copy is not in packages/i18n yet; the editor falls back to
  // ported literals for those keys while agent.* resolves normally.
  const editorT = useMemo(() => makeEditorT(locale), [locale]);
  const [viewer, setViewer] = useState<AgentViewer>({ userId: '', isAdmin: false, isContributor: false });
  const [viewerReady, setViewerReady] = useState(false);
  // Vue useTenantModelReadiness (frontend/src/composables/useTenantModelReadiness.ts):
  // the agent list tours and the create gate read tenant model readiness —
  // a configured chat (KnowledgeQA / type llm) model.
  const [modelsReady, setModelsReady] = useState<boolean | null>(null);
  const [data, setData] = useState<AgentsPageData | null>(null);
  const [spaceItems, setSpaceItems] = useState<Array<Record<string, unknown>>>([]);
  const [spaceLoading, setSpaceLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [reloadToken, setReloadToken] = useState(0);
  const [space, setSpaceState] = useState<string>(() => (typeof window === 'undefined' ? 'all' : readSpaceFromUrl() ?? ''));
  const [favorites, setFavorites] = useState<Set<string>>(() => new Set());
  const [recents, setRecents] = useState<PinEntry[]>([]);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const [collapsedSections, setCollapsedSections] = useState<ReadonlySet<string>>(new Set());
  const [drawer, setDrawer] = useState<AgentsPageViewProps['drawer']>(null);
  const [editor, setEditor] = useState<AgentsPageViewProps['editor']>(null);
  const [deleteTarget, setDeleteTarget] = useState<AgentCardModel | null>(null);
  const [deleting, setDeleting] = useState(false);

  const tenantKey = tenantId === undefined || tenantId === null ? null : String(tenantId);

  // Viewer + per-(user, tenant) pins hydrate (App.tsx membershipRole pattern).
  useEffect(() => {
    let active = true;
    setViewerReady(false);
    void client.auth.me().then((me) => {
      if (!active) return;
      const role = membershipRoleOf(me.memberships, tenantKey);
      const userId = typeof me.user?.id === 'string' ? me.user.id : '';
      setViewer({
        userId,
        isAdmin: role === 'owner' || role === 'admin' || me.user?.is_system_admin === true,
        isContributor: role === 'owner' || role === 'admin' || role === 'contributor' || me.user?.is_system_admin === true,
      });
      setViewerReady(true);
      setFavorites(new Set(readFavoriteIds(window.localStorage, userId, tenantKey)));
      setRecents(readAgentRecents(window.localStorage, userId, tenantKey));
    }).catch(() => {
      // Deep-link consumption waits for an authoritative permission result;
      // a failed identity lookup must not accidentally grant an edit surface.
      setViewerReady(true);
    });
    return () => { active = false; };
  }, [client, tenantKey]);

  useEffect(() => {
    let active = true;
    setLoadError(null);
    void loadAgentsPageData(client).then((next) => {
      if (active) setData(next);
    }).catch((loadFailure) => {
      if (active) setLoadError(loadFailure instanceof Error ? loadFailure.message : t('common.error'));
    });
    return () => { active = false; };
  }, [client, reloadToken, t]);

  useEffect(() => {
    let active = true;
    void client.configuration.models.list().then((models) => {
      if (active) setModelsReady(hasAgentChatModel(models));
    }).catch(() => { if (active) setModelsReady(false); });
    return () => { active = false; };
  }, [client]);

  // Default scope follows the Vue role rule: contributor → mine, else all
  // (AgentList.vue defaultScope). Resolved once here so rows, sections, and
  // the rail all agree even before the user picks a scope.
  const effectiveSpace = space || (viewer.isContributor ? 'mine' : 'all');

  // Space view fetch (GET /organizations/:id/shared-agents — routes_agent.go:171).
  // While in flight the space tab shows the Vue spaceAgentsLoading spinner
  // instead of the "no shared agents" empty state (AgentList.vue:498-500,710).
  useEffect(() => {
    if (!effectiveSpace || RESERVED_SCOPES.has(effectiveSpace)) { setSpaceItems([]); setSpaceLoading(false); return; }
    let active = true;
    setSpaceLoading(true);
    void client.identity.organizations.agentShares.listInOrganization(effectiveSpace).then((rows) => {
      if (!active) return;
      setSpaceItems(rows);
      setSpaceLoading(false);
    }).catch(() => { if (active) { setSpaceItems([]); setSpaceLoading(false); } });
    return () => { active = false; };
  }, [client, effectiveSpace]);

  const setSpace = useCallback((next: string) => {
    setSpaceState(next);
    setOpenMenuId(null);
    writeSpaceToUrl(next);
  }, []);

  const reload = useCallback(() => setReloadToken((value) => value + 1), []);

  const toggleFavorite = useCallback((id: string) => {
    setFavorites((current) => {
      const next = new Set(toggleFavoriteId([...current], id));
      writeFavoriteIds(window.localStorage, viewer.userId, tenantKey, [...next]);
      return next;
    });
  }, [tenantKey, viewer.userId]);

  const openCard = useCallback((agent: AgentCardModel) => {
    setOpenMenuId(null);
    setRecents((current) => {
      const next = touchAgentRecent(current, agent.id, Date.now());
      writeAgentRecents(window.localStorage, viewer.userId, tenantKey, next);
      return next;
    });
    // AgentList.vue handleCardClick / handleSpaceAgentCardClick: shared →
    // detail drawer, own (and space-view "shared by me") → editor.
    if (opensEditorOnCardClick(agent)) { setEditor({ mode: 'edit', agent }); return; }
    setDrawer({ kind: 'shared', agent });
  }, [tenantKey, viewer.userId]);

  const onMenuAction = useCallback((action: AgentCardAction, agent: AgentCardModel) => {
    setOpenMenuId(null);
    if (action === 'edit') { setEditor({ mode: 'edit', agent }); return; }
    if (action === 'delete') { setDeleteTarget(agent); return; }
    if (action === 'copy') {
      void client.configuration.agents.copy(agent.id).then(() => { void MessagePlugin.success(t('agent.messages.copied')); reload(); }).catch(() => { void MessagePlugin.error(t('agent.messages.copyFailed')); });
      return;
    }
    if (action === 'toggle') {
      void client.identity.organizations.agentShares.setDisabledByMe(agent.id, !agent.disabledByMe)
        .then(() => { void MessagePlugin.success(t(agent.disabledByMe ? 'agent.messages.enabled' : 'agent.messages.disabled')); reload(); })
        .catch(() => { void MessagePlugin.error(t('agent.messages.saveFailed')); });
    }
  }, [client, reload, t]);

  const onCreate = useCallback(() => {
    // Vue AgentList.vue:1597-1602 — without a chat model the create click
    // warns and opens the models settings section instead; the tenantModels
    // (agent) tour re-arms there via the queued intent.
    if (modelsReady === false) {
      void MessagePlugin.warning(contextualGuideMessage(locale, 'contextualGuide.tenantModels.needChatModelFirst'));
      openContextualGuide('tenantModels', { variant: 'agent' });
      navigate('/platform/settings?section=models');
      return;
    }
    // Vue AgentList.vue:1603 + AgentCreateContextualGuide :when="create" —
    // opening the editor retires the agentList tour and arms agentCreate.
    markContextualGuideDone(window.localStorage, 'agentList');
    openContextualGuide('agentCreate', { isAgentMode: false });
    setEditor({ mode: 'create', agent: null });
  }, [locale, modelsReady]);

  const onEditorSaved = useCallback((_agent: Record<string, unknown>, mode: 'create' | 'edit') => {
    // Vue AgentEditorModal.vue:4825 — a successful create retires agentCreate.
    if (mode === 'create') markContextualGuideDone(window.localStorage, 'agentCreate');
    // create stays open inside the modal (post-create session); refresh the list either way
    void MessagePlugin.success(t(mode === 'create' ? 'agent.messages.created' : 'agent.messages.updated'));
    reload();
  }, [reload, t]);

  const onDeleteConfirm = useCallback(() => {
    if (!deleteTarget || deleting) return;
    setDeleting(true);
    void client.configuration.agents.remove(deleteTarget.id).then(() => {
      setDeleting(false); setDeleteTarget(null);
      void MessagePlugin.success(t('agent.messages.deleted'));
      reload();
    })
      .catch(() => { setDeleting(false); void MessagePlugin.error(t('agent.messages.deleteFailed')); });
  }, [client, deleteTarget, deleting, reload, t]);

  const onUseInChat = useCallback((agent: AgentCardModel) => {
    setDrawer(null);
    navigate(chatNavigationPath(agent));
  }, []);

  const rows = useMemo<AgentCardModel[]>(() => {
    if (!data) return [];
    const options = { userId: viewer.userId, disabledOwnIds: data.disabledOwnIds };
    if (effectiveSpace === 'favorites' || effectiveSpace === 'recents') {
      const index = buildPinIndex(data.ownAgents, data.sharedAgents, options);
      const ids = effectiveSpace === 'favorites' ? [...favorites] : recents.map((entry) => entry.id);
      return hydratePinnedCards(index, ids);
    }
    if (effectiveSpace === 'mine') return buildMineViewRows(data.ownAgents, options);
    if (!RESERVED_SCOPES.has(effectiveSpace)) return buildSpaceViewRows(spaceItems);
    return buildAllViewRows(data.ownAgents, data.sharedAgents, options);
  }, [data, effectiveSpace, favorites, recents, spaceItems, viewer.userId]);

  useEffect(() => {
    if (!data || !viewerReady) return;
    const deepLink = parseAgentEditDeepLink(window.location.search);
    if (!deepLink) return;
    const allRows = buildAllViewRows(data.ownAgents, data.sharedAgents, { userId: viewer.userId, disabledOwnIds: data.disabledOwnIds });
    const target = resolveAgentEditTarget(
      allRows.filter((agent) => agent.isMine),
      allRows.filter((agent) => !agent.isMine),
      deepLink.editId,
      deepLink.sourceTenantId,
    );
    if (!target) return;
    setDrawer(null);
    setEditor({
      mode: 'edit',
      agent: target,
      initialSection: deepLink.section,
      initialHighlight: deepLink.highlight,
      readOnly: !canManageAgent(target, viewer),
    });
    const url = new URL(window.location.href);
    url.searchParams.delete('edit');
    url.searchParams.delete('section');
    url.searchParams.delete('highlight');
    url.searchParams.delete('sourceTenantId');
    window.history.replaceState({}, '', url);
  }, [data, viewer, viewerReady]);

  const sectioned = isSectionedView(effectiveSpace);
  const sections = useMemo<AgentSectionView[]>(() => (sectioned ? sectionize(rows, viewer.userId) : []), [rows, sectioned, viewer.userId]);
  const flatCards = useMemo(() => (sectioned ? [] : rows), [rows, sectioned]);

  const rail = useMemo<AgentRailItem[]>(() => {
    const ownCount = data?.ownAgents.length ?? 0;
    const sharedCount = data?.sharedAgents.length ?? 0;
    const counts = data ? sharedCountByOrg(data.sharedAgents) : {};
    return [
      { key: 'all', label: t('common.all'), icon: 'layers' as const, count: ownCount + sharedCount, active: effectiveSpace === 'all' },
      { key: 'favorites', label: t('common.favorite'), icon: 'star' as const, count: favorites.size, active: effectiveSpace === 'favorites' },
      { key: 'recents', label: railLabel(t, 'listSpaceSidebar.recents', 'recents', locale), icon: 'history' as const, count: recents.length, active: effectiveSpace === 'recents' },
      { key: 'mine', label: railLabel(t, 'listSpaceSidebar.workspace', 'workspace', locale), icon: 'workspace' as const, count: ownCount, active: effectiveSpace === 'mine' },
      ...(data?.organizations ?? []).filter((org) => (counts[org.id] ?? 0) > 0).map((org) => ({
        key: org.id, label: org.name, icon: 'space' as const, avatarName: org.name, count: counts[org.id], active: effectiveSpace === org.id,
      })),
    ];
  }, [data, effectiveSpace, favorites.size, locale, recents.length, t]);

  // Vue AgentList.vue:1088-1102 — the empty contributor list (all/mine scope,
  // editor closed) selects between the tenantModels tour (agent variant, when
  // no chat model is configured) and the agentList tour (when it is). The
  // shell-level guide host applies the dismissal + welcome-tour gates.
  const agentListGuideBase = data !== null
    && viewer.isContributor
    && editor === null
    && (effectiveSpace === 'all' || effectiveSpace === 'mine')
    && rows.length === 0;
  const tenantModelsGuideWhen = agentListGuideBase && modelsReady === false;
  const agentListGuideWhen = agentListGuideBase && modelsReady === true;
  useEffect(() => {
    if (!tenantModelsGuideWhen) return;
    openContextualGuide('tenantModels', { variant: 'agent' });
  }, [tenantModelsGuideWhen]);
  useEffect(() => {
    if (!agentListGuideWhen) return;
    openContextualGuide('agentList');
  }, [agentListGuideWhen]);

  return (
    <AgentsPageView
      t={t}
      editorT={editorT}
      client={client}
      viewer={viewer}
      loading={!data && !loadError}
      space={effectiveSpace}
      spaceLoading={spaceLoading}
      rail={rail}
      sections={sections}
      flatCards={flatCards}
      isSectioned={isSectionedView(effectiveSpace)}
      favorites={favorites}
      openMenuId={openMenuId}
      error={loadError}
      notice={null}
      drawer={drawer}
      editor={editor}
      deleteTarget={deleteTarget}
      deleting={deleting}
      collapsedSections={collapsedSections}
      canCreate={viewer.isContributor}
      onSpaceChange={setSpace}
      onOpenCard={openCard}
      onToggleFavorite={toggleFavorite}
      onToggleMenu={setOpenMenuId}
      onMenuAction={onMenuAction}
      onCreate={onCreate}
      onCloseDrawer={() => setDrawer(null)}
      onUseInChat={onUseInChat}
      onCloseEditor={() => setEditor(null)}
      onEditorSaved={onEditorSaved}
      onDeleteConfirm={onDeleteConfirm}
      onDeleteCancel={() => setDeleteTarget(null)}
      onToggleSection={(key) => setCollapsedSections((current) => {
        const next = new Set(current);
        if (next.has(key)) next.delete(key); else next.add(key);
        return next;
      })}
    />
  );
}
