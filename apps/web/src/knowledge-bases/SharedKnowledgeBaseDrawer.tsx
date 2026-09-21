import type { ReactNode } from 'react';
import { Badge, Button, Sheet } from '@weknora/ui';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { KbIcon } from './kb-list-icons.tsx';
import './shared-kb-drawer.css';
// KB 编辑器弹窗（App.tsx 新建/编辑知识库）的 Vue 几何对齐规则；此模块被
// App.tsx 静态引用，CSS 随之在知识库列表页就绪（弹窗宿主在 App.tsx，
// 规则落在允许维护的 knowledge-bases/ 目录内）。
import './kb-editor-parity.css';

// R438 A1 — shared knowledge base detail drawer, ported from Vue
// KnowledgeBaseList.vue:709-776 (structure) + :1489-1525 (state & actions).
// The drawer opens from the info-circle trigger on non-own shared KB cards
// (KnowledgeBaseList.vue:305-315) and shows name / source type / source org
// or agent (+ agent KB strategy) / shared at / my permission, with
// 关闭 + 进入知识库 footer actions (goToSharedKbFromPanel, :1521-1525).

/** Vue SourceFromAgentInfo (frontend/src/api/organization/index.ts:106-111). */
export interface SharedKbAgentInfo {
  agent_id?: string;
  agent_name?: string;
  kb_selection_mode?: string;
}

/** Raw row of GET /shared-knowledge-bases as consumed by the drawer. */
export interface SharedKnowledgeBaseDetail {
  knowledge_base?: { id?: unknown; name?: unknown; [key: string]: unknown } | null;
  permission?: unknown;
  shared_at?: unknown;
  org_name?: unknown;
  source_from_agent?: SharedKbAgentInfo;
  [key: string]: unknown;
}

/** Vue agentKbStrategyText (KnowledgeBaseList.vue:1514-1518). */
export function agentKbStrategyKey(mode: string): string {
  if (mode === 'all') return 'knowledgeList.detail.agentKbStrategyAll';
  if (mode === 'selected') return 'knowledgeList.detail.agentKbStrategySelected';
  return 'knowledgeList.detail.agentKbStrategyNone';
}

/** Vue formatStringDate (frontend/src/utils/index.ts:55): local YYYY-MM-DD HH:mm:ss. */
export function formatSharedAt(value: unknown): string {
  if (typeof value !== 'string' && !(value instanceof Date)) return String(value ?? '');
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

/** Vue t-tag theme (KnowledgeBaseList.vue:759-761): admin→primary, editor→warning, else default. */
export function permissionTone(permission: unknown): 'primary' | 'warning' | 'neutral' {
  if (permission === 'admin') return 'primary';
  if (permission === 'editor') return 'warning';
  return 'neutral';
}

const PERMISSION_TAG_CLASS: Record<'primary' | 'warning' | 'neutral', string> = {
  primary: 'kb-shared-detail-permission h-[22px] rounded-[3px] border-0 bg-[rgba(7,192,95,0.08)] px-[7px] py-0 text-[#07c05f]',
  warning: 'kb-shared-detail-permission h-[22px] rounded-[3px] border-0 bg-[rgba(237,123,47,0.08)] px-[7px] py-0 text-[#ed7b2f]',
  neutral: 'kb-shared-detail-permission h-[22px] rounded-[3px] border-0 bg-[#f3f3f3] px-[7px] py-0 text-[rgba(0,0,0,0.66)]',
};

function DetailRow({ label, children }: { label: string; children: ReactNode }) {
  // Vue .shared-detail-row (KnowledgeBaseList.vue:3021-3046): column layout,
  // 12px secondary label over a 14px primary value.
  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-xs leading-[1.4] text-[#646e74]">{label}</span>
      {children}
    </div>
  );
}

export interface SharedKnowledgeBaseDrawerProps {
  open: boolean;
  shared: SharedKnowledgeBaseDetail | null;
  onClose: () => void;
  /** Vue goToSharedKbFromPanel: router.push to the KB detail route, then close. */
  onGoToKb: (kbId: string) => void;
}

export function SharedKnowledgeBaseDrawer({ open, shared, onClose, onGoToKb }: SharedKnowledgeBaseDrawerProps) {
  const locale = useAppLocale();
  const t = createTranslator(locale);
  const kb = shared?.knowledge_base ?? null;
  const kbId = typeof kb?.id === 'string' ? kb.id : typeof shared?.id === 'string' ? shared.id : '';
  const name = typeof kb?.name === 'string' && kb.name
    ? kb.name
    : typeof shared?.name === 'string' ? shared.name : '';
  const agent = shared?.source_from_agent;
  const orgName = typeof shared?.org_name === 'string' ? shared.org_name : '';
  const permission = typeof shared?.permission === 'string' && shared.permission ? shared.permission : 'viewer';

  return (
    <Sheet
      open={open && shared !== null}
      onClose={onClose}
      title={t('knowledgeList.detail.title')}
      width="360px"
      // Vue header close (KnowledgeBaseList.vue:713-716): × icon button with
      // aria-label $t('general.close') = 「关闭设置」; the footer text button
      // below stays on common.close, exactly like the Vue template.
      closeLabel={t('general.close')}
      className="kb-shared-detail-drawer max-w-[90vw]"
    >
      <div className="kb-shared-detail-rows flex flex-col gap-5">
        <DetailRow label={t('knowledgeBase.name')}>
          <span className="break-words text-sm leading-[1.5] text-[#1d2129]">{name}</span>
        </DetailRow>
        <DetailRow label={t('knowledgeList.detail.sourceType')}>
          <span className="break-words text-sm font-medium leading-[1.5] text-[#1d2129]">
            {agent ? t('knowledgeList.detail.sourceTypeAgent') : t('knowledgeList.detail.sourceTypeKbShare')}
          </span>
        </DetailRow>
        <DetailRow label={agent ? t('knowledgeList.detail.sourceFromAgent') : t('knowledgeList.detail.sourceOrg')}>
          <span className="inline-flex items-center gap-1.5 text-sm leading-[1.5] text-[#1d2129] [&_svg]:shrink-0 [&_svg]:text-[#07c05f]">
            {/* Vue organization-green.svg (KnowledgeBaseList.vue:738-744) */}
            {!agent ? <KbIcon name="workspace" size={14} /> : null}
            {agent ? (agent.agent_name ?? '') : orgName}
          </span>
        </DetailRow>
        {agent ? (
          <DetailRow label={t('knowledgeList.detail.agentKbStrategy')}>
            <span className="break-words text-sm leading-[1.5] text-[#1d2129]">
              {t(agentKbStrategyKey(agent?.kb_selection_mode ?? ''))}
            </span>
          </DetailRow>
        ) : null}
        <DetailRow label={t('knowledgeList.detail.sharedAt')}>
          <span className="break-words text-sm leading-[1.5] text-[#1d2129]">{formatSharedAt(shared?.shared_at)}</span>
        </DetailRow>
        <DetailRow label={t('knowledgeList.detail.myPermission')}>
          <Badge className={PERMISSION_TAG_CLASS[permissionTone(permission)]}>
            {t(`organization.role.${permission}`)}
          </Badge>
        </DetailRow>
      </div>
      <footer className="flex shrink-0 justify-end gap-3 border-t border-[#e3e7ee] bg-white px-6 py-4">
        <Button type="button" onClick={onClose}>{t('common.close')}</Button>
        <Button
          type="button"
          variant="primary"
          disabled={!kbId}
          onClick={() => { if (kbId) onGoToKb(kbId); }}
        >
          <KbIcon name="browse" size={16} />
          {t('knowledgeList.detail.goToKb')}
        </Button>
      </footer>
    </Sheet>
  );
}
