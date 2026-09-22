import type { ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { Button, Tag } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { createTranslator, useAppLocale } from '../i18n.ts';
// 样式由页面模块统一加载（kb-list.td.css §7 平移段 + kb-editor-parity.css 留守段）。

// R438 A1 — shared knowledge base detail drawer, ported from Vue
// KnowledgeBaseList.vue:709-776 (structure) + :1489-1525 (state & actions).
// Task 11a：DOM 重写为 Vue unscoped 块 1:1（.shared-detail-drawer-overlay
// 经 createPortal 挂 body，复刻 Vue <Teleport to="body">），样式走
// kb-list.td.css §7 平移段（playbook §2.5/§2.6）。

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
export function permissionTheme(permission: unknown): 'primary' | 'warning' | 'default' {
  if (permission === 'admin') return 'primary';
  if (permission === 'editor') return 'warning';
  return 'default';
}

/** 兼容旧名（颜色 tone 语义并入 t-tag theme；default→neutral）。 */
export const permissionTone = (permission: unknown): 'primary' | 'warning' | 'neutral' => {
  const theme = permissionTheme(permission);
  return theme === 'default' ? 'neutral' : theme;
};

/* frontend/src/assets/img/organization-green.svg —— 共享来源空间徽标（20×20）。 */
function OrgGreenIcon({ className }: { className: string }) {
  return (
    <svg className={className} width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
      <path d="M10 10C8.8 7.5 7.8 3.8 4.8 3.8C2.2 3.8 0.8 6.8 0.8 10C0.8 13.2 2.2 16.2 4.8 16.2C7.8 16.2 8.8 12.5 10 10C11.2 7.5 12.5 5.5 14.5 5.5C16.5 5.5 18 7.5 18 10C18 12.5 16.5 14.5 14.5 14.5C12.5 14.5 11.2 12.5 10 10Z" stroke="#07C05F" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
    </svg>
  );
}

function DetailRow({ label, children }: { label: string; children: ReactNode }) {
  // Vue .shared-detail-row (KnowledgeBaseList.vue unscoped :3021-3027)。
  return (
    <div className="shared-detail-row">
      <span className="shared-detail-label">{label}</span>
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
  // Vue Transition：仅打开瞬间挂载（sharedDetailPanelVisible && currentSharedKbForDetail）。
  const visible = open && shared !== null;
  if (!visible) return null;
  return createPortal(
    <div className="shared-detail-drawer-overlay" onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}>
      <div className="shared-detail-drawer" role="dialog" aria-label={t('knowledgeList.detail.title')}>
        <div className="shared-detail-drawer-header">
          <h3 className="shared-detail-drawer-title">{t('knowledgeList.detail.title')}</h3>
          <button type="button" className="shared-detail-drawer-close" aria-label={t('general.close')} onClick={onClose}>
            <TIcon name="close" size="20px" />
          </button>
        </div>
        <div className="shared-detail-drawer-body">
          <DetailRow label={t('knowledgeBase.name')}>
            <span className="shared-detail-value">{name}</span>
          </DetailRow>
          <DetailRow label={t('knowledgeList.detail.sourceType')}>
            <span className="shared-detail-value shared-detail-source-type">
              {agent ? t('knowledgeList.detail.sourceTypeAgent') : t('knowledgeList.detail.sourceTypeKbShare')}
            </span>
          </DetailRow>
          <DetailRow label={agent ? t('knowledgeList.detail.sourceFromAgent') : t('knowledgeList.detail.sourceOrg')}>
            <span className="shared-detail-value shared-detail-org">
              {agent ? null : <OrgGreenIcon className="shared-detail-org-icon" />}
              {agent ? (agent.agent_name ?? '') : orgName}
            </span>
          </DetailRow>
          {agent ? (
            <DetailRow label={t('knowledgeList.detail.agentKbStrategy')}>
              <span className="shared-detail-value">
                {t(agentKbStrategyKey(agent.kb_selection_mode ?? ''))}
              </span>
            </DetailRow>
          ) : null}
          <DetailRow label={t('knowledgeList.detail.sharedAt')}>
            <span className="shared-detail-value">{formatSharedAt(shared?.shared_at)}</span>
          </DetailRow>
          <DetailRow label={t('knowledgeList.detail.myPermission')}>
            <Tag size="small" theme={permissionTheme(permission)}>
              {t(`organization.role.${permission}`)}
            </Tag>
          </DetailRow>
        </div>
        <div className="shared-detail-drawer-footer">
          <Button theme="default" variant="outline" onClick={onClose}>{t('common.close')}</Button>
          <Button theme="primary" className="go-to-kb-btn" disabled={!kbId} onClick={() => { if (kbId) onGoToKb(kbId); }}>
            <TIcon name="browse" />
            {t('knowledgeList.detail.goToKb')}
          </Button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
