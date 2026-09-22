/**
 * kb-list 列表页 —— TDesign 同构迁移（Task 11a）。
 *
 * 事实源：frontend/src/views/knowledge/KnowledgeBaseList.vue（template/DOM/类名
 * 1:1 复刻）+ frontend/src/components/{ListSpaceSidebar,SpaceAvatar,
 * ResourceOriginBadge}.vue + knowledge/components/KbWikiBadge.vue +
 * KnowledgeBaseEditorModal.vue（settings-overlay 壳 / basic / faq / footer）。
 * 组件从 tdesign-react 具名导入（playbook §1），图标用 tdesign-icons-react 的
 * Icon（= Vue `Icon as TIcon`，本地 sprite `<use>` 渲染，台账 #10）；样式平移在
 * kb-list.td.css。纯逻辑仍在 list.ts / editor-config.ts / editor-sections.ts /
 * upload-progress.ts / @weknora/domain（本文件不重复实现）。
 * wk-* / data-* 测试 hook 按迁移前锚点保留（playbook §2.4）。
 *
 * 边界（任务报告详述）：编辑器 models / vectorStore / parser / storage /
 * chunking / multimodal / asr / graph / advanced / datasource / share / activity
 * 段仍为 R490 的 React 端口（与 knowledge-settings 域共享），仅其承载壳
 * （settings-overlay / sidebar / section / footer）随本任务迁 Vue DOM；这些
 * 段内部留待 knowledge-settings 批次（R8 批次 3）同构。
 */
import { Fragment, useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type MouseEvent as ReactMouseEvent, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import type { ModelConfiguration, WeKnoraClient } from '@weknora/api-client';
import { createScopeController, scopedKey, filterKnowledgeBases } from '@weknora/domain';
import {
  canDuplicateKBCard,
  canManageKBCard,
  isKnowledgeBaseInitialized,
  isSharedKbEditable,
  groupKnowledgeBaseSections,
  mergeAllScopeKnowledgeBases,
  type MergedKnowledgeBase,
} from '@weknora/domain';
import { formatMessage, type Locale, type MessageValues } from '@weknora/i18n';
import { usePreferredLocale } from '../locale.ts';
import { navigate } from '../platform/navigation.ts';
import {
  Button,
  Checkbox,
  Dialog,
  Input,
  Loading,
  Popup,
  Radio,
  RadioGroup,
  Skeleton,
  Textarea,
  Tooltip,
  MessagePlugin,
} from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
/* 旧栈组件仅供编辑器留守段（models / vectorStore / parser / storage / multimodal /
 * asr / advanced 段内部）继续使用——这些段待 knowledge-settings 批次迁移，本页
 * 未新增任何旧栈用法（playbook §4.2 留守例外，见文件头边界说明）。 */
// S6 抽屉收编：kb 编辑器深设置留守段离开 @weknora/ui（T15 硬前置），换 tdesign。
import { Checkbox as TCheckbox, Input as TInput, Select as TSelect, Textarea as TTextarea } from 'tdesign-react';
import {
  isContextualGuideDone,
  markContextualGuideDone,
  openContextualGuide,
} from '@weknora/views/guides/contextual-guides';
import {
  createDeleteGuard,
  knowledgeBaseDetailPath,
  loadKnowledgeBaseListPage,
  saveKnowledgeBase,
  type KnowledgeBaseListPageState,
  type KnowledgeBaseSaveInput,
} from './list.ts';
import { KnowledgeBaseShareDialog } from './KnowledgeBaseShareDialog.tsx';
import { KnowledgeBaseActivityPanel } from './KnowledgeBaseActivityPanel.tsx';
import { SharedKnowledgeBaseDrawer } from './SharedKnowledgeBaseDrawer.tsx';
import { defaultKnowledgeEditorConfig, hydrateKnowledgeEditorConfig, knowledgeEditorConfigPayload, type KnowledgeEditorConfig } from './editor-config.ts';
import { visibleKnowledgeEditorSections, type KnowledgeEditorSection } from './editor-sections.ts';
import { ChunkingSettingsFields } from '../knowledge-settings/chunkingSection.tsx';
import { GraphSettings, type GraphExtractConfig } from '../knowledge-settings/GraphSettings.tsx';
import { patchUploadTask, summarizeUploadTasks, upsertUploadTask, type UploadTaskState } from './upload-progress.ts';
import { DataSourcesPage } from '../data-sources/DataSourcesPage.tsx';
import { ModelOptionSelect, type ModelOption } from '../settings/ModelOptionSelect.tsx';
import { selectInitialModelId } from '../agents/agent-editor.ts';
import { resolveChatModelOptions, listChatModels } from '../chat/model-chip.ts';
import './kb-list.td.css';
/* kb-editor-parity.css：只剩编辑器留守段（R490 端口 internals）的锚点规则，
 * 壳/导航/footer/基础段规则已随 Vue 平移废弃并在该文件内清理。 */
import './kb-editor-parity.css';

export interface KnowledgeBasesPageProps {
  client: WeKnoraClient;
  scopeController: ReturnType<typeof createScopeController>;
}

interface Viewer {
  userId: string;
  isAdmin: boolean;
  isContributor: boolean;
}

type Translate = (key: string, values?: MessageValues) => string;

// Vue ?scope= semantics (KnowledgeBaseList.vue:821-830, 897-906): the state
// lives in the URL so links are shareable. The four reserved pseudo-scopes
// keep their Vue names; any other value is a per-org space id (?scope=<orgId>).
type KbListSpace = 'all' | 'mine' | 'favorites' | 'recents' | (string & {});

const KB_RESERVED_SCOPES: ReadonlySet<string> = new Set(['all', 'mine', 'favorites', 'recents']);

function isOrgScope(space: KbListSpace): boolean {
  return !KB_RESERVED_SCOPES.has(space);
}

/* ListSpaceSidebar truncateLabel (ListSpaceSidebar.vue:244-247)：折叠条标签
   ~44px（4 个 CJK 字符）；完整名称经 t-tooltip content 呈现。 */
function truncateLabel(text: string, max = 4): string {
  return text.length > max ? text.slice(0, max) + '…' : text;
}

function membershipRole(memberships: unknown, tenantId: string | null): string {
  if (!Array.isArray(memberships)) return 'viewer';
  for (const item of memberships) {
    if (!item || typeof item !== 'object' || Array.isArray(item)) continue;
    const row = item as Record<string, unknown>;
    const id = row.tenant_id ?? row.tenantId;
    const role = row.role;
    if (tenantId !== null && String(id) === tenantId && typeof role === 'string') return role;
  }
  return 'viewer';
}

function readScopeFromUrl(): KbListSpace {
  const value = new URLSearchParams(window.location.search).get('scope');
  // Stale URL guard (KnowledgeBaseList.vue:1246-1251): an older "协作"
  // view used scope=shared; that view is gone — land on all.
  if (value === null || value === '' || value === 'all' || value === 'shared') return 'all';
  if (value === 'mine' || value === 'favorites' || value === 'recents') return value;
  return value; // per-org space id (?scope=<orgId>)
}

function writeScopeToUrl(space: KbListSpace): void {
  const url = new URL(window.location.href);
  if (space === 'all') url.searchParams.delete('scope');
  else url.searchParams.set('scope', space);
  window.history.replaceState({}, '', url);
}

function readFavorites(): Set<string> {
  try {
    const raw = window.localStorage.getItem('wk-kb-favorites');
    return new Set(raw ? (JSON.parse(raw) as string[]) : []);
  } catch {
    return new Set();
  }
}

/* Vue kbSectionOf / kbOriginVariant (KnowledgeBaseList.vue:1118-1123, 1380-1391)。 */
function isMyKb(kb: { creator_id?: unknown }, userId: string): boolean {
  const creator = typeof kb.creator_id === 'string' ? kb.creator_id : '';
  return !!(creator && userId && creator === userId);
}

/** Vue shouldShowResourceOriginBadge (frontend/src/utils/card-list-badge.ts)。 */
function shouldShowResourceOriginBadge(opts: {
  section: string;
  variant: 'mine' | 'creator';
  creatorName?: string;
}): boolean {
  const hasCreator = Boolean((opts.creatorName ?? '').trim());
  if (opts.section === 'mine' && opts.variant === 'mine') return false;
  if (opts.section === 'tenantOthers' && opts.variant === 'creator' && !hasCreator) return false;
  return true;
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
/* frontend/src/assets/img/upload.svg —— 空状态插画（162×162，10KB；内联副本，
   与 agents.td.css 同源文件，按页各持一份，Phase 4 归并）。 */
const UPLOAD_SVG = 'data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMTYyIiBoZWlnaHQ9IjE2MiIgdmlld0JveD0iMCAwIDE2MiAxNjIiIGZpbGw9Im5vbmUiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyI+CjxnIGZpbHRlcj0idXJsKCNmaWx0ZXIwX2RfNjAyMl81MTczMSkiPgo8cGF0aCBkPSJNMzYuODc1IDc4TDIwIDExMS43NVYxMzMuMDQ3QzIwIDE0MC43NiAyNi4yNTI2IDE0Ny4wMTMgMzMuOTY1NSAxNDcuMDEzSDgwLjc1SDEyNy41MzRDMTM1LjI0NyAxNDcuMDEzIDE0MS41IDE0MC43NiAxNDEuNSAxMzMuMDQ3VjExMS43NUwxMjQuNjI1IDc4SDgwLjc1SDM2Ljg3NVoiIGZpbGw9InVybCgjcGFpbnQwX2xpbmVhcl82MDIyXzUxNzMxKSIvPgo8L2c+CjxwYXRoIGQ9Ik0zNy4xMjUgMTExLjM3NVY3Ny42MjVMMjAuMjUgMTExLjM3NUgzNy4xMjVaIiBmaWxsPSJ1cmwoI3BhaW50MV9saW5lYXJfNjAyMl81MTczMSkiLz4KPHBhdGggZD0iTTEyNSAxMTEuNzVWNzhMMTQxLjg3NSAxMTEuNzVIMTI1WiIgZmlsbD0idXJsKCNwYWludDJfbGluZWFyXzYwMjJfNTE3MzEpIi8+CjxwYXRoIGQ9Ik03Ny45ODY0IDEwOC42MjdMNjYuMjc0IDkzLjc0MzZDNjUuNDAyOSA5Mi42MzY1IDY2LjE5MTUgOTEuMDEyNSA2Ny42MDAyIDkxLjAxMjVINzIuNjM1QzczLjU2NyA5MS4wMTI1IDc0LjMyOTIgOTAuMjYyNSA3NC4yMDExIDg5LjMzOTRDNzIuNjc1NCA3OC4zNTAzIDU2Ljg4MDUgNTkuNDM1NSAzMy4xMDA3IDUwLjg1NDlDMzIuMTcyOSA1MC41MjAxIDMyLjQwNjcgNDguOTM3NSAzMy4zOTMgNDguOTM3NUgxMjUuMjMyQzEyNi4yMTggNDguOTM3NSAxMjYuNDUyIDUwLjUyMDEgMTI1LjUyNCA1MC44NTQ5QzEwMS43NDQgNTkuNDM1NSA4NS45NDk2IDc4LjM1MDMgODQuNDIzOSA4OS4zMzk0Qzg0LjI5NTcgOTAuMjYyNSA4NS4wNTggOTEuMDEyNSA4NS45OSA5MS4wMTI1SDkxLjAyNDhDOTIuNDMzNSA5MS4wMTI1IDkzLjIyMjEgOTIuNjM2NSA5Mi4zNTEgOTMuNzQzNkw4MC42Mzg2IDEwOC42MjdDNzkuOTYzIDEwOS40ODYgNzguNjYyIDEwOS40ODYgNzcuOTg2NCAxMDguNjI3WiIgZmlsbD0idXJsKCNwYWludDNfbGluZWFyXzYwMjJfNTE3MzEpIi8+CjxwYXRoIGZpbGwtcnVsZT0iZXZlbm9kZCIgY2xpcC1ydWxlPSJldmVub2RkIiBkPSJNNjcuNjA0IDExMS4zNzVIMjAuMjVWMTMzLjEzOEMyMC4yNSAxNDAuNTk0IDI2LjI5NDIgMTQ2LjYzOCAzMy43NSAxNDYuNjM4SDEyOC4yNUMxMzUuNzA2IDE0Ni42MzggMTQxLjc1IDE0MC41OTQgMTQxLjc1IDEzMy4xMzhWMTExLjM3NUg5NC4zOTUxQzkzLjU2NDcgMTE4LjAzNCA4Ny44ODM5IDEyMy4xODggODAuOTk5NSAxMjMuMTg4Qzc0LjExNTIgMTIzLjE4OCA2OC40MzQ0IDExOC4wMzQgNjcuNjA0IDExMS4zNzVaIiBmaWxsPSJ1cmwoI3BhaW50NF9saW5lYXJfNjAyMl81MTczMSkiLz4KPHBhdGggZD0iTTQ2LjkzNjYgMTguNTQzNkM0Ni43NDA4IDE4LjE0MTEgNDYuNzEyOSAxNy42Nzc0IDQ2Ljg1OTEgMTcuMjU0NEw0OS4xMTAyIDEwLjczNzVDNDkuODcxIDguNTM1MiA1Mi4yNzI5IDcuMzY2NjEgNTQuNDc1MiA4LjEyNzM0TDY4LjgzMDQgMTMuMDg2MUM3MS4wMzI2IDEzLjg0NjggNzIuMjAxMiAxNi4yNDg4IDcxLjQ0MDUgMTguNDUxMUw2Ny41ODM3IDI5LjYxNjJDNjYuODIyOSAzMS44MTg1IDY0LjQyMSAzMi45ODcxIDYyLjIxODcgMzIuMjI2M0w1Mi41MTE3IDI4Ljg3MzJDNTIuMDg4NyAyOC43MjcxIDUxLjc0MTEgMjguNDE4OSA1MS41NDUzIDI4LjAxNjVMNDYuOTM2NiAxOC41NDM2WiIgZmlsbD0idXJsKCNwYWludDVfbGluZWFyXzYwMjJfNTE3MzEpIi8+CjxtYXNrIGlkPSJtYXNrMF82MDIyXzUxNzMxIiBzdHlsZT0ibWFzay10eXBlOmFscGhhIiBtYXNrVW5pdHM9InVzZXJTcGFjZU9uVXNlIiB4PSI0NiIgeT0iNyIgd2lkdGg9IjI2IiBoZWlnaHQ9IjI2Ij4KPHBhdGggZD0iTTQ2LjkzNjYgMTguNTQzNkM0Ni43NDA4IDE4LjE0MTEgNDYuNzEyOSAxNy42Nzc0IDQ2Ljg1OTEgMTcuMjU0NEw0OS4xMTAyIDEwLjczNzVDNDkuODcxIDguNTM1MiA1Mi4yNzI5IDcuMzY2NjEgNTQuNDc1MiA4LjEyNzM0TDY4LjgzMDQgMTMuMDg2MUM3MS4wMzI2IDEzLjg0NjggNzIuMjAxMiAxNi4yNDg4IDcxLjQ0MDUgMTguNDUxMUw2Ny41ODM3IDI5LjYxNjJDNjYuODIyOSAzMS44MTg1IDY0LjQyMSAzMi45ODcxIDYyLjIxODcgMzIuMjI2M0w1Mi41MTE3IDI4Ljg3MzJDNTIuMDg4NyAyOC43MjcxIDUxLjc0MTEgMjguNDE4OSA1MS41NDUzIDI4LjAxNjVMNDYuOTM2NiAxOC41NDM2WiIgZmlsbD0iI0Q5RDlEOSIvPgo8L21hc2s+CjxnIG1hc2s9InVybCgjbWFzazBfNjAyMl81MTczMSkiPgo8cGF0aCBkPSJNNDMuODc2IDI1Ljg5MDFMNDYuNjMwOCAxNy45MTVMNTEuNDE1OSAxOS41NjhDNTMuMTc3NyAyMC4xNzY1IDU0LjExMjYgMjIuMDk4MSA1My41MDQgMjMuODU5OUw1MS44NTExIDI4LjY0NUw0My44NzYgMjUuODkwMVoiIGZpbGw9IiNCNUVDQ0YiLz4KPC9nPgo8cGF0aCBkPSJNODkuNTU3MiAxNi40Mzg1Qzg5LjY2NjUgMTYuMTE4MSA4OS44OTg3IDE1Ljg1NDMgOTAuMjAyNSAxNS43MDUxTDk0Ljg4MzIgMTMuNDA2NkM5Ni40NjQ5IDEyLjYyOTkgOTguMzc2OCAxMy4yODI1IDk5LjE1MzUgMTQuODY0MkwxMDQuMjE3IDI1LjE3NDZDMTA0Ljk5MyAyNi43NTYzIDEwNC4zNDEgMjguNjY4MiAxMDIuNzU5IDI5LjQ0NUw5NC43Mzk4IDMzLjM4MjlDOTMuMTU4MSAzNC4xNTk2IDkxLjI0NjIgMzMuNTA3MSA5MC40Njk1IDMxLjkyNTNMODcuMDQ1OCAyNC45NTM1Qzg2Ljg5NjYgMjQuNjQ5NiA4Ni44NzQyIDI0LjI5OSA4Ni45ODM2IDIzLjk3ODZMODkuNTU3MiAxNi40Mzg1WiIgZmlsbD0idXJsKCNwYWludDZfbGluZWFyXzYwMjJfNTE3MzEpIi8+CjxtYXNrIGlkPSJtYXNrMV82MDIyXzUxNzMxIiBzdHlsZT0ibWFzay10eXBlOmFscGhhIiBtYXNrVW5pdHM9InVzZXJTcGFjZU9uVXNlIiB4PSI4NiIgeT0iMTMiIHdpZHRoPSIxOSIgaGVpZ2h0PSIyMSI+CjxwYXRoIGQ9Ik04OS41NTcyIDE2LjQzODVDODkuNjY2NSAxNi4xMTgxIDg5Ljg5ODcgMTUuODU0MyA5MC4yMDI1IDE1LjcwNTFMOTQuODgzMiAxMy40MDY2Qzk2LjQ2NDkgMTIuNjI5OSA5OC4zNzY4IDEzLjI4MjUgOTkuMTUzNSAxNC44NjQyTDEwNC4yMTcgMjUuMTc0NkMxMDQuOTkzIDI2Ljc1NjMgMTA0LjM0MSAyOC42NjgyIDEwMi43NTkgMjkuNDQ1TDk0LjczOTggMzMuMzgyOUM5My4xNTgxIDM0LjE1OTYgOTEuMjQ2MiAzMy41MDcxIDkwLjQ2OTUgMzEuOTI1M0w4Ny4wNDU4IDI0Ljk1MzVDODYuODk2NiAyNC42NDk2IDg2Ljg3NDIgMjQuMjk5IDg2Ljk4MzYgMjMuOTc4Nkw4OS41NTcyIDE2LjQzODVaIiBmaWxsPSIjRDlEOUQ5Ii8+CjwvbWFzaz4KPGcgbWFzaz0idXJsKCNtYXNrMV82MDIyXzUxNzMxKSI+CjxwYXRoIGQ9Ik04NCAxOC43NTFMODkuNzI4IDE1LjkzODJMOTEuNDE1NyAxOS4zNzVDOTIuMDM3MSAyMC42NDAzIDkxLjUxNSAyMi4xNjk5IDkwLjI0OTYgMjIuNzkxM0w4Ni44MTI4IDI0LjQ3OUw4NCAxOC43NTFaIiBmaWxsPSIjRTdFN0U3Ii8+CjwvZz4KPHBhdGggZD0iTTQ2LjM3MzQgNTcuMjI4OUM0Ni4yNTAyIDU3LjYxMjUgNDUuOTc5NiA1Ny45MzE1IDQ1LjYyMTMgNTguMTE1N0w0MC4xMDAxIDYwLjk1MzJDMzguMjM0NCA2MS45MTIxIDM1Ljk0NDUgNjEuMTc2OSAzNC45ODU3IDU5LjMxMTFMMjguNzM1NCA0Ny4xNDk0QzI3Ljc3NjYgNDUuMjgzNiAyOC41MTE4IDQyLjk5MzggMzAuMzc3NSA0Mi4wMzQ5TDM5LjgzNjYgMzcuMTczNkM0MS43MDI0IDM2LjIxNDggNDMuOTkyMiAzNi45NSA0NC45NTExIDM4LjgxNTdMNDkuMTc3NSA0Ny4wMzk1QzQ5LjM2MTcgNDcuMzk3OSA0OS4zOTU5IDQ3LjgxNDcgNDkuMjcyOCA0OC4xOTg0TDQ2LjM3MzQgNTcuMjI4OVoiIGZpbGw9InVybCgjcGFpbnQ3X2xpbmVhcl82MDIyXzUxNzMxKSIvPgo8bWFzayBpZD0ibWFzazJfNjAyMl81MTczMSIgc3R5bGU9Im1hc2stdHlwZTphbHBoYSIgbWFza1VuaXRzPSJ1c2VyU3BhY2VPblVzZSIgeD0iMjgiIHk9IjM2IiB3aWR0aD0iMjIiIGhlaWdodD0iMjYiPgo8cGF0aCBkPSJNNDYuMzczNCA1Ny4yMjg5QzQ2LjI1MDIgNTcuNjEyNSA0NS45Nzk2IDU3LjkzMTUgNDUuNjIxMyA1OC4xMTU3TDQwLjEwMDEgNjAuOTUzMkMzOC4yMzQ0IDYxLjkxMjEgMzUuOTQ0NSA2MS4xNzY5IDM0Ljk4NTcgNTkuMzExMUwyOC43MzU0IDQ3LjE0OTRDMjcuNzc2NiA0NS4yODM2IDI4LjUxMTggNDIuOTkzOCAzMC4zNzc1IDQyLjAzNDlMMzkuODM2NiAzNy4xNzM2QzQxLjcwMjQgMzYuMjE0OCA0My45OTIyIDM2Ljk1IDQ0Ljk1MTEgMzguODE1N0w0OS4xNzc1IDQ3LjAzOTVDNDkuMzYxNyA0Ny4zOTc5IDQ5LjM5NTkgNDcuODE0NyA0OS4yNzI4IDQ4LjE5ODRMNDYuMzczNCA1Ny4yMjg5WiIgZmlsbD0iI0Q5RDlEOSIvPgo8L21hc2s+CjxnIG1hc2s9InVybCgjbWFzazJfNjAyMl81MTczMSkiPgo8cGF0aCBkPSJNNTIuOTM3NSA1NC4zNTU3TDQ2LjE4MSA1Ny44MjgxTDQ0LjA5NzYgNTMuNzc0MkM0My4zMzA1IDUyLjI4MTYgNDMuOTE4NiA1MC40NDk3IDQ1LjQxMTIgNDkuNjgyNkw0OS40NjUxIDQ3LjU5OTJMNTIuOTM3NSA1NC4zNTU3WiIgZmlsbD0iI0U3RTdFNyIvPgo8L2c+CjxwYXRoIGQ9Ik0xMjAuODggMzcuMzg5MkMxMjEuMjAzIDM3LjQ3NTggMTIxLjQ3OSAzNy42ODcyIDEyMS42NDYgMzcuOTc2OUwxMjQuMjIzIDQyLjQzOTlDMTI1LjA5MyA0My45NDgxIDEyNC41NzcgNDUuODc2NiAxMjMuMDY5IDQ2Ljc0NzRMMTEzLjIzOCA1Mi40MjMzQzExMS43MjkgNTMuMjk0IDEwOS44MDEgNTIuNzc3MyAxMDguOTMgNTEuMjY5MUwxMDQuNTE1IDQzLjYyMjhDMTAzLjY0NSA0Mi4xMTQ2IDEwNC4xNjEgNDAuMTg2MSAxMDUuNjcgMzkuMzE1M0wxMTIuMzE3IDM1LjQ3NzNDMTEyLjYwNyAzNS4zMSAxMTIuOTUxIDM1LjI2NDcgMTEzLjI3NCAzNS4zNTEzTDEyMC44OCAzNy4zODkyWiIgZmlsbD0idXJsKCNwYWludDhfbGluZWFyXzYwMjJfNTE3MzEpIi8+CjxtYXNrIGlkPSJtYXNrM182MDIyXzUxNzMxIiBzdHlsZT0ibWFzay10eXBlOmFscGhhIiBtYXNrVW5pdHM9InVzZXJTcGFjZU9uVXNlIiB4PSIxMDQiIHk9IjM1IiB3aWR0aD0iMjEiIGhlaWdodD0iMTgiPgo8cGF0aCBkPSJNMTIwLjg4IDM3LjM4OTJDMTIxLjIwMyAzNy40NzU4IDEyMS40NzkgMzcuNjg3MiAxMjEuNjQ2IDM3Ljk3NjlMMTI0LjIyMyA0Mi40Mzk5QzEyNS4wOTMgNDMuOTQ4MSAxMjQuNTc3IDQ1Ljg3NjYgMTIzLjA2OSA0Ni43NDc0TDExMy4yMzggNTIuNDIzM0MxMTEuNzI5IDUzLjI5NCAxMDkuODAxIDUyLjc3NzMgMTA4LjkzIDUxLjI2OTFMMTA0LjUxNSA0My42MjI4QzEwMy42NDUgNDIuMTE0NiAxMDQuMTYxIDQwLjE4NjEgMTA1LjY3IDM5LjMxNTNMMTEyLjMxNyAzNS40NzczQzExMi42MDcgMzUuMzEgMTEyLjk1MSAzNS4yNjQ3IDExMy4yNzQgMzUuMzUxM0wxMjAuODggMzcuMzg5MloiIGZpbGw9IiNEOUQ5RDkiLz4KPC9tYXNrPgo8ZyBtYXNrPSJ1cmwoI21hc2szXzYwMjJfNTE3MzEpIj4KPHBhdGggZD0iTTExOC4yMzEgMzIuMDYyN0wxMjEuMzg1IDM3LjUyNDRMMTE4LjEwOCAzOS40MTY0QzExNi45MDEgNDAuMTEzIDExNS4zNTggMzkuNjk5NiAxMTQuNjYyIDM4LjQ5M0wxMTIuNzcgMzUuMjE2TDExOC4yMzEgMzIuMDYyN1oiIGZpbGw9IiNCNUVDQ0YiLz4KPC9nPgo8cGF0aCBkPSJNNzMuMzQ4MyA0NS4wOTg0QzczLjM0NzggNDQuODQ2OCA3My40NDczIDQ0LjYwNTMgNzMuNjI0OCA0NC40MjdMNzYuMzYwMyA0MS42ODA1Qzc3LjI4NDcgNDAuNzUyNCA3OC43ODY0IDQwLjc0OTQgNzkuNzE0NSA0MS42NzM4TDg1Ljc2NDMgNDcuNjk5M0M4Ni42OTI0IDQ4LjYyMzcgODYuNjk1NSA1MC4xMjU1IDg1Ljc3MTEgNTEuMDUzNkw4MS4wODQ1IDU1Ljc1OUM4MC4xNjAxIDU2LjY4NzEgNzguNjU4NCA1Ni42OTAxIDc3LjczMDMgNTUuNzY1N0w3My42Mzk0IDUxLjY5MTJDNzMuNDYxMSA1MS41MTM3IDczLjM2MDcgNTEuMjcyNiA3My4zNjAyIDUxLjAyMUw3My4zNDgzIDQ1LjA5ODRaIiBmaWxsPSJ1cmwoI3BhaW50OV9saW5lYXJfNjAyMl81MTczMSkiLz4KPG1hc2sgaWQ9Im1hc2s0XzYwMjJfNTE3MzEiIHN0eWxlPSJtYXNrLXR5cGU6YWxwaGEiIG1hc2tVbml0cz0idXNlclNwYWNlT25Vc2UiIHg9IjczIiB5PSI0MCIgd2lkdGg9IjE0IiBoZWlnaHQ9IjE3Ij4KPHBhdGggZD0iTTczLjM0ODMgNDUuMDk4NEM3My4zNDc4IDQ0Ljg0NjggNzMuNDQ3MyA0NC42MDUzIDczLjYyNDggNDQuNDI3TDc2LjM2MDMgNDEuNjgwNUM3Ny4yODQ3IDQwLjc1MjQgNzguNzg2NCA0MC43NDk0IDc5LjcxNDUgNDEuNjczOEw4NS43NjQzIDQ3LjY5OTNDODYuNjkyNCA0OC42MjM3IDg2LjY5NTUgNTAuMTI1NSA4NS43NzExIDUxLjA1MzZMODEuMDg0NSA1NS43NTlDODAuMTYwMSA1Ni42ODcxIDc4LjY1ODQgNTYuNjkwMSA3Ny43MzAzIDU1Ljc2NTdMNzMuNjM5NCA1MS42OTEyQzczLjQ2MTEgNTEuNTEzNyA3My4zNjA3IDUxLjI3MjYgNzMuMzYwMiA1MS4wMjFMNzMuMzQ4MyA0NS4wOTg0WiIgZmlsbD0iI0Q5RDlEOSIvPgo8L21hc2s+CjxnIG1hc2s9InVybCgjbWFzazRfNjAyMl81MTczMSkiPgo8cGF0aCBkPSJNNzAgNDguMDY2NEw3My4zNDc1IDQ0LjcwNTRMNzUuMzY0MSA0Ni43MTM5Qzc2LjEwNjYgNDcuNDUzNCA3Ni4xMDkgNDguNjU0OCA3NS4zNjk1IDQ5LjM5NzNMNzMuMzYxIDUxLjQxMzlMNzAgNDguMDY2NFoiIGZpbGw9IiMwN0MwNUYiLz4KPC9nPgo8cGF0aCBkPSJNMTA2LjEzOCAxMjAuMTAzQzEwNi4xMzggMTE4Ljk0NiAxMDcuMDc2IDExOC4wMDkgMTA4LjIzMyAxMTguMDA5SDExMy44MTlDMTE0Ljk3NiAxMTguMDA5IDExNS45MTQgMTE4Ljk0NiAxMTUuOTE0IDEyMC4xMDNWMTIwLjEwM0MxMTUuOTE0IDEyMS4yNiAxMTQuOTc2IDEyMi4xOTggMTEzLjgxOSAxMjIuMTk4SDEwOC4yMzNDMTA3LjA3NiAxMjIuMTk4IDEwNi4xMzggMTIxLjI2IDEwNi4xMzggMTIwLjEwM1YxMjAuMTAzWiIgZmlsbD0iIzE0ODVFRSIvPgo8cGF0aCBkPSJNMTIyLjg5NiAxMjAuMTAzQzEyMi44OTYgMTE4Ljk0NiAxMjMuODM0IDExOC4wMDkgMTI0Ljk5MSAxMTguMDA5SDEzMC41NzhDMTMxLjczNCAxMTguMDA5IDEzMi42NzIgMTE4Ljk0NiAxMzIuNjcyIDEyMC4xMDNWMTIwLjEwM0MxMzIuNjcyIDEyMS4yNiAxMzEuNzM0IDEyMi4xOTggMTMwLjU3OCAxMjIuMTk4SDEyNC45OTFDMTIzLjgzNCAxMjIuMTk4IDEyMi44OTYgMTIxLjI2IDEyMi44OTYgMTIwLjEwM1YxMjAuMTAzWiIgZmlsbD0iIzA3QzA1RiIvPgo8cmVjdCB4PSIxMDYuMTM4IiB5PSIxMTcuMzEiIHdpZHRoPSI5Ljc3NTg2IiBoZWlnaHQ9IjQuMTg5NjYiIHJ4PSIyLjA5NDgzIiBmaWxsPSIjNDM5REYxIi8+CjxyZWN0IHg9IjEyMi44OTYiIHk9IjExNy4zMSIgd2lkdGg9IjkuNzc1ODYiIGhlaWdodD0iNC4xODk2NiIgcng9IjIuMDk0ODMiIGZpbGw9IiMzOUNEODAiLz4KPGRlZnM+CjxmaWx0ZXIgaWQ9ImZpbHRlcjBfZF82MDIyXzUxNzMxIiB4PSIxNC40MTM4IiB5PSI3NS4yMDY5IiB3aWR0aD0iMTMyLjY3MiIgaGVpZ2h0PSI4MC4xODU0IiBmaWx0ZXJVbml0cz0idXNlclNwYWNlT25Vc2UiIGNvbG9yLWludGVycG9sYXRpb24tZmlsdGVycz0ic1JHQiI+CjxmZUZsb29kIGZsb29kLW9wYWNpdHk9IjAiIHJlc3VsdD0iQmFja2dyb3VuZEltYWdlRml4Ii8+CjxmZUNvbG9yTWF0cml4IGluPSJTb3VyY2VBbHBoYSIgdHlwZT0ibWF0cml4IiB2YWx1ZXM9IjAgMCAwIDAgMCAwIDAgMCAwIDAgMCAwIDAgMCAwIDAgMCAwIDEyNyAwIiByZXN1bHQ9ImhhcmRBbHBoYSIvPgo8ZmVPZmZzZXQgZHk9IjIuNzkzMSIvPgo8ZmVHYXVzc2lhbkJsdXIgc3RkRGV2aWF0aW9uPSIyLjc5MzEiLz4KPGZlQ29tcG9zaXRlIGluMj0iaGFyZEFscGhhIiBvcGVyYXRvcj0ib3V0Ii8+CjxmZUNvbG9yTWF0cml4IHR5cGU9Im1hdHJpeCIgdmFsdWVzPSIwIDAgMCAwIDAuMTkyNjkxIDAgMCAwIDAgMC4xOTI2OTEgMCAwIDAgMCAwLjE5MjY5MSAwIDAgMCAwLjEgMCIvPgo8ZmVCbGVuZCBtb2RlPSJub3JtYWwiIGluMj0iQmFja2dyb3VuZEltYWdlRml4IiByZXN1bHQ9ImVmZmVjdDFfZHJvcFNoYWRvd182MDIyXzUxNzMxIi8+CjxmZUJsZW5kIG1vZGU9Im5vcm1hbCIgaW49IlNvdXJjZUdyYXBoaWMiIGluMj0iZWZmZWN0MV9kcm9wU2hhZG93XzYwMjJfNTE3MzEiIHJlc3VsdD0ic2hhcGUiLz4KPC9maWx0ZXI+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQwX2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iODAuNzUiIHkxPSI3OCIgeDI9IjgwLjc1IiB5Mj0iMTMyIiBncmFkaWVudFVuaXRzPSJ1c2VyU3BhY2VPblVzZSI+CjxzdG9wIHN0b3AtY29sb3I9IiNFNEY5RUUiLz4KPHN0b3Agb2Zmc2V0PSIxIiBzdG9wLWNvbG9yPSIjOUVERUJEIi8+CjwvbGluZWFyR3JhZGllbnQ+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQxX2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iMjguNjg3NSIgeTE9Ijc3LjYyNSIgeDI9IjI4LjY4NzUiIHkyPSIxMTEuMzc1IiBncmFkaWVudFVuaXRzPSJ1c2VyU3BhY2VPblVzZSI+CjxzdG9wIHN0b3AtY29sb3I9IiNEQkZBRTkiLz4KPHN0b3Agb2Zmc2V0PSIxIiBzdG9wLWNvbG9yPSIjMkNEODdFIi8+CjwvbGluZWFyR3JhZGllbnQ+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQyX2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iMTMzLjQzOCIgeTE9Ijc4IiB4Mj0iMTMzLjQzOCIgeTI9IjExMS43NSIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjREJGQUU5Ii8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0iIzJDRDg3RSIvPgo8L2xpbmVhckdyYWRpZW50Pgo8bGluZWFyR3JhZGllbnQgaWQ9InBhaW50M19saW5lYXJfNjAyMl81MTczMSIgeDE9Ijc5LjMxMjUiIHkxPSIxMDYuMzEyIiB4Mj0iNzkuMzEyNSIgeTI9IjQ4LjkzNzUiIGdyYWRpZW50VW5pdHM9InVzZXJTcGFjZU9uVXNlIj4KPHN0b3Agc3RvcC1jb2xvcj0iIzgzQzFGQSIvPgo8c3RvcCBvZmZzZXQ9IjEiIHN0b3AtY29sb3I9IiM4M0MxRkEiIHN0b3Atb3BhY2l0eT0iMCIvPgo8L2xpbmVhckdyYWRpZW50Pgo8bGluZWFyR3JhZGllbnQgaWQ9InBhaW50NF9saW5lYXJfNjAyMl81MTczMSIgeDE9IjgxIiB5MT0iMTExLjM3NSIgeDI9IjgxIiB5Mj0iMTQ2LjYzOCIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjRjNGRkY3Ii8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0id2hpdGUiLz4KPC9saW5lYXJHcmFkaWVudD4KPGxpbmVhckdyYWRpZW50IGlkPSJwYWludDVfbGluZWFyXzYwMjJfNTE3MzEiIHgxPSI0Ny4xODE4IiB5MT0iMTYuMzIiIHgyPSI2OS41MTIxIiB5Mj0iMjQuMDMzNiIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjMDdDMDVGIi8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0iIzA3QzA1RiIgc3RvcC1vcGFjaXR5PSIwIi8+CjwvbGluZWFyR3JhZGllbnQ+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQ2X2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iOTAuODczNiIgeTE9IjE1LjM3NTYiIHgyPSI5OC43NDk0IiB5Mj0iMzEuNDEzOSIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjQzVDNUM1Ii8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0iI0M1QzVDNSIgc3RvcC1vcGFjaXR5PSIwIi8+CjwvbGluZWFyR3JhZGllbnQ+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQ3X2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iNDQuODI5NyIgeTE9IjU4LjUyMjUiIHgyPSIzNS4xMDcxIiB5Mj0iMzkuNjA0MyIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjQzVDNUM1Ii8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0iI0M1QzVDNSIgc3RvcC1vcGFjaXR5PSIwIi8+CjwvbGluZWFyR3JhZGllbnQ+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQ4X2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iMTIxLjczMiIgeTE9IjM4LjEyNjIiIHgyPSIxMDUuOTc0IiB5Mj0iNDUuODI0NCIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjNTRFODlBIi8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0iIzA3QzA1RiIgc3RvcC1vcGFjaXR5PSIwLjEiLz4KPC9saW5lYXJHcmFkaWVudD4KPGxpbmVhckdyYWRpZW50IGlkPSJwYWludDlfbGluZWFyXzYwMjJfNTE3MzEiIHgxPSI3My42NTYyIiB5MT0iNDQuMzk1NSIgeDI9IjgyLjI0NDYiIHkyPSI1NC4zNzMxIiBncmFkaWVudFVuaXRzPSJ1c2VyU3BhY2VPblVzZSI+CjxzdG9wIHN0b3AtY29sb3I9IiM2RUUxQTUiLz4KPHN0b3Agb2Zmc2V0PSIxIiBzdG9wLWNvbG9yPSIjNkVFMUE1IiBzdG9wLW9wYWNpdHk9IjAiLz4KPC9saW5lYXJHcmFkaWVudD4KPC9kZWZzPgo8L3N2Zz4K';

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

export function SpaceAvatar({ name, size = 'medium', className }: { name: string; size?: 'small' | 'medium' | 'large'; className?: string }) {
  const g = spaceGradient(name || '');
  const first = (name || '').trim().charAt(0);
  const letter = !first ? '?' : (/[a-zA-Z]/.test(first) ? first.toUpperCase() : first);
  return (
    <div
      className={`space-avatar${size === 'small' ? ' space-avatar-small' : size === 'large' ? ' space-avatar-large' : ''}${className ? ` ${className}` : ''}`}
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

/* resourceOrigin.* 文案键尚未进 packages/i18n（agents 同题，报告 gap 记录）——
   字面值直引 Vue zh-CN/en-US 文案。 */
const ORIGIN_TEXT: Record<'mine' | 'tenant' | 'creator', Partial<Record<Locale, string>>> = {
  mine: { 'zh-CN': '我创建', 'en-US': 'Created by me' },
  tenant: { 'zh-CN': '本空间', 'en-US': 'This space' },
  creator: { 'zh-CN': '本空间', 'en-US': 'This space' },
};

export function ResourceOriginBadge({ variant, creatorName, locale }: {
  variant: 'mine' | 'tenant' | 'creator';
  creatorName?: string;
  locale: Locale;
}) {
  const tenantText = ORIGIN_TEXT.tenant[locale] ?? '本空间';
  const text = variant === 'mine'
    ? ORIGIN_TEXT.mine[locale] ?? '我创建'
    : variant === 'creator' ? (creatorName || tenantText)
      : tenantText;
  return (
    <span className={`resource-origin-badge origin-${variant}`}>
      <TIcon name={ORIGIN_ICON[variant] ?? 'usergroup'} size="12px" className="badge-icon" />
      <span className="badge-text">{text}</span>
    </span>
  );
}

// --- KbWikiBadge.vue 端口 ------------------------------------------------------------

function KbWikiBadge({ t }: { t: Translate }) {
  return (
    <Tooltip content={t('knowledgeList.features.wiki')} placement="top">
      <span className="kb-wiki-badge" role="img" aria-label={t('knowledgeList.features.wiki')}>
        <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
          <path d="M12 7v14" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
          <path d="M16 12h2" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
          <path d="M16 8h2" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
          <path
            d="M3 18a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h5a4 4 0 0 1 4 4 4 4 0 0 1 4-4h5a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1h-6a3 3 0 0 0-3 3 3 3 0 0 0-3-3z"
            stroke="currentColor"
            strokeWidth="1.75"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <path d="M6 12h2" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
          <path d="M6 8h2" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </span>
    </Tooltip>
  );
}

// --- 左侧空间栏（ListSpaceSidebar.vue 1:1：collapsed 条带 ↔ expanded 面板） ---------

const RAIL_COLLAPSED_WIDTH = 56;
const RAIL_EXPANDED_WIDTH = 208;
const RAIL_SNAP_THRESHOLD = 120;
const RAIL_MAX_DRAG_WIDTH = RAIL_EXPANDED_WIDTH + 20;
const RAIL_STORAGE_KEY = 'sidebar-collapsed-list-expanded';

export interface KbRailItem {
  key: string;
  label: string;
  icon?: 'layers' | 'star' | 'history' | 'workspace';
  org?: { id: string; name: string };
  count: number | undefined;
  active: boolean;
}

const RAIL_ICON_NAME: Record<NonNullable<KbRailItem['icon']>, string> = {
  layers: 'layers',
  star: 'star',
  history: 'history',
  /* Vue workspace 条目是 t-icon system-sum（ListSpaceSidebar.vue:31）。 */
  workspace: 'system-sum',
};

function ListSpaceSidebar({ t, items, onSelect }: { t: Translate; items: KbRailItem[]; onSelect: (key: string) => void }) {
  const [expanded, setExpanded] = useState(() => {
    try { return window.localStorage.getItem(RAIL_STORAGE_KEY) === 'true'; } catch { return false; }
  });
  const [dragging, setDragging] = useState(false);
  const [dragWidth, setDragWidth] = useState<number | null>(null);
  const drag = useRef<{ startX: number; startWidth: number; width: number } | null>(null);

  const tooltipText = (name: string, count?: number) => (count === undefined ? name : `${name} (${count})`);

  const onDragStart = (event: ReactMouseEvent<HTMLElement>) => {
    event.preventDefault();
    const startWidth = expanded ? RAIL_EXPANDED_WIDTH : RAIL_COLLAPSED_WIDTH;
    drag.current = { startX: event.clientX, startWidth, width: startWidth };
    setDragWidth(startWidth);
    setDragging(true);
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  };

  useEffect(() => {
    if (!dragging) return;
    const onMove = (event: MouseEvent) => {
      const state = drag.current;
      if (!state) return;
      state.width = Math.max(RAIL_COLLAPSED_WIDTH, Math.min(RAIL_MAX_DRAG_WIDTH, state.startWidth + (event.clientX - state.startX)));
      setDragWidth(state.width);
    };
    const onUp = () => {
      const width = drag.current?.width ?? RAIL_COLLAPSED_WIDTH;
      drag.current = null;
      const shouldExpand = width >= RAIL_SNAP_THRESHOLD;
      setExpanded(shouldExpand);
      try { window.localStorage.setItem(RAIL_STORAGE_KEY, String(shouldExpand)); } catch { /* storage unavailable */ }
      setDragging(false);
      setDragWidth(null);
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
    return () => {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };
  }, [dragging]);

  const baseItems = items.filter((item) => !item.org);
  const orgItems = items.filter((item) => item.org);
  /* Expanded-panel count 徽标可见性（ListSpaceSidebar.vue:83/93/101/109）：
     all/mine 有值即渲染（含 0）；favorites/recents 仅 >0。 */
  const countVisible = (item: KbRailItem) => (item.key === 'favorites' || item.key === 'recents' ? (item.count ?? 0) > 0 : item.count !== undefined);

  return (
    <div
      className={`list-space-sidebar${expanded ? ' expanded' : ''}${dragging ? ' dragging' : ''}`}
      style={dragging && dragWidth !== null ? { width: `${dragWidth}px` } : undefined}
    >
      {!expanded ? (
        <div className="icon-strip">
          {baseItems.map((item) => (
            <Tooltip key={item.key} content={tooltipText(item.label, item.count)} placement="right" showArrow={false}>
              <div
                className={`icon-item-labeled${item.key === 'mine' ? ' workspace-item' : ''}${item.active ? ' active' : ''}`}
                data-space-key={item.key}
                onClick={() => onSelect(item.key)}
              >
                <TIcon name={RAIL_ICON_NAME[item.icon ?? 'layers']} size="16px" />
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
                <SpaceAvatar name={item.org?.name ?? item.label} size="small" />
                <span className="icon-label">{truncateLabel(item.org?.name ?? item.label)}</span>
              </div>
            </Tooltip>
          ))}
        </div>
      ) : (
        <nav className="expanded-panel">
          {baseItems.map((item, index) => (
            <Fragment key={item.key}>
              {/* Vue :103：favorites/recents 与 workspace 之间分隔线。 */}
              {index === 3 ? <div className="sidebar-divider" /> : null}
              <div className={`sidebar-item${item.active ? ' active' : ''}`} data-space-key={item.key} onClick={() => onSelect(item.key)}>
                <div className="item-left">
                  <TIcon name={RAIL_ICON_NAME[item.icon ?? 'layers']} className="item-icon" />
                  <span className="item-label">{item.label}</span>
                </div>
                {countVisible(item) ? <span className="item-count">{item.count}</span> : null}
              </div>
            </Fragment>
          ))}
          {orgItems.length > 0 ? (
            <>
              <div className="sidebar-section">
                <span className="section-title">{t('listSpaceSidebar.spaces')}</span>
              </div>
              {orgItems.map((item) => (
                <div key={item.key} className={`sidebar-item org-item${item.active ? ' active' : ''}`} data-space-key={item.key} onClick={() => onSelect(item.key)}>
                  <div className="item-left">
                    <SpaceAvatar name={item.org?.name ?? item.label} size="small" className="item-avatar" />
                    <span className="item-label" title={item.org?.name}>{item.org?.name ?? item.label}</span>
                  </div>
                  {item.count !== undefined ? <span className="item-count">{item.count}</span> : null}
                </div>
              ))}
            </>
          ) : null}
        </nav>
      )}
      {/* Vue .resize-handle (:146-149)：mousedown 起拖，mouseup 按绝对宽度
          （>= 120px）决定展开 / 收起。 */}
      <div className="resize-handle" aria-hidden="true" onMouseDown={onDragStart}>
        <div className="resize-handle-line" />
      </div>
    </div>
  );
}

// --- 分组标题（.kb-section-header，KnowledgeBaseList.vue:97-185） --------------------

const KB_SECTION_ICONS: Record<string, string> = {
  pinned: 'pin-filled',
  mine: 'user',
  tenantOthers: 'usergroup',
  sharedByMe: 'share',
  sharedEditable: 'usergroup-add',
  sharedReadonly: 'usergroup-add',
};

const KB_SECTION_SUBICONS: Record<string, string> = {
  /* Vue :163/:180 子图标是 edit-1 / browse。 */
  sharedEditable: 'edit-1',
  sharedReadonly: 'browse',
};

function KbSectionHeader({ sectionKey, label, count, collapsed, onToggle }: {
  sectionKey: string;
  label: string;
  count: number;
  collapsed: boolean;
  onToggle: (key: string) => void;
}) {
  const subIcon = KB_SECTION_SUBICONS[sectionKey];
  return (
    <div
      className={`kb-section-header${sectionKey === 'pinned' ? ' kb-section-header-pinned' : ''}`}
      role="button"
      tabIndex={0}
      data-kb-section={sectionKey}
      onClick={() => onToggle(sectionKey)}
      onKeyDown={(event) => {
        if (event.key === 'Enter') { event.preventDefault(); onToggle(sectionKey); }
        if (event.key === ' ') { event.preventDefault(); onToggle(sectionKey); }
      }}
    >
      <TIcon name={KB_SECTION_ICONS[sectionKey] ?? 'user'} size="14px" />
      {subIcon ? <TIcon name={subIcon} size="12px" className="kb-section-subicon" /> : null}
      <span>{label}</span>
      <span className="kb-section-count">{count}</span>
      <TIcon className="kb-section-toggle" name={collapsed ? 'chevron-right' : 'chevron-down'} size="14px" />
    </div>
  );
}

// --- 卡片（.kb-card，KnowledgeBaseList.vue:187-372 / 424-529 / 585-629） -------------

interface KbCardProps {
  card: MergedKnowledgeBase;
  t: Translate;
  locale: Locale;
  viewer: Viewer;
  sectionKey: string;
  favorited: boolean;
  highlighted: boolean;
  /** Vue v-show="!isKbSectionCollapsed(kbSectionOf(kb))"：折叠保留 DOM 仅隐藏。 */
  hidden: boolean;
  menuFor: string | null;
  favoriteCountSource: 'own' | 'shared' | 'space';
  onOpen: (card: MergedKnowledgeBase) => void;
  onToggleFavorite: (id: string) => void;
  onToggleMenu: (id: string | null) => void;
  onMenuAction: (action: 'pin' | 'duplicate' | 'settings' | 'delete', card: MergedKnowledgeBase) => void;
  onOpenDetail: (card: MergedKnowledgeBase) => void;
}

function KbCard(props: KbCardProps) {
  const { card, t, locale, viewer, sectionKey, favorited, highlighted, hidden, menuFor, favoriteCountSource, onOpen, onToggleFavorite, onToggleMenu, onMenuAction, onOpenDetail } = props;
  const kb = card as Record<string, unknown>;
  const isSharedCard = card.isMine === false;
  const isSpaceCard = favoriteCountSource === 'space';
  const isFaq = card.type === 'faq';
  const initialized = isKnowledgeBaseInitialized(card as never);
  const manageable = canManageKBCard(kb, { userId: viewer.userId, isAdmin: viewer.isAdmin });
  const duplicable = canDuplicateKBCard(kb, { userId: viewer.userId, isContributor: viewer.isContributor });
  const isWiki = (card as { indexing_strategy?: { wiki_enabled?: boolean } }).indexing_strategy?.wiki_enabled === true;
  const count = isFaq
    ? (typeof card.chunk_count === 'number' ? card.chunk_count : 0)
    : (typeof card.knowledge_count === 'number' ? card.knowledge_count : 0);
  /* Vue :259-260 own `|| 0`；:336-338 / :621-623 shared/space `|| '-'`。 */
  const countText = favoriteCountSource === 'own' ? String(count) : String(count || '-');
  const extractEnabled = (card as { extract_config?: { enabled?: boolean } }).extract_config?.enabled === true;
  const vlmEnabled = (card as { vlm_config?: { enabled?: boolean } }).vlm_config?.enabled === true;
  const questionEnabled = (card as { question_generation_config?: { enabled?: boolean } }).question_generation_config?.enabled === true;
  const shareCount = typeof card.share_count === 'number' ? card.share_count : 0;
  const isProcessing = card.isProcessing === true || (typeof kb.is_processing === 'boolean' ? kb.is_processing : false);
  /* Vue :class 逐字复刻（顺序与对象键序一致）。 */
  const cardClass = [
    'kb-card',
    ...(isSharedCard ? ['shared-kb-card'] : []),
    ...(initialized ? [] : ['uninitialized']),
    ...((card.type || 'document') === 'document' ? ['kb-type-document'] : []),
    ...(isFaq ? ['kb-type-faq'] : []),
    ...(highlighted ? ['highlight-flash'] : []),
  ].join(' ');
  /* 右下角来源徽章（Vue showKbOriginBadge，utils/card-list-badge.ts）。 */
  const variant = isMyKb(kb, viewer.userId) ? 'mine' : 'creator';
  const creatorName = typeof kb.creator_name === 'string' ? kb.creator_name : undefined;
  const showOriginBadge = !isSharedCard && !isSpaceCard
    && shouldShowResourceOriginBadge({ section: sectionKey, variant, creatorName });
  const menuOpen = menuFor === card.id;

  return (
    <div
      className={cardClass}
      data-kb-id={card.id}
      style={hidden ? { display: 'none' } : undefined}
      onClick={() => onOpen(card)}
    >
      {!isSpaceCard ? (
        <button
          type="button"
          className={`kb-favorite-star${favorited ? ' is-favorited' : ''}`}
          onClick={(event) => { event.stopPropagation(); onToggleFavorite(card.id); }}
        >
          <TIcon name={favorited ? 'star-filled' : 'star'} size="14px" />
        </button>
      ) : null}
      {/* 卡片头部 */}
      <div className="card-header">
        <span className="card-title" title={String(card.name ?? '')}>
          {isWiki ? <KbWikiBadge t={t} /> : null}
          <span className="card-title-text">{String(card.name ?? '')}</span>
        </span>
        {isSpaceCard ? (
          /* Vue :595-600：空间视图卡片以 info-circle 触发详情（React 数据面
             per-space 全为共享卡，无 is_mine 分叉）。 */
          <Tooltip content={t('knowledgeList.menu.viewDetails')} placement="top">
            <button
              type="button"
              className="shared-detail-trigger"
              aria-label={t('knowledgeList.menu.viewDetails')}
              onClick={(event) => { event.stopPropagation(); onOpenDetail(card); }}
            >
              <TIcon name="info-circle" size="16px" />
            </button>
          </Tooltip>
        ) : isSharedCard ? (
          /* Vue :311-316：「全部」视图共享卡片的详情触发替代三点菜单。 */
          <Tooltip content={t('knowledgeList.menu.viewDetails')} placement="top">
            <button
              type="button"
              className="shared-detail-trigger"
              aria-label={t('knowledgeList.menu.viewDetails')}
              onClick={(event) => { event.stopPropagation(); onOpenDetail(card); }}
            >
              <TIcon name="info-circle" size="16px" />
            </button>
          </Tooltip>
        ) : (
          <Popup
            visible={menuOpen}
            overlayClassName="card-more-popup"
            trigger="click"
            destroyOnClose
            placement="bottom-right"
            onVisibleChange={(visible) => { if (!visible) onToggleMenu(null); }}
            content={(
              <div className="popup-menu" onClick={(event) => event.stopPropagation()}>
                <div className="popup-menu-item" onClick={() => onMenuAction('pin', card)}>
                  <TIcon className="menu-icon" name={card.is_pinned ? 'pin-filled' : 'pin'} />
                  <span>{card.is_pinned ? t('knowledgeList.pin.unpin') : t('knowledgeList.pin.pin')}</span>
                </div>
                {duplicable ? (
                  <div className="popup-menu-item" onClick={() => onMenuAction('duplicate', card)}>
                    <TIcon className="menu-icon" name="file-copy" />
                    <span>{t('knowledgeList.menu.duplicate')}</span>
                  </div>
                ) : null}
                {manageable ? (
                  <>
                    <div className="popup-menu-item" onClick={() => onMenuAction('settings', card)}>
                      <TIcon className="menu-icon" name="setting" />
                      <span>{t('knowledgeBase.settings')}</span>
                    </div>
                    <div className="popup-menu-item delete" onClick={() => onMenuAction('delete', card)}>
                      <TIcon className="menu-icon" name="delete" />
                      <span>{t('common.delete')}</span>
                    </div>
                  </>
                ) : null}
              </div>
            )}
          >
            <div className="more-wrap" onClick={(event) => { event.stopPropagation(); onToggleMenu(menuOpen ? null : card.id); }}>
              <img className="more-icon" src={MORE_PNG} alt="" />
            </div>
          </Popup>
        )}
      </div>

      {/* 卡片内容 */}
      <div className="card-content">
        <div className="card-description">{String(card.description ?? '') || t('knowledgeBase.noDescription')}</div>
      </div>

      {/* 卡片底部 */}
      <div className="card-bottom">
        <div className="bottom-left">
          <div className="feature-badges">
            <Tooltip
              content={isFaq ? t('knowledgeEditor.basic.typeFAQ') : t('knowledgeEditor.basic.typeDocument')}
              placement="top"
            >
              <div className={`feature-badge${(card.type || 'document') === 'document' ? ' type-document' : ''}${isFaq ? ' type-faq' : ''}`}>
                <TIcon name={isFaq ? 'chat-bubble-help' : 'folder'} size="14px" />
                <span className="badge-count">{countText}</span>
                {isProcessing ? <TIcon name="loading" size="12px" className="processing-icon" /> : null}
              </div>
            </Tooltip>
            {!isSpaceCard && extractEnabled ? (
              <Tooltip content={t('knowledgeList.features.knowledgeGraph')} placement="top">
                <div className="feature-badge kg">
                  <TIcon name="relation" size="14px" />
                </div>
              </Tooltip>
            ) : null}
            {!isSpaceCard && vlmEnabled ? (
              <Tooltip content={t('knowledgeList.features.multimodal')} placement="top">
                <div className="feature-badge multimodal">
                  <TIcon name="image" size="14px" />
                </div>
              </Tooltip>
            ) : null}
            {!isSpaceCard && questionEnabled ? (
              <Tooltip content={t('knowledgeList.features.questionGeneration')} placement="top">
                <div className="feature-badge question">
                  <TIcon name="help-circle" size="14px" />
                </div>
              </Tooltip>
            ) : null}
            {!isSharedCard && !isSpaceCard && shareCount > 0 ? (
              <Tooltip content={t('knowledgeList.sharedToOrgs', { count: shareCount })} placement="top">
                <div className="feature-badge shared">
                  <TIcon name="share" size="14px" />
                </div>
              </Tooltip>
            ) : null}
          </div>
        </div>
        {!isSharedCard && !isSpaceCard && showOriginBadge ? (
          <div className="bottom-right">
            <ResourceOriginBadge variant={variant} creatorName={creatorName} locale={locale} />
          </div>
        ) : null}
        {isSharedCard && !isSpaceCard && typeof card.org_name === 'string' && card.org_name ? (
          <div className="bottom-right">
            <Tooltip content={card.org_name} placement="top">
              <div className="org-source">
                <OrgGreenIcon />
                <span>{card.org_name}</span>
              </div>
            </Tooltip>
          </div>
        ) : null}
      </div>
    </div>
  );
}

// --- 页面 -----------------------------------------------------------------------------

type KbIndexingStrategy = {
  vector_enabled: boolean;
  keyword_enabled: boolean;
  wiki_enabled: boolean;
  graph_enabled: boolean;
};

type KnowledgeEditorOptions = {
  parserEngines: Array<{ Name: string; Description: string; Available?: boolean }>;
  storageBackends: Array<{ id: string; name: string; provider: string; status: string; config?: Record<string, unknown> }>;
  defaultStorageBackendId: string;
  vectorStores: Array<{ id: string; name: string; engine_type: string; source: string; readonly: boolean }>;
  loading: boolean;
  error: string | null;
};

const DEFAULT_KB_INDEXING: KbIndexingStrategy = {
  vector_enabled: true,
  keyword_enabled: true,
  wiki_enabled: false,
  graph_enabled: false,
};

/* Vue navItems 图标（KnowledgeBaseEditorModal.vue:604-634）。 */
const EDITOR_NAV_ICONS: Record<KnowledgeEditorSection, string> = {
  basic: 'info-circle',
  models: 'control-platform',
  vectorStore: 'data-base',
  faq: 'help-circle',
  parser: 'file-search',
  chunking: 'file-copy',
  multimodal: 'image',
  asr: 'sound',
  graph: 'chart-bubble',
  advanced: 'setting',
  storage: 'cloud',
  datasource: 'cloud-download',
  share: 'share',
  activity: 'history',
};

/* Vue navItems label（parser 用 settings.parserEngine，其余 knowledgeEditor.sidebar.*）。 */
const EDITOR_NAV_LABEL_KEYS: Record<KnowledgeEditorSection, string> = {
  basic: 'knowledgeEditor.sidebar.basic',
  models: 'knowledgeEditor.sidebar.models',
  vectorStore: 'knowledgeEditor.sidebar.vectorStore',
  faq: 'knowledgeEditor.sidebar.faq',
  parser: 'settings.parserEngine',
  chunking: 'knowledgeEditor.sidebar.chunking',
  multimodal: 'knowledgeEditor.sidebar.multimodal',
  asr: 'knowledgeEditor.sidebar.asr',
  graph: 'knowledgeEditor.sidebar.graph',
  advanced: 'knowledgeEditor.sidebar.advanced',
  storage: 'knowledgeEditor.sidebar.storage',
  datasource: 'knowledgeEditor.sidebar.datasource',
  share: 'knowledgeEditor.sidebar.share',
  activity: 'knowledgeEditor.sidebar.activity',
};

export function KnowledgeBasesPage({ client, scopeController }: KnowledgeBasesPageProps) {
  const locale = usePreferredLocale();
  const t = useCallback((key: string, values: MessageValues = {}) => formatMessage(locale, key, values), [locale]);

  const [reloadToken, setReloadToken] = useState(0);
  const [pageState, setPageState] = useState<KnowledgeBaseListPageState>({ status: 'loading' });
  const [viewer, setViewer] = useState<Viewer>({ userId: '', isAdmin: false, isContributor: false });
  const [space, setSpaceState] = useState<KbListSpace>(readScopeFromUrl);
  const [query, setQuery] = useState(() => new URLSearchParams(window.location.search).get('q') ?? '');
  const [favorites, setFavorites] = useState<Set<string>>(readFavorites);
  const [recents, setRecents] = useState<Set<string>>(() => {
    try { return new Set(JSON.parse(window.localStorage.getItem('wk-kb-recents') ?? '[]') as string[]); } catch { return new Set(); }
  });
  const [collapsedSections, setCollapsedSections] = useState<ReadonlySet<string>>(new Set());
  const [menuFor, setMenuFor] = useState<string | null>(null);
  const [highlightId, setHighlightId] = useState<string | null>(() => {
    const hl = new URLSearchParams(window.location.search).get('highlightKbId');
    return hl || null;
  });
  const [uploadTasks, setUploadTasks] = useState<UploadTaskState[]>([]);
  const uploadCleanupTimers = useRef(new Map<string, ReturnType<typeof setTimeout>>());
  const uploadRefreshTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Create / edit dialog state. Prefills the full config when editing.
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editorSection, setEditorSection] = useState<KnowledgeEditorSection>('basic');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [type, setType] = useState<'document' | 'faq'>('document');
  const [embeddingModelId, setEmbeddingModelId] = useState('');
  const [summaryModelId, setSummaryModelId] = useState('');
  const [indexingStrategy, setIndexingStrategy] = useState<KbIndexingStrategy>(DEFAULT_KB_INDEXING);
  const [editorConfig, setEditorConfig] = useState<KnowledgeEditorConfig>(defaultKnowledgeEditorConfig);
  const [modelRows, setModelRows] = useState<readonly ModelConfiguration[]>([]);
  // Vue KnowledgeBaseEditorModal.vue:687-693 — a fresh create form preseeds the
  // model config with the tenant defaults once the model rows land.
  useEffect(() => {
    if (!dialogOpen || editingId !== null) return;
    if (summaryModelId && embeddingModelId) return;
    const candidates = modelRows.map((row) => ({
      id: row.id,
      type: typeof row.type === 'string' ? row.type : undefined,
      status: typeof row.status === 'string' ? row.status : undefined,
      is_default: row.is_default === true,
    }));
    if (!summaryModelId) setSummaryModelId(selectInitialModelId(candidates, 'KnowledgeQA') || '');
    if (!embeddingModelId) setEmbeddingModelId(selectInitialModelId(candidates, 'Embedding') || '');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dialogOpen, editingId, modelRows]);
  const [editorOptions, setEditorOptions] = useState<KnowledgeEditorOptions>({ parserEngines: [], storageBackends: [], defaultStorageBackendId: '', vectorStores: [], loading: false, error: null });
  const [editingHasFiles, setEditingHasFiles] = useState(false);
  const editLoadGeneration = useRef(0);
  const [editorActivity, setEditorActivity] = useState<Array<{ id: number; action: string; outcome: string; created_at: string }>>([]);
  const [editorActivityLoading, setEditorActivityLoading] = useState(false);
  const [sharedDetail, setSharedDetail] = useState<Record<string, unknown> | null>(null);
  const [saving, setSaving] = useState(false);

  // Delete confirmation dialog state; DELETE fires only after confirm.
  const [deletingKb, setDeletingKb] = useState<{ id: string; name: string } | null>(null);
  const [deleting, setDeleting] = useState(false);
  const deleteGuard = useRef(createDeleteGuard(client));

  // URL deep-link scope (?scope=) wins; otherwise the default follows the
  // viewer role once known (Vue defaultScope, KnowledgeBaseList.vue:816-830).
  const urlHasScope = useRef(new URLSearchParams(window.location.search).has('scope'));
  const userPickedScope = useRef(false);

  const scope = scopeController.current();
  const queryKey = useMemo(() => scopedKey(scope.scope, 'knowledge-bases'), [scope.scope]);

  const setSpace = (next: KbListSpace) => {
    userPickedScope.current = true;
    setSpaceState(next);
    writeScopeToUrl(next);
  };

  useEffect(() => {
    let active = true;
    scopeController.current().scope.userId && setViewer((current) => ({ ...current, userId: scopeController.current().scope.userId ?? '' }));
    void client.auth.me().then((me) => {
      if (!active) return;
      const tenantId = scopeController.current().scope.tenantId;
      const role = membershipRole(me.memberships, tenantId);
      setViewer({
        userId: typeof me.user.id === 'string' ? me.user.id : '',
        isAdmin: role === 'owner' || role === 'admin' || me.user.is_system_admin === true,
        isContributor: role === 'owner' || role === 'admin' || role === 'contributor' || me.user.is_system_admin === true,
      });
    }).catch(() => { /* gating falls back to creator-id matching only */ });
    return () => { active = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, scopeController]);

  // Vue defaultScope (KnowledgeBaseList.vue:826): contributor -> 'mine'.
  useEffect(() => {
    if (urlHasScope.current || userPickedScope.current || !viewer.isContributor) return;
    setSpaceState((current) => (current === 'all' ? 'mine' : current));
  }, [viewer.isContributor]);

  // Stale-URL cleanup for the legacy scope=shared aggregate view (:1246-1251).
  useEffect(() => {
    if (new URLSearchParams(window.location.search).get('scope') === 'shared') writeScopeToUrl('all');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    let active = true;
    setPageState({ status: 'loading' });
    void loadKnowledgeBaseListPage(client, scope.signal).then((next) => {
      if (active && scopeController.isCurrent(scope.scope)) setPageState(next);
    }).catch(() => { /* aborted */ });
    return () => { active = false; };
  }, [client, reloadToken, scopeController, scope.scope, scope.signal]);

  useEffect(() => {
    if (pageState.status === 'error') console.error('[kb-list] load failed:', pageState.message);
  }, [pageState]);

  // Merged (owned + shared) cards drive the "all" scope and the rail counts.
  const mergedCards = useMemo<MergedKnowledgeBase[]>(() => {
    if (pageState.status !== 'success') return [];
    return mergeAllScopeKnowledgeBases(pageState.owned, pageState.shared, viewer.userId || undefined);
  }, [pageState, viewer.userId]);

  const orgShareData = useMemo(() => {
    const countByOrg = new Map<string, { id: string; name: string; count: number }>();
    const orgIdByShareId = new Map<string, string>();
    if (pageState.status === 'success') {
      for (const entry of pageState.shared) {
        const row = entry as Record<string, unknown>;
        const orgId = typeof row.organization_id === 'string' ? row.organization_id : '';
        if (!orgId) continue;
        if (typeof row.share_id === 'string') orgIdByShareId.set(row.share_id, orgId);
        const name = typeof row.org_name === 'string' && row.org_name ? row.org_name : orgId;
        const bucket = countByOrg.get(orgId);
        if (bucket) bucket.count += 1;
        else countByOrg.set(orgId, { id: orgId, name, count: 1 });
      }
    }
    // Vue organizationsWithCount (ListSpaceSidebar.vue:271-274)：仅正数空间渲染。
    return { orgs: [...countByOrg.values()].filter((org) => org.count > 0), orgIdByShareId };
  }, [pageState]);

  const scopedCards = useMemo<MergedKnowledgeBase[]>(() => {
    if (space === 'mine') {
      if (pageState.status !== 'success') return [];
      return pageState.owned.map((kb) => ({ ...kb, isMine: true as const }));
    }
    if (space === 'favorites') return mergedCards.filter((c) => favorites.has(c.id));
    if (space === 'recents') return mergedCards.filter((c) => recents.has(c.id));
    if (isOrgScope(space)) {
      // Vue per-space view (KnowledgeBaseList.vue:908-913)：该空间共享给我的 KB。
      return mergedCards.filter((card) => card.isMine === false && orgShareData.orgIdByShareId.get(card.share_id) === space);
    }
    return mergedCards;
  }, [favorites, mergedCards, orgShareData, pageState, recents, space]);

  // ?q= keeps working as a deep link (search lives in the command palette).
  const filtered = useMemo(() => filterKnowledgeBases(scopedCards, {
    query,
    currentUserId: viewer.userId || undefined,
    pageSize: Number.MAX_SAFE_INTEGER,
  }), [query, scopedCards, viewer.userId]);

  const sections = useMemo(() => {
    if (pageState.status !== 'success' || isOrgScope(space)) return [];
    // Vue tenantSectionLabelKey：contributor/viewer 读「本空间 · 仅查看」。
    return groupKnowledgeBaseSections(filtered.items, viewer.userId || undefined, { tenantReadonly: !viewer.isAdmin });
  }, [filtered.items, pageState, space, viewer.userId, viewer.isAdmin]);

  const toggleSection = (key: string) => {
    setCollapsedSections((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  // Close the per-card more menu on any outside click / Escape.
  useEffect(() => {
    if (!menuFor) return;
    const close = () => setMenuFor(null);
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') setMenuFor(null); };
    window.addEventListener('click', close);
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('click', close);
      window.removeEventListener('keydown', onKey);
    };
  }, [menuFor]);

  const railItems: KbRailItem[] = [
    { key: 'all', label: t('listSpaceSidebar.all'), icon: 'layers', count: mergedCards.length, active: space === 'all' },
    { key: 'favorites', label: t('listSpaceSidebar.favorites'), icon: 'star', count: favorites.size, active: space === 'favorites' },
    { key: 'recents', label: t('listSpaceSidebar.recents'), icon: 'history', count: recents.size, active: space === 'recents' },
    { key: 'mine', label: t('listSpaceSidebar.workspace'), icon: 'workspace', count: pageState.status === 'success' ? pageState.owned.length : 0, active: space === 'mine' },
    ...orgShareData.orgs.map((org) => ({ key: org.id, label: org.name, count: org.count, org: { id: org.id, name: org.name }, active: space === org.id })),
  ];

  useEffect(() => {
    type UploadEventDetail = { uploadId?: string; kbId?: string | number; fileName?: string; progress?: number; status?: UploadTaskState['status']; error?: string };
    const addOrPatch = (event: Event, fallbackStatus: UploadTaskState['status']) => {
      const detail = (event as CustomEvent<UploadEventDetail>).detail;
      if (!detail?.uploadId || detail.kbId === undefined) return;
      const task: UploadTaskState = {
        uploadId: detail.uploadId,
        kbId: String(detail.kbId),
        fileName: detail.fileName,
        progress: typeof detail.progress === 'number' ? detail.progress : 0,
        status: detail.status ?? fallbackStatus,
        error: detail.error,
      };
      setUploadTasks((current) => upsertUploadTask(current, task));
    };
    const onStart = (event: Event) => addOrPatch(event, 'uploading');
    const onProgress = (event: Event) => {
      const detail = (event as CustomEvent<UploadEventDetail>).detail;
      if (!detail?.uploadId || typeof detail.progress !== 'number') return;
      const uploadId = detail.uploadId;
      const progress = detail.progress;
      if (detail.kbId === undefined) return;
      setUploadTasks((current) => current.some((task) => task.uploadId === uploadId)
        ? patchUploadTask(current, uploadId, { progress })
        : upsertUploadTask(current, { uploadId, kbId: String(detail.kbId), progress, status: 'uploading' }));
    };
    const onComplete = (event: Event) => {
      const detail = (event as CustomEvent<UploadEventDetail>).detail;
      if (!detail?.uploadId || detail.kbId === undefined) return;
      setUploadTasks((current) => upsertUploadTask(current, {
        uploadId: detail.uploadId!, kbId: String(detail.kbId), fileName: detail.fileName,
        progress: typeof detail.progress === 'number' ? detail.progress : 100,
        status: detail.status ?? 'success', error: detail.error,
      }));
      const existing = uploadCleanupTimers.current.get(detail.uploadId);
      if (existing) clearTimeout(existing);
      const timer = setTimeout(() => {
        setUploadTasks((current) => current.filter((task) => task.uploadId !== detail.uploadId));
        uploadCleanupTimers.current.delete(detail.uploadId!);
      }, 10000);
      uploadCleanupTimers.current.set(detail.uploadId, timer);
    };
    const onFinished = (event: Event) => {
      const detail = (event as CustomEvent<{ kbId?: string | number }>).detail;
      if (detail?.kbId === undefined) return;
      if (uploadRefreshTimer.current) clearTimeout(uploadRefreshTimer.current);
      uploadRefreshTimer.current = setTimeout(() => {
        setReloadToken((value) => value + 1);
        uploadRefreshTimer.current = null;
      }, 800);
    };
    window.addEventListener('knowledgeFileUploadStart', onStart);
    window.addEventListener('knowledgeFileUploadProgress', onProgress);
    window.addEventListener('knowledgeFileUploadComplete', onComplete);
    window.addEventListener('knowledgeFileUploaded', onFinished);
    return () => {
      window.removeEventListener('knowledgeFileUploadStart', onStart);
      window.removeEventListener('knowledgeFileUploadProgress', onProgress);
      window.removeEventListener('knowledgeFileUploadComplete', onComplete);
      window.removeEventListener('knowledgeFileUploaded', onFinished);
      uploadCleanupTimers.current.forEach((timer) => clearTimeout(timer));
      uploadCleanupTimers.current.clear();
      if (uploadRefreshTimer.current) clearTimeout(uploadRefreshTimer.current);
      uploadRefreshTimer.current = null;
    };
  }, []);

  const uploadSummaries = useMemo(() => summarizeUploadTasks(uploadTasks, (kbId) => {
    const match = mergedCards.find((card) => String(card.id) === kbId);
    return match ? String(match.name ?? '') : t('knowledgeList.uploadProgress.unknownKb', { id: kbId });
  }), [mergedCards, t, uploadTasks]);

  useEffect(() => {
    if (!highlightId || pageState.status !== 'success') return;
    const escaped = typeof CSS !== 'undefined' && typeof CSS.escape === 'function' ? CSS.escape(highlightId) : highlightId.replace(/[^a-zA-Z0-9_-]/g, '\\$&');
    const element = document.querySelector<HTMLElement>(`[data-kb-id="${escaped}"]`);
    if (element) element.scrollIntoView({ behavior: 'smooth', block: 'center' });
    const url = new URL(window.location.href);
    url.searchParams.delete('highlightKbId');
    window.history.replaceState({}, '', url);
    const timer = setTimeout(() => {
      setHighlightId(null);
    }, 3000);
    return () => clearTimeout(timer);
  }, [highlightId, pageState.status]);

  const hasUninitialized = useMemo(
    () => pageState.status === 'success' && pageState.owned.some((kb) => !isKnowledgeBaseInitialized(kb as never)),
    [pageState],
  );

  // Vue showKbListContextualGuide (KnowledgeBaseList.vue:1195-1197)：kbList tour
  // 的触发条件由 shell 级 guide host 承接（dismissal + welcome-tour 门）。
  const kbListGuideWhen = pageState.status === 'success'
    && viewer.isContributor
    && !dialogOpen
    && (space === 'all' || space === 'mine')
    && scopedCards.length === 0;
  useEffect(() => {
    if (!kbListGuideWhen) return;
    openContextualGuide('kbList');
  }, [kbListGuideWhen]);

  const toggleFavorite = (kbId: string) => {
    setFavorites((current) => {
      const next = new Set(current);
      if (next.has(kbId)) next.delete(kbId);
      else next.add(kbId);
      try { window.localStorage.setItem('wk-kb-favorites', JSON.stringify([...next])); } catch { /* storage unavailable */ }
      return next;
    });
  };

  async function loadEditorOptions() {
    setEditorOptions((current) => ({ ...current, loading: true, error: null }));
    const [parser, storage, vector, models] = await Promise.allSettled([
      client.knowledgeBases.settings.parserEngines(),
      client.knowledgeBases.settings.storageBackends(),
      client.knowledgeBases.settings.vectorStores(),
      client.configuration.models.list(),
    ]);
    if (parser.status === 'rejected' && storage.status === 'rejected' && vector.status === 'rejected') {
      setEditorOptions((current) => ({ ...current, loading: false, error: t('knowledgeEditor.messages.loadDataFailed') }));
      return;
    }
    const nextModelRows = models.status === 'fulfilled' ? models.value : [];
    setModelRows(nextModelRows);
    const defaultStorageId = storage.status === 'fulfilled' && typeof storage.value.default_storage_backend_id === 'string' ? storage.value.default_storage_backend_id : '';
    setEditorOptions({
      parserEngines: parser.status === 'fulfilled' ? parser.value.data.map((item) => ({ Name: item.Name, Description: item.Description, ...(item.Available === undefined ? {} : { Available: item.Available }) })) : [],
      storageBackends: storage.status === 'fulfilled' ? storage.value.data.filter((item) => item.status === 'active').map((item) => ({ id: item.id, name: item.name, provider: item.provider, status: item.status, config: item.config })) : [],
      defaultStorageBackendId: defaultStorageId,
      vectorStores: vector.status === 'fulfilled' ? vector.value.data.map((item) => ({ id: item.id, name: item.name, engine_type: item.engine_type, source: item.source, readonly: item.readonly })) : [],
      loading: false,
      error: null,
    });
    if (defaultStorageId) {
      setEditorConfig((current) => (current.storageBackendId ? current : { ...current, storageBackendId: defaultStorageId }));
    }
  }

  async function loadEditorActivity(id: string) {
    setEditorActivityLoading(true);
    try {
      const result = await client.knowledgeBases.settings.activity(id);
      setEditorActivity((result.data ?? []).map((entry) => ({ id: entry.id, action: entry.action, outcome: entry.outcome, created_at: entry.created_at })));
    } catch {
      setEditorActivity([]);
    } finally {
      setEditorActivityLoading(false);
    }
  }

  const modelsEmbeddingRequired = type === 'document' && (indexingStrategy.vector_enabled || indexingStrategy.keyword_enabled);
  const chatModelOptions: ModelOption[] = resolveChatModelOptions(listChatModels(modelRows)).map((option) => ({ value: option.id, label: option.name }));
  const embeddingModelOptions: ModelOption[] = modelRows
    .filter((row) => row.type === 'Embedding')
    .map((row) => ({ value: row.id, label: String(row.display_name || row.name || row.id) }));

  function openCreate() {
    editLoadGeneration.current += 1; // retire any in-flight edit hydration
    setEditingId(null);
    setName('');
    setDescription('');
    setType('document');
    setEmbeddingModelId('');
    setSummaryModelId('');
    setIndexingStrategy({ ...DEFAULT_KB_INDEXING });
    setEditorConfig(defaultKnowledgeEditorConfig());
    setEditingHasFiles(false);
    void loadEditorOptions();
    setEditorSection('basic');
    // Vue KnowledgeBaseList.vue:1696 — opening the create wizard retires the
    // empty-list kbList tour; the editor arms the kbCreate tour.
    markContextualGuideDone(window.localStorage, 'kbList');
    openContextualGuide('kbCreate', { isFaq: false, needsEmbedding: true });
    setDialogOpen(true);
  }

  function openEdit(kb: Record<string, unknown>) {
    setEditingId(String(kb.id));
    setName(String(kb.name ?? ''));
    setDescription(String(kb.description ?? ''));
    setType(kb.type === 'faq' ? 'faq' : 'document');
    setEmbeddingModelId(String(kb.embedding_model_id ?? ''));
    setSummaryModelId(String(kb.summary_model_id ?? ''));
    const serverStrategy = kb.indexing_strategy as Partial<KbIndexingStrategy> | null | undefined;
    setIndexingStrategy({
      vector_enabled: serverStrategy?.vector_enabled ?? true,
      keyword_enabled: serverStrategy?.keyword_enabled ?? true,
      wiki_enabled: serverStrategy?.wiki_enabled ?? false,
      graph_enabled: serverStrategy?.graph_enabled ?? false,
    });
    setEditorConfig(hydrateKnowledgeEditorConfig(kb));
    setEditingHasFiles(false);
    // R490 A3①（Vue loadKBData）：detail 响应填充 LLM 选择器；文件探测驱动锁。
    const editGeneration = ++editLoadGeneration.current;
    void (async () => {
      try {
        const [detail, files] = await Promise.allSettled([
          client.knowledgeBases.settings.get(String(kb.id)),
          client.knowledgeBases.documents.list(String(kb.id), { page: 1, page_size: 1 }),
        ]);
        if (editGeneration !== editLoadGeneration.current) return; // a newer open took over
        const fileTotal = files.status === 'fulfilled' ? Number(files.value.total ?? files.value.data?.length ?? 0) : 0;
        setEditingHasFiles(fileTotal > 0);
        if (detail.status !== 'fulfilled') return; // keep the list-row first paint
        const row = detail.value as Record<string, unknown>;
        setSummaryModelId(typeof row.summary_model_id === 'string' ? row.summary_model_id : '');
        setEmbeddingModelId(typeof row.embedding_model_id === 'string' ? row.embedding_model_id : '');
      } catch { /* keep the list-row first paint (degraded client surface) */ }
    })();
    void loadEditorOptions();
    setEditorSection('basic');
    setDialogOpen(true);
  }

  // Vue KnowledgeBaseEditorModal.vue:1142-1191 validateForm：提交前 UI 管道校验，
  // 失败跳区 + MessagePlugin.warning。
  function validateEditorForm(): boolean {
    if (!name.trim()) {
      MessagePlugin.warning(t('knowledgeEditor.messages.nameRequired'));
      setEditorSection('basic');
      return false;
    }
    if (type !== 'faq') {
      if (!indexingStrategy.vector_enabled && !indexingStrategy.keyword_enabled && !indexingStrategy.wiki_enabled && !indexingStrategy.graph_enabled) {
        MessagePlugin.warning(t('knowledgeEditor.indexing.atLeastOne'));
        setEditorSection('basic');
        return false;
      }
    }
    const needsEmbedding = indexingStrategy.vector_enabled || indexingStrategy.keyword_enabled;
    if (needsEmbedding && !embeddingModelId) {
      MessagePlugin.warning(t('knowledgeEditor.indexing.embeddingRequired'));
      setEditorSection('models');
      return false;
    }
    if (!summaryModelId) {
      MessagePlugin.warning(t('knowledgeEditor.messages.summaryRequired'));
      setEditorSection('models');
      return false;
    }
    if (editorConfig.multimodalConfig.enabled && !editorConfig.multimodalConfig.vllmModelId.trim()) {
      MessagePlugin.warning(t('knowledgeEditor.messages.multimodalInvalid'));
      setEditorSection('multimodal');
      return false;
    }
    if (type === 'faq' && !editorConfig.faqConfig.indexMode) {
      MessagePlugin.warning(t('knowledgeEditor.messages.indexModeRequired'));
      setEditorSection('faq');
      return false;
    }
    return true;
  }

  // Vue handleSubmit → doSubmit（:1335-1531）：footer 按钮纯 @click，无原生表单。
  async function save() {
    if (saving) return; // double-submit guard
    if (!validateEditorForm()) return;
    setSaving(true);
    const input: KnowledgeBaseSaveInput = {
      name: name.trim(),
      type,
      ...(description.trim() ? { description: description.trim() } : {}),
      ...(embeddingModelId ? { embedding_model_id: embeddingModelId } : {}),
      ...(summaryModelId ? { summary_model_id: summaryModelId } : {}),
      ...knowledgeEditorConfigPayload(editorConfig, type),
      ...(type === 'document' ? { indexing_strategy: { ...indexingStrategy, graph_enabled: editorConfig.nodeExtractConfig.enabled } } : {}),
    };
    try {
      const record = await saveKnowledgeBase(client, editingId, input) as { id?: unknown };
      // Vue :1386 — a successful editor run retires the kbCreate tour.
      markContextualGuideDone(window.localStorage, 'kbCreate');
      setDialogOpen(false);
      setEditingId(null);
      setReloadToken((value) => value + 1);
      // Vue handleKBEditorSuccess：kbDetail tour 未跑过时落在新 KB 详情页。
      if (editingId === null && record?.id !== undefined && !isContextualGuideDone(window.localStorage, 'kbDetail')) {
        openContextualGuide('kbDetail');
        navigate(knowledgeBaseDetailPath(String(record.id)));
      }
    } catch (saveError) {
      MessagePlugin.error(saveError instanceof Error ? saveError.message : t('common.operationFailed'));
    } finally {
      setSaving(false);
    }
  }

  async function confirmDelete() {
    if (!deletingKb || deleting) return; // exactly one DELETE per confirmed action
    setDeleting(true);
    try {
      const result = await deleteGuard.current.confirm(deletingKb.id);
      if (result === 'deleted') {
        setDeletingKb(null);
        MessagePlugin.success(t('knowledgeList.messages.deleted'));
        setReloadToken((value) => value + 1);
      }
    } catch (deleteError) {
      MessagePlugin.error(deleteError instanceof Error ? deleteError.message : t('knowledgeList.messages.deleteFailed'));
    } finally {
      setDeleting(false);
    }
  }

  async function togglePin(kb: Record<string, unknown>) {
    try {
      const result = await client.knowledgeBases.togglePin(String(kb.id));
      MessagePlugin.success(t(result.is_pinned ? 'knowledgeList.pin.pinSuccess' : 'knowledgeList.pin.unpinSuccess'));
      setReloadToken((value) => value + 1);
    } catch {
      MessagePlugin.error(t('knowledgeList.pin.failed'));
    }
  }

  async function duplicate(kb: Record<string, unknown>) {
    try {
      await client.knowledgeBases.duplicate(String(kb.id));
      MessagePlugin.success(t('knowledgeList.messages.duplicateSuccess'));
      setReloadToken((value) => value + 1);
    } catch {
      MessagePlugin.error(t('knowledgeList.messages.duplicateFailed'));
    }
  }

  function onMenuAction(action: 'pin' | 'duplicate' | 'settings' | 'delete', card: MergedKnowledgeBase) {
    setMenuFor(null);
    const kb = card as Record<string, unknown>;
    if (action === 'pin') void togglePin(kb);
    else if (action === 'duplicate') void duplicate(kb);
    else if (action === 'settings') openEdit(kb);
    else setDeletingKb({ id: card.id, name: String(card.name ?? '') });
  }

  // Vue openSharedDetailFromAll (:1499-1505)：按 share_id 找回原始共享行。
  function openSharedDetail(card: MergedKnowledgeBase) {
    const rows = pageState.status === 'success' ? (pageState.shared as Array<Record<string, unknown>>) : [];
    const raw = typeof card.share_id === 'string' ? rows.find((row) => row.share_id === card.share_id) : undefined;
    setSharedDetail(raw ?? {
      name: card.name,
      permission: card.permission,
      shared_at: card.shared_at,
      org_name: typeof card.org_name === 'string' ? card.org_name : undefined,
      knowledge_base: { id: card.id },
    });
  }

  function openCard(kbCard: MergedKnowledgeBase) {
    const kb = kbCard as Record<string, unknown>;
    const id = String(kb.id);
    try {
      const raw = window.localStorage.getItem('wk-kb-recents');
      const list: string[] = raw ? JSON.parse(raw) : [];
      const next = [id, ...list.filter((v) => v !== id)].slice(0, 20);
      window.localStorage.setItem('wk-kb-recents', JSON.stringify(next));
      setRecents(new Set(next));
    } catch { /* storage unavailable */ }
    if (isKnowledgeBaseInitialized(kb as never)) {
      // Vue KnowledgeBase.vue:339-345 — kbDetail tour 在空可编辑文档库详情入口武装。
      const count = typeof kb.knowledge_count === 'number' ? kb.knowledge_count : 0;
      if (kb.type !== 'faq' && count === 0 && canManageKBCard(kb, { userId: viewer.userId, isAdmin: viewer.isAdmin })) {
        openContextualGuide('kbDetail');
      }
      navigate(knowledgeBaseDetailPath(id));
      return;
    }
    // Vue handleCardClick else-branch：未初始化卡片点开就地设置弹窗。
    openEdit(kb);
  }

  const isLoading = pageState.status === 'loading';
  const listVisible = pageState.status !== 'loading' && filtered.items.length > 0;
  /* Vue authority：列表加载失败渲染与空结果同样的空态（无裸错误输出）。 */
  const emptyVisible = pageState.status === 'error' || (pageState.status === 'success' && filtered.items.length === 0);

  /* 编辑器留守段的容器锚点类（kb-editor-parity.css 留守段消费）。 */
  const sectionShell = (key: KnowledgeEditorSection, visible: boolean, children: ReactNode) => (
    <div className="section" data-editor-section={key} style={visible ? undefined : { display: 'none' }}>{children}</div>
  );

  const editorGroups = visibleKnowledgeEditorSections({ type, editing: Boolean(editingId) });
  const saveButtonLabel = editingId ? t('knowledgeEditor.buttons.saveAndClose') : t('knowledgeEditor.buttons.create');
  const isIndexingLocked = Boolean(editingId) && editingHasFiles;

  const toggleVectorIndexing = () => {
    if (isIndexingLocked) return;
    setIndexingStrategy((current) => ({ ...current, vector_enabled: !current.vector_enabled, keyword_enabled: !current.vector_enabled }));
  };
  const toggleWikiIndexing = () => {
    if (isIndexingLocked) return;
    setIndexingStrategy((current) => ({ ...current, wiki_enabled: !current.wiki_enabled }));
  };

  return (
    <div className="kb-list-container">
      <ListSpaceSidebar t={t} items={railItems} onSelect={setSpace} />
      <div className="kb-list-content">
        <div className="header" style={{ '--wails-draggable': 'drag' } as CSSProperties}>
          <div className="header-title" style={{ '--wails-draggable': 'drag' } as CSSProperties}>
            <div className="title-row" style={{ '--wails-draggable': 'drag' } as CSSProperties}>
              <h2 style={{ '--wails-draggable': 'drag' } as CSSProperties}>{t('knowledgeBase.title')}</h2>
              {viewer.isContributor ? (
                /* Vue :11-16：t-tooltip + 纯图标 t-button（无 aria/title——扫描器
                   clickCss data-guide 兜底两端同构命中）。 */
                <Tooltip content={t('knowledgeList.create')} placement="bottom">
                  <Button
                    variant="text"
                    theme="default"
                    size="small"
                    className="header-action-btn"
                    data-guide="kb-list-create"
                    style={{ '--wails-draggable': 'no-drag' } as CSSProperties}
                    icon={<TIcon name="folder-add" size="16px" />}
                    onClick={openCreate}
                  />
                </Tooltip>
              ) : null}
            </div>
            <p className="header-subtitle" style={{ '--wails-draggable': 'drag' } as CSSProperties}>{t('knowledgeList.subtitle')}</p>
          </div>
        </div>
        <div className="kb-list-main">
          {/* 未初始化知识库提示（Vue :31-34） */}
          {hasUninitialized ? (
            <div className="warning-banner">
              <TIcon name="info-circle" size="16px" />
              <span>{t('knowledgeList.uninitializedBanner')}</span>
            </div>
          ) : null}

          {/* 上传进度提示（Vue :37-72） */}
          {uploadSummaries.length ? (
            <div className="upload-progress-panel">
              {uploadSummaries.map((summary) => (
                <div className="upload-progress-item" key={summary.kbId}>
                  <div className="upload-progress-icon">
                    <TIcon name={summary.completed === summary.total ? 'check-circle-filled' : 'upload'} size="20px" />
                  </div>
                  <div className="upload-progress-content">
                    <div className="progress-title">
                      {summary.completed === summary.total
                        ? t('knowledgeList.uploadProgress.completedTitle', { name: summary.kbName })
                        : t('knowledgeList.uploadProgress.uploadingTitle', { name: summary.kbName })}
                    </div>
                    <div className="progress-subtitle">
                      {summary.completed === summary.total
                        ? t('knowledgeList.uploadProgress.completedDetail', { total: summary.total })
                        : t('knowledgeList.uploadProgress.detail', { completed: summary.completed, total: summary.total })}
                    </div>
                    <div className="progress-subtitle secondary">
                      {summary.completed === summary.total
                        ? t('knowledgeList.uploadProgress.refreshing')
                        : t('knowledgeList.uploadProgress.keepPageOpen')}
                    </div>
                    {summary.hasError ? <div className="progress-subtitle error">{t('knowledgeList.uploadProgress.errorTip')}</div> : null}
                    <div className="progress-bar">
                      <div className="progress-bar-inner" style={{ width: `${summary.progress}%` }} />
                    </div>
                  </div>
                </div>
              ))}
            </div>
          ) : null}

          {/* 骨架屏占位（Vue :75-89：t-skeleton rowCol 三段） */}
          {isLoading ? (
            <div className="kb-card-wrap">
              {Array.from({ length: 6 }, (_, index) => (
                <div className="kb-card kb-card-skeleton" key={index}>
                  <div className="card-header">
                    <Skeleton animation="gradient" rowCol={[{ width: '60%', height: '20px' }]} />
                  </div>
                  <div className="card-content">
                    <Skeleton animation="gradient" rowCol={[{ width: '100%', height: '14px' }, { width: '80%', height: '14px' }]} />
                  </div>
                  <div className="card-bottom">
                    <Skeleton animation="gradient" rowCol={[[{ width: '28px', height: '28px', type: 'rect' }, { width: '28px', height: '28px', type: 'rect' }]]} />
                  </div>
                </div>
              ))}
            </div>
          ) : null}

          {/* 卡片网格：全部 / 收藏 / 最近 / 我的知识库（共用卡片模板）。 */}
          {listVisible && !isOrgScope(space) ? (
            <div className="kb-card-wrap">
              {sections.length === 0
                ? filtered.items.map((card) => (
                  <KbCard
                    key={card.id}
                    card={card}
                    t={t}
                    locale={locale}
                    viewer={viewer}
                    sectionKey=""
                    favorited={favorites.has(card.id)}
                    highlighted={highlightId === card.id}
                    hidden={false}
                    menuFor={menuFor}
                    favoriteCountSource="own"
                    onOpen={openCard}
                    onToggleFavorite={toggleFavorite}
                    onToggleMenu={setMenuFor}
                    onMenuAction={onMenuAction}
                    onOpenDetail={openSharedDetail}
                  />
                ))
                : sections.map((section) => (
                  <Fragment key={section.key}>
                    <KbSectionHeader
                      sectionKey={section.key}
                      label={t(section.labelKey)}
                      count={section.items.length}
                      collapsed={collapsedSections.has(section.key)}
                      onToggle={toggleSection}
                    />
                    {section.items.map((card) => (
                      <KbCard
                        key={card.id}
                        card={card}
                        t={t}
                        locale={locale}
                        viewer={viewer}
                        sectionKey={section.key}
                        favorited={favorites.has(card.id)}
                        highlighted={highlightId === card.id}
                        hidden={collapsedSections.has(section.key)}
                        menuFor={menuFor}
                        favoriteCountSource={card.isMine === false ? 'shared' : 'own'}
                        onOpen={openCard}
                        onToggleFavorite={toggleFavorite}
                        onToggleMenu={setMenuFor}
                        onMenuAction={onMenuAction}
                        onOpenDetail={openSharedDetail}
                      />
                    ))}
                  </Fragment>
                ))}
            </div>
          ) : null}

          {/* 按空间筛选（React 数据面：orgScope 平铺共享卡片，Vue 空间态卡片
              无收藏星/无右下角 chip——favoriteCountSource="space"）。 */}
          {listVisible && isOrgScope(space) ? (
            <div className="kb-card-wrap">
              {filtered.items.map((card) => (
                <KbCard
                  key={card.id}
                  card={card}
                  t={t}
                  locale={locale}
                  viewer={viewer}
                  sectionKey=""
                  favorited={favorites.has(card.id)}
                  highlighted={highlightId === card.id}
                  hidden={false}
                  menuFor={menuFor}
                  favoriteCountSource="space"
                  onOpen={openCard}
                  onToggleFavorite={toggleFavorite}
                  onToggleMenu={setMenuFor}
                  onMenuAction={onMenuAction}
                  onOpenDetail={openSharedDetail}
                />
              ))}
            </div>
          ) : null}

          {/* 全部空状态：保留「新建知识库」CTA（Vue :634-643） */}
          {emptyVisible && space === 'all' && !isLoading ? (
            <div className="empty-state">
              <img className="empty-img" src={UPLOAD_SVG} alt="" />
              <span className="empty-txt">{t('knowledgeList.empty.title')}</span>
              <span className="empty-desc">{t('knowledgeList.empty.description')}</span>
              {viewer.isContributor ? (
                <Button className="kb-create-btn empty-state-btn" data-guide="kb-list-create" icon={<TIcon name="folder-add" />} onClick={openCreate}>
                  {t('knowledgeList.create')}
                </Button>
              ) : null}
            </div>
          ) : null}

          {/* 收藏空状态：不放创建按钮（Vue :645-652） */}
          {emptyVisible && space === 'favorites' && !isLoading ? (
            <div className="empty-state">
              <TIcon name="star" size="48px" className="empty-icon" />
              <span className="empty-txt">{t('knowledgeList.empty.favoritesTitle')}</span>
              <span className="empty-desc">{t('knowledgeList.empty.favoritesDescription')}</span>
            </div>
          ) : null}

          {/* 最近空状态（Vue :654-659） */}
          {emptyVisible && space === 'recents' && !isLoading ? (
            <div className="empty-state">
              <TIcon name="history" size="48px" className="empty-icon" />
              <span className="empty-txt">{t('knowledgeList.empty.recentsTitle')}</span>
              <span className="empty-desc">{t('knowledgeList.empty.recentsDescription')}</span>
            </div>
          ) : null}

          {/* 我的知识库空状态（Vue :661-671） */}
          {emptyVisible && space === 'mine' && !isLoading ? (
            <div className="empty-state">
              <img className="empty-img" src={UPLOAD_SVG} alt="" />
              <span className="empty-txt">{t('knowledgeList.empty.title')}</span>
              <span className="empty-desc">{t('knowledgeList.empty.description')}</span>
              {viewer.isContributor ? (
                <Button className="kb-create-btn empty-state-btn" data-guide="kb-list-create" icon={<TIcon name="folder-add" />} onClick={openCreate}>
                  {t('knowledgeList.create')}
                </Button>
              ) : null}
            </div>
          ) : null}

          {/* 空间下知识库空状态（Vue :673-678） */}
          {emptyVisible && isOrgScope(space) && !isLoading ? (
            <div className="empty-state">
              <img className="empty-img" src={UPLOAD_SVG} alt="" />
              <span className="empty-txt">{t('knowledgeList.empty.sharedTitle')}</span>
              <span className="empty-desc">{t('knowledgeList.empty.sharedDescription')}</span>
            </div>
          ) : null}
        </div>
      </div>

      {/* 删除确认对话框（Vue :683-699：t-dialog + circle-wrap 自绘内容） */}
      <Dialog
        visible={deletingKb !== null}
        dialogClassName="del-knowledge-dialog"
        closeBtn={false}
        cancelBtn={null}
        confirmBtn={null}
        onClose={() => { if (!deleting) setDeletingKb(null); }}
      >
        <div className="circle-wrap">
          <div className="dialog-header">
            <img className="circle-img" src={CIRCLE_PNG} alt="" />
            <span className="circle-title">{t('knowledgeList.delete.confirmTitle')}</span>
          </div>
          <span className="del-circle-txt">
            {t('knowledgeList.delete.confirmMessage', { name: deletingKb?.name ?? '' })}
          </span>
          <div className="circle-btn">
            <span className="circle-btn-txt" onClick={() => { if (!deleting) setDeletingKb(null); }}>{t('common.cancel')}</span>
            <span className="circle-btn-txt confirm" onClick={() => { void confirmDelete(); }}>{t('knowledgeList.delete.confirmButton')}</span>
          </div>
        </div>
      </Dialog>

      {/* 知识库编辑器（创建/编辑统一组件，Vue Teleport body → createPortal） */}
      {dialogOpen ? createPortal(
        <div className="settings-overlay" onClick={(event) => { if (event.target === event.currentTarget) setDialogOpen(false); }}>
          <div className="settings-modal wk-kb-editor-dialog">
            {editorOptions.loading ? (
              <div className="editor-initializing" role="status" aria-label={t('common.loading')}>
                <Loading size="medium" text={t('common.loading')} />
              </div>
            ) : null}
            {/* 关闭按钮（Vue :10-14 内联 X svg） */}
            <button className="close-btn" aria-label={t('general.close')} onClick={() => setDialogOpen(false)}>
              <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
                <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
              </svg>
            </button>
            <div className="settings-container">
              {/* 左侧导航 */}
              <div className="settings-sidebar">
                <div className="sidebar-header">
                  <h2 className="sidebar-title">{editingId ? t('knowledgeEditor.titleEdit') : t('knowledgeEditor.titleCreate')}</h2>
                </div>
                <div className="settings-nav" data-guide="kb-editor-sidebar">
                  {editorGroups.map((group) => (
                    <Fragment key={group.key}>
                      <div className="nav-group-title">{t(group.labelKey)}</div>
                      {group.items.map((section) => (
                        <div
                          key={section}
                          className={`nav-item${editorSection === section ? ' active' : ''}`}
                          data-guide={`kb-editor-nav-${section}`}
                          onClick={() => { setEditorSection(section); if (section === 'activity' && editingId) void loadEditorActivity(editingId); }}
                        >
                          <TIcon name={EDITOR_NAV_ICONS[section]} className="nav-icon" />
                          <span className="nav-label">{t(EDITOR_NAV_LABEL_KEYS[section])}</span>
                        </div>
                      ))}
                    </Fragment>
                  ))}
                </div>
              </div>
              {/* 右侧内容区域 */}
              <div className="settings-content">
                <div className="content-wrapper kb-editor-content">
                  {/* 基本信息（Vue v-show） */}
                  <div className="section" data-editor-section="basic" style={editorSection === 'basic' ? undefined : { display: 'none' }}>
                    <div className="section-content">
                      <div className="section-header">
                        <h3 className="section-title">{t('knowledgeEditor.basic.title')}</h3>
                        <p className="section-desc">{t('knowledgeEditor.basic.description')}</p>
                      </div>
                      <div className="section-body">
                        {editingId ? (
                          <div className="form-item">
                            <label className="form-label">{t('knowledgeEditor.basic.kbId')}</label>
                            <p className="form-tip">{t('knowledgeEditor.basic.kbIdDesc')}</p>
                            <div className="kb-id-field">
                              <code className="kb-id-value" title={editingId}>{editingId}</code>
                              <Tooltip content={t('common.copy')} placement="top">
                                <Button theme="default" size="small" variant="text" className="kb-id-copy" icon={<TIcon name="file-copy" />} onClick={() => { void navigator.clipboard?.writeText(editingId).catch(() => undefined); }} />
                              </Tooltip>
                            </div>
                          </div>
                        ) : null}
                        <div className="form-item" data-guide="kb-create-type">
                          <label className="form-label required">{t('knowledgeEditor.basic.typeLabel')}</label>
                          {/* Vue t-radio-group 默认 outline 连体按钮组（:66-74）。 */}
                          <RadioGroup
                            value={type}
                            disabled={Boolean(editingId)}
                            onChange={(value) => {
                              const next = value === 'faq' ? 'faq' : 'document';
                              setType(next);
                              setEditorSection(next === 'faq' ? 'faq' : 'basic');
                            }}
                          >
                            <Radio.Button value="document">{t('knowledgeEditor.basic.typeDocument')}</Radio.Button>
                            <Radio.Button value="faq">{t('knowledgeEditor.basic.typeFAQ')}</Radio.Button>
                          </RadioGroup>
                          <p className="form-tip">{t('knowledgeEditor.basic.typeDescription')}</p>
                        </div>
                        {/* 索引策略（紧跟类型选择，Vue :79-117） */}
                        {type !== 'faq' ? (
                          <div className="form-item">
                            <label className="form-label required">{t('knowledgeEditor.indexing.title')}</label>
                            <p className="form-tip">{t('knowledgeEditor.indexing.description')}</p>
                            <div className={`indexing-checks${isIndexingLocked ? ' is-locked' : ''}`} data-guide="kb-create-indexing">
                              <div
                                className={`indexing-check-item${indexingStrategy.vector_enabled ? ' is-checked' : ''}${isIndexingLocked ? ' is-disabled' : ''}`}
                                onClick={toggleVectorIndexing}
                              >
                                <Checkbox checked={indexingStrategy.vector_enabled} disabled={isIndexingLocked} className="indexing-check-box">
                                  {t('knowledgeEditor.indexing.searchTitle')}
                                </Checkbox>
                                <p className="indexing-check-desc">{t('knowledgeEditor.indexing.searchDesc')}</p>
                              </div>
                              <div
                                className={`indexing-check-item${indexingStrategy.wiki_enabled ? ' is-checked' : ''}${isIndexingLocked ? ' is-disabled' : ''}`}
                                onClick={toggleWikiIndexing}
                              >
                                <Checkbox checked={indexingStrategy.wiki_enabled} disabled={isIndexingLocked} className="indexing-check-box">
                                  <span className="indexing-check-title">
                                    {t('knowledgeEditor.indexing.wikiTitle')}<span className="indexing-new-badge">NEW</span>
                                  </span>
                                </Checkbox>
                                <p className="indexing-check-desc">{t('knowledgeEditor.indexing.wikiDesc')}</p>
                              </div>
                            </div>
                            {isIndexingLocked ? <p className="form-tip locked-tip" data-indexing-locked-tip="">{t('knowledgeEditor.indexing.lockedTip')}</p> : null}
                          </div>
                        ) : null}
                        {/* Wiki 提取粒度（仅当 Wiki 启用时显示，Vue :120-161） */}
                        {type !== 'faq' && indexingStrategy.wiki_enabled ? (
                          <>
                            <div className="form-item">
                              <label className="form-label">{t('knowledgeEditor.wiki.extractionGranularityLabel')}</label>
                              <p className="form-tip">{t('knowledgeEditor.wiki.extractionGranularityTip')}</p>
                              <RadioGroup
                                value={editorConfig.wikiConfig?.extractionGranularity ?? 'standard'}
                                className="granularity-radio-group"
                                onChange={(value) => {
                                  const next = value === 'focused' || value === 'exhaustive' ? value : 'standard';
                                  setEditorConfig((current) => ({ ...current, wikiConfig: { ...current.wikiConfig, extractionGranularity: next } }));
                                }}
                              >
                                <Radio.Button value="focused">{t('knowledgeEditor.wiki.granularityFocused')}</Radio.Button>
                                <Radio.Button value="standard">{t('knowledgeEditor.wiki.granularityStandard')}</Radio.Button>
                                <Radio.Button value="exhaustive">{t('knowledgeEditor.wiki.granularityExhaustive')}</Radio.Button>
                              </RadioGroup>
                              <p className="form-tip granularity-hint">{t(`knowledgeEditor.wiki.granularity${(editorConfig.wikiConfig?.extractionGranularity ?? 'standard') === 'focused' ? 'Focused' : (editorConfig.wikiConfig?.extractionGranularity ?? 'standard') === 'exhaustive' ? 'Exhaustive' : 'Standard'}Hint`)}</p>
                            </div>
                            <div className="form-item">
                              <label className="form-label">{t('knowledgeEditor.wiki.contentInstructionsLabel')}</label>
                              <p className="form-tip">{t('knowledgeEditor.wiki.contentInstructionsTip')}</p>
                              <Textarea
                                value={editorConfig.wikiConfig?.contentInstructions ?? ''}
                                placeholder={t('knowledgeEditor.wiki.contentInstructionsPlaceholder')}
                                maxlength={4000}
                                autosize={{ minRows: 3, maxRows: 8 }}
                                onChange={(value) => setEditorConfig((current) => ({ ...current, wikiConfig: { ...current.wikiConfig, contentInstructions: String(value) } }))}
                              />
                            </div>
                            <div className="form-item">
                              <label className="form-label">{t('knowledgeEditor.wiki.extractionInstructionsLabel')}</label>
                              <p className="form-tip">{t('knowledgeEditor.wiki.extractionInstructionsTip')}</p>
                              <Textarea
                                value={editorConfig.wikiConfig?.extractionInstructions ?? ''}
                                placeholder={t('knowledgeEditor.wiki.extractionInstructionsPlaceholder')}
                                maxlength={4000}
                                autosize={{ minRows: 3, maxRows: 8 }}
                                onChange={(value) => setEditorConfig((current) => ({ ...current, wikiConfig: { ...current.wikiConfig, extractionInstructions: String(value) } }))}
                              />
                            </div>
                          </>
                        ) : null}
                        <div className="form-item" data-guide="kb-create-name">
                          <label className="form-label required">{t('knowledgeEditor.basic.nameLabel')}</label>
                          {/* Vue t-input :165-169 仅 maxlength；空名由 JS 管道拦截。 */}
                          <Input value={name} placeholder={t('knowledgeEditor.basic.namePlaceholder')} maxlength={50} onChange={(value) => setName(String(value))} />
                        </div>
                        <div className="form-item">
                          <label className="form-label">{t('knowledgeEditor.basic.descriptionLabel')}</label>
                          <Textarea
                            value={description}
                            placeholder={t('knowledgeEditor.basic.descriptionPlaceholder')}
                            maxlength={200}
                            autosize={{ minRows: 3, maxRows: 6 }}
                            onChange={(value) => setDescription(String(value))}
                          />
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* 模型配置（留守段：R490 React 端口 internals） */}
                  {sectionShell('models', editorSection === 'models', (
                    <div className="grid gap-4">
                      <div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.models.title')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('knowledgeEditor.models.description')}</p></div>
                      <div className="grid gap-1" data-guide="kb-create-llm"><label className="text-[15px] font-medium text-ink">{t('knowledgeEditor.models.llmLabel')}<span className="ml-1 text-[#e34d59]">*</span></label><p className="m-0 text-[13px] leading-[1.5] text-muted">{t('knowledgeEditor.models.llmDesc')}</p><ModelOptionSelect value={summaryModelId} options={chatModelOptions} addModelLabel={t('model.addModelInSettings')} onAddModel={() => navigate('/platform/settings?section=models&subsection=chat')} onChange={setSummaryModelId} /></div>
                      <div className="grid gap-1" data-guide="kb-create-embedding"><label className="text-[15px] font-medium text-ink">{t('knowledgeEditor.models.embeddingLabel')}{modelsEmbeddingRequired ? <span className="ml-1 text-[#e34d59]">*</span> : null}</label><p className="m-0 text-[13px] leading-[1.5] text-muted">{t('knowledgeEditor.models.embeddingDesc')}</p>{modelsEmbeddingRequired && editingHasFiles ? <p className="m-0 text-[13px] leading-[1.5] text-muted" data-embedding-locked-tip="" style={{ margin: '0.5rem 0 0', fontSize: '0.85rem' }}>{t('knowledgeEditor.models.embeddingLocked')}</p> : null}<ModelOptionSelect value={embeddingModelId} options={embeddingModelOptions} disabled={modelsEmbeddingRequired && editingHasFiles} addModelLabel={t('model.addModelInSettings')} onAddModel={() => navigate('/platform/settings?section=models&subsection=embedding')} onChange={setEmbeddingModelId} /></div>
                    </div>
                  ))}

                  {/* VectorStore 绑定（留守段） */}
                  {sectionShell('vectorStore', editorSection === 'vectorStore', (
                    <div className="grid gap-4"><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.sidebar.vectorStore')}</h3><p className="m-0 text-sm leading-[22px] text-muted">{t('kbSettings.vectorStore.description')}</p><label className="grid gap-1">{t('kbSettings.vectorStore.engineLabel')}<TSelect value={editorConfig.vectorStoreId} disabled={Boolean(editingId) || editorOptions.loading} options={[{ value: '', label: t('kbSettings.vectorStore.systemDefault') }, ...editorOptions.vectorStores.map((store) => ({ value: store.id, label: `${store.name} · ${store.engine_type}` }))]} onChange={(value) => setEditorConfig((current) => ({ ...current, vectorStoreId: String(value) }))} /></label><p className="m-0 text-xs leading-[18px] text-muted">{editingId ? t('kbSettings.vectorStore.immutableHint') : editorOptions.error ?? t('kbSettings.vectorStore.engineDesc')}</p></div>
                  ))}

                  {/* FAQ 配置（Vue :215-247 DOM） */}
                  {type === 'faq' ? sectionShell('faq', editorSection === 'faq', (
                    <div className="section-content">
                      <div className="section-header">
                        <h3 className="section-title">{t('knowledgeEditor.faq.title')}</h3>
                        <p className="section-desc">{t('knowledgeEditor.faq.description')}</p>
                      </div>
                      <div className="section-body">
                        <div className="form-item">
                          <label className="form-label required">{t('knowledgeEditor.faq.indexModeLabel')}</label>
                          <RadioGroup value={editorConfig.faqConfig.indexMode} onChange={(value) => setEditorConfig((current) => ({ ...current, faqConfig: { ...current.faqConfig, indexMode: value as KnowledgeEditorConfig['faqConfig']['indexMode'] } }))}>
                            <Radio.Button value="question_only">{t('knowledgeEditor.faq.modes.questionOnly')}</Radio.Button>
                            <Radio.Button value="question_answer">{t('knowledgeEditor.faq.modes.questionAnswer')}</Radio.Button>
                          </RadioGroup>
                          <p className="form-tip">{t('knowledgeEditor.faq.indexModeDescription')}</p>
                        </div>
                        <div className="form-item">
                          <label className="form-label required">{t('knowledgeEditor.faq.questionIndexModeLabel')}</label>
                          <RadioGroup value={editorConfig.faqConfig.questionIndexMode} onChange={(value) => setEditorConfig((current) => ({ ...current, faqConfig: { ...current.faqConfig, questionIndexMode: value as KnowledgeEditorConfig['faqConfig']['questionIndexMode'] } }))}>
                            <Radio.Button value="combined">{t('knowledgeEditor.faq.modes.combined')}</Radio.Button>
                            <Radio.Button value="separate">{t('knowledgeEditor.faq.modes.separate')}</Radio.Button>
                          </RadioGroup>
                          <p className="form-tip">{t('knowledgeEditor.faq.questionIndexModeDescription')}</p>
                        </div>
                        <div className="faq-guide"><p>{t('knowledgeEditor.faq.entryGuide')}</p></div>
                      </div>
                    </div>
                  )) : null}

                  {/* 解析引擎（留守段） */}
                  {type !== 'faq' && editorSection === 'parser' ? sectionShell('parser', true, (
                    <div className="grid gap-4"><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('kbSettings.parser.title')}</h3><p className="m-0 text-sm leading-[22px] text-muted">{t('kbSettings.parser.description')}</p><label className="grid gap-1">{t('kbSettings.parser.title')}<TSelect value={editorConfig.chunkingConfig.parserEngineRules[0]?.engine ?? ''} disabled={editorOptions.loading} options={[{ value: '', label: t('kbSettings.parser.default') }, ...editorOptions.parserEngines.filter((engine) => engine.Available !== false).map((engine) => ({ value: engine.Name, label: engine.Name }))]} onChange={(value) => setEditorConfig((current) => ({ ...current, chunkingConfig: { ...current.chunkingConfig, parserEngineRules: String(value) ? [{ file_types: ['pdf'], engine: String(value) }] : [] } }))} /></label><p className="m-0 text-xs leading-[18px] text-muted">{editorOptions.error ?? t('kbSettings.parser.goConfig')}</p></div>
                  )) : null}

                  {/* 存储引擎（留守段） */}
                  {type !== 'faq' && editorSection === 'storage' ? sectionShell('storage', true, (() => {
                    const storageLocked = Boolean(editingId) && editingHasFiles;
                    const selectedBackend = editorOptions.storageBackends.find((candidate) => candidate.id === editorConfig.storageBackendId);
                    const backendConfig = (selectedBackend?.config ?? {}) as Record<string, unknown>;
                    const text = (value: unknown) => (typeof value === 'string' ? value : '');
                    const selectedHint = selectedBackend ? (text(backendConfig.endpoint) || text(backendConfig.bucket_name) || text(backendConfig.path_prefix) || t('kbSettings.storage.localStorage')) : '';
                    return <div className="grid gap-4"><div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.sidebar.storage')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('kbSettings.storage.selectDescription')}</p></div><div className="grid gap-1"><label className="text-[15px] font-medium text-ink">{t('kbSettings.storage.instanceLabel')}</label><p className="m-0 text-xs leading-[18px] text-muted">{t('kbSettings.storage.instanceDesc')}</p><TSelect value={editorConfig.storageBackendId} disabled={storageLocked || editorOptions.loading} aria-label={t('kbSettings.storage.instanceLabel')} options={[...editorOptions.storageBackends.map((backend) => ({ value: backend.id, label: `${backend.name} · ${backend.provider.toUpperCase()}${backend.id === editorOptions.defaultStorageBackendId ? ` · ${t('kbSettings.storage.defaultTag')}` : ''}` })), ...(editorOptions.storageBackends.every((backend) => backend.id !== editorConfig.storageBackendId) && editorConfig.storageBackendId ? [{ value: editorConfig.storageBackendId, label: editorConfig.storageBackendId }] : [])]} onChange={(value) => { const backendId = String(value); const backend = editorOptions.storageBackends.find((candidate) => candidate.id === backendId); setEditorConfig((current) => ({ ...current, storageBackendId: backendId, storageProvider: backend?.provider ?? current.storageProvider })); }} />{storageLocked ? <p className="m-0 text-xs leading-[18px]" style={{ color: 'var(--color-warning-text, #b54708)' }} data-storage-migrate-hint="">{t('kbSettings.storage.migrateHint')}</p> : null}{!storageLocked && selectedHint ? <p className="m-0 text-xs leading-[18px] text-muted" data-storage-instance-hint="">{selectedHint}</p> : null}<a data-storage-manage-instances="" href="/platform/settings?section=storage" className="justify-self-start text-[13px]" style={{ color: 'var(--color-brand)' }} onClick={(event) => { event.preventDefault(); setDialogOpen(false); navigate('/platform/settings?section=storage'); }}>{t('kbSettings.storage.manageInstances')}</a></div></div>;
                  })()) : null}

                  {/* 分块设置（留守段：ChunkingSettingsFields 与 knowledge-settings 共享） */}
                  {type !== 'faq' ? sectionShell('chunking', editorSection === 'chunking', (
                    <div className="grid gap-4"><div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.chunking.title')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('knowledgeEditor.chunking.description')}</p></div><ChunkingSettingsFields splitting={editorConfig.chunkingConfig} onPatch={(patch) => setEditorConfig((current) => ({ ...current, chunkingConfig: { ...current.chunkingConfig, ...patch } }))} client={client} t={t} /></div>
                  )) : null}

                  {/* 多模态配置（留守段） */}
                  {type !== 'faq' ? sectionShell('multimodal', editorSection === 'multimodal', (
                    <div className="grid gap-4"><div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.multimodal.title')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('knowledgeEditor.multimodal.description')}</p></div><label className="flex items-center gap-2 text-sm"><TCheckbox checked={editorConfig.multimodalConfig.enabled} onChange={(value) => setEditorConfig((current) => ({ ...current, multimodalConfig: { ...current.multimodalConfig, enabled: Boolean(value) } }))} />{t('knowledgeEditor.advanced.multimodal.label')}</label>{editorConfig.multimodalConfig.enabled ? <><label className="grid gap-1">{t('knowledgeEditor.advanced.multimodal.vllmLabel')}<TInput value={editorConfig.multimodalConfig.vllmModelId} onChange={(value) => setEditorConfig((current) => ({ ...current, multimodalConfig: { ...current.multimodalConfig, vllmModelId: String(value) } }))} placeholder={t('knowledgeEditor.advanced.multimodal.vllmPlaceholder')} /></label><label className="grid gap-1">{t('knowledgeEditor.advanced.multimodal.customInstructionsLabel')}<TTextarea rows={4} maxlength={4000} value={editorConfig.multimodalConfig.customInstructions} onChange={(value) => setEditorConfig((current) => ({ ...current, multimodalConfig: { ...current.multimodalConfig, customInstructions: String(value) } }))} placeholder={t('knowledgeEditor.advanced.multimodal.customInstructionsPlaceholder')} /></label></> : null}</div>
                  )) : null}

                  {/* 音频处理（ASR）设置（留守段） */}
                  {type !== 'faq' ? sectionShell('asr', editorSection === 'asr', (
                    <div className="grid gap-4"><div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.asr.title')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('knowledgeEditor.asr.description')}</p></div><label className="flex items-center gap-2 text-sm"><TCheckbox checked={editorConfig.asrConfig.enabled} onChange={(value) => setEditorConfig((current) => ({ ...current, asrConfig: { ...current.asrConfig, enabled: Boolean(value) } }))} />{t('knowledgeEditor.asr.label')}</label>{editorConfig.asrConfig.enabled ? <label className="grid gap-1">{t('knowledgeEditor.asr.modelLabel')}<TInput value={editorConfig.asrConfig.modelId} onChange={(value) => setEditorConfig((current) => ({ ...current, asrConfig: { ...current.asrConfig, modelId: String(value) } }))} placeholder={t('knowledgeEditor.asr.modelPlaceholder')} /></label> : null}</div>
                  )) : null}

                  {/* 知识图谱（留守段：GraphSettings 与 knowledge-settings 共享） */}
                  {type !== 'faq' && editorSection === 'graph' ? sectionShell('graph', true, (
                    <GraphSettings graphExtract={editorConfig.nodeExtractConfig as GraphExtractConfig} modelId={summaryModelId} client={client} canRunGraphExtract={viewer.isAdmin} onChange={(value) => setEditorConfig((current) => ({ ...current, nodeExtractConfig: value }))} onOpenGraphGuide={() => { window.open(((import.meta as { env?: Record<string, string | undefined> }).env?.VITE_KG_GUIDE_URL) || 'https://github.com/Tencent/WeKnora/blob/main/docs/KnowledgeGraph.md', '_blank', 'noopener'); }} />
                  )) : null}

                  {/* 高级设置（留守段） */}
                  {type !== 'faq' ? sectionShell('advanced', editorSection === 'advanced', (
                    <div className="grid gap-4"><div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.advanced.title')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('knowledgeEditor.advanced.description')}</p></div><label className="flex items-center gap-2 text-sm"><TCheckbox checked={editorConfig.questionGenerationConfig.enabled} onChange={(value) => setEditorConfig((current) => ({ ...current, questionGenerationConfig: { ...current.questionGenerationConfig, enabled: Boolean(value) } }))} />{t('knowledgeEditor.advanced.questionGeneration.label')}</label>{editorConfig.questionGenerationConfig.enabled ? <label className="grid gap-1">{t('knowledgeEditor.advanced.questionGeneration.countLabel')}<TInput value={String(editorConfig.questionGenerationConfig.questionCount)} onChange={(value) => setEditorConfig((current) => ({ ...current, questionGenerationConfig: { ...current.questionGenerationConfig, questionCount: Number(String(value)) } }))} /></label> : null}<label className="flex items-center gap-2 text-sm"><TCheckbox checked={editorConfig.autoTagConfig.enabled} onChange={(value) => setEditorConfig((current) => ({ ...current, autoTagConfig: { ...current.autoTagConfig, enabled: Boolean(value) } }))} />{t('knowledgeEditor.advanced.autoTag.label')}</label><label className="grid gap-1">{t('knowledgeEditor.advanced.tableMetadataInstructions.label')}<TTextarea rows={3} maxlength={4000} value={editorConfig.chunkingConfig.tableMetadataInstructions} placeholder={t('knowledgeEditor.advanced.tableMetadataInstructions.placeholder')} onChange={(value) => setEditorConfig((current) => ({ ...current, chunkingConfig: { ...current.chunkingConfig, tableMetadataInstructions: String(value) } }))} /><span className="kb-editor-desc-count justify-self-end text-xs leading-5 text-[var(--color-text-placeholder)]" aria-live="polite" data-table-metadata-count="">{editorConfig.chunkingConfig.tableMetadataInstructions.length}/4000</span></label></div>
                  )) : null}

                  {/* 数据源管理（仅编辑模式，留守段） */}
                  {editingId && editorSection === 'datasource' ? sectionShell('datasource', true, (
                    <div className="max-h-[34rem] overflow-auto"><DataSourcesPage client={client} knowledgeBaseId={editingId} canManage={viewer.isAdmin} embedded /></div>
                  )) : null}

                  {/* 共享设置（仅编辑模式，留守段） */}
                  {editingId && editorSection === 'share' ? sectionShell('share', true, (
                    <KnowledgeBaseShareDialog client={client} knowledgeBaseId={editingId} knowledgeBaseName={name} open inline onClose={() => undefined} onChanged={() => setReloadToken((value) => value + 1)} />
                  )) : null}

                  {/* 活动记录（仅编辑模式，留守段） */}
                  {editingId && editorSection === 'activity' ? sectionShell('activity', true, (
                    <KnowledgeBaseActivityPanel client={client} knowledgeBaseId={editingId} />
                  )) : null}
                </div>
                {/* 保存按钮（Vue :438-456：settings-footer 常驻底栏） */}
                <div className="settings-footer">
                  <div className="settings-footer-actions">
                    <Button theme="default" variant="outline" onClick={() => setDialogOpen(false)}>{t('common.cancel')}</Button>
                    {/* Vue :451-453 `<t-button @click="handleSubmit" :loading="saving" :disabled="loading">`。 */}
                    <Button theme="primary" data-guide="kb-create-submit" loading={saving} disabled={editorOptions.loading} onClick={() => { void save(); }}>{saveButtonLabel}</Button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>,
        document.body,
      ) : null}

      {/* 右侧：共享知识库详情面板（Vue Teleport + Transition → createPortal） */}
      <SharedKnowledgeBaseDrawer
        open={sharedDetail !== null}
        shared={sharedDetail}
        onClose={() => setSharedDetail(null)}
        onGoToKb={(kbId) => { navigate(knowledgeBaseDetailPath(kbId)); setSharedDetail(null); }}
      />
    </div>
  );
}
