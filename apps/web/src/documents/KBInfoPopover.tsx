// KBInfoPopover——Vue frontend/src/components/KBInfoPopover.vue 同构平移。
// ⓘ 信息按钮 + t-popup 弹层：基础/访问/能力/分块/统计/存储绑定 六组 section。
// 弹层 portal 到 body（overlayClassName-free：kb-info-card 族规则在各页 td.css
// 内 unscoped 平移，见 faq.td.css / documents.td.css 头注）。
import { useState } from 'react';
import type { ReactNode } from 'react';
import { Popup, Tag, Tooltip } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { createTranslator } from '../i18n.ts';

type Translate = ReturnType<typeof createTranslator>;

/** KB 记录宽松视图：popover 逐字段可选读取（Vue props.kbInfo: any 同语义）。 */
export type KBInfoPopoverKB = Record<string, unknown>;

/** Vue formatStringDate（frontend/src/utils/index.ts:55）：本地时区 YYYY-MM-DD HH:mm:ss。 */
export function formatStringDate(input: string | Date): string {
  const data = new Date(input);
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${data.getFullYear()}-${pad(data.getMonth() + 1)}-${pad(data.getDate())} ${pad(data.getHours())}:${pad(data.getMinutes())}:${pad(data.getSeconds())}`;
}

type RoleTheme = 'success' | 'primary' | 'warning' | 'default';

export interface KBInfoPopoverProps {
  t: Translate;
  /** KB 详情/列表记录（后端同构字段全集）。 */
  kbInfo: KBInfoPopoverKB;
  /** 调用方已知的可上传扩展名（无则不渲染该行；FAQ KB 不传）。 */
  supportedFileTypes?: string[];
  /** Vue authStore.user?.id——与 creator_id 对照判 owner。 */
  userId?: string;
  /**
   * Vue orgStore 侧共享行：当前 KB 在 shared-knowledge-bases 列表中的记录
   * （org_name + shared_at）。null = 非共享渠道。
   */
  sharedKb?: { orgName: string; sharedAt: string } | null;
  /** Vue effectiveKBPermission = getKBPermission(id) || kbInfo.my_permission。 */
  permission?: string;
  /** 附加共享信息行（Vue kbInfo.share_count）。 */
  shareCount?: number;
}

/** Vue VectorStoreBadge.vue 同构（内部仅本组件消费）。 */
function VectorStoreBadge(props: { t: Translate; source?: string; name?: string; engineType?: string; status?: string }) {
  const { t, source, name, engineType, status } = props;
  const effectiveSource = source || 'env';
  const isUnavailable = status === 'unavailable' || effectiveSource === 'unavailable';
  const iconName = effectiveSource === 'env' || effectiveSource === 'user' ? 'data-base'
    : effectiveSource === 'shared' ? 'share' : 'help-circle';
  const displayName = effectiveSource === 'env' ? t('vectorStoreBadge.systemDefault')
    : effectiveSource === 'shared' ? t('vectorStoreBadge.sharedFromOrg')
    : name || t('vectorStoreBadge.unknownStore');
  return (
    <span className={`vs-badge vs-badge-${effectiveSource}${isUnavailable ? ' vs-badge-warn' : ''}`}>
      <TIcon name={iconName} className="vs-badge-icon" />
      <span className="vs-badge-name">{displayName}</span>
      {engineType && (effectiveSource === 'user' || effectiveSource === 'env') ? (
        <span className="vs-badge-engine">{' ('}{engineType}{')'}</span>
      ) : null}
      {isUnavailable ? (
        <Tag theme="danger" variant="light" size="small" className="vs-badge-warn-tag">{t('vectorStoreBadge.unavailable')}</Tag>
      ) : null}
    </span>
  );
}

export function KBInfoPopover(props: KBInfoPopoverProps) {
  const { t, kbInfo, supportedFileTypes, userId, sharedKb, permission, shareCount } = props;
  const [open, setOpen] = useState(false);

  const str = (value: unknown): string => (typeof value === 'string' ? value : '');
  const kbType = str(kbInfo.type);

  // Vue isOwner：creator_id === 当前用户（legacy 空 creator_id 落 role/share 检查）。
  const isOwner = (() => {
    const creatorId = str(kbInfo.creator_id);
    return creatorId !== '' && creatorId === (userId ?? '');
  })();
  const isViaShare = !!sharedKb;
  const effectivePermission = permission || str(kbInfo.my_permission);

  const accessRoleLabel = !isViaShare && isOwner ? t('knowledgeBase.accessInfo.roleOwner')
    : effectivePermission ? t(`organization.role.${effectivePermission}`) : '--';
  const accessPermissionSummary = !isViaShare && isOwner ? t('knowledgeBase.accessInfo.permissionOwner')
    : effectivePermission === 'admin' ? t('knowledgeBase.accessInfo.permissionAdmin')
    : effectivePermission === 'editor' ? t('knowledgeBase.accessInfo.permissionEditor')
    : effectivePermission === 'viewer' ? t('knowledgeBase.accessInfo.permissionViewer')
    : '--';
  const roleTagTheme: RoleTheme = !isViaShare && isOwner ? 'success'
    : effectivePermission === 'admin' ? 'primary'
    : effectivePermission === 'editor' ? 'warning' : 'default';

  // hasDistinctUpdate：updated_at 存在且 ≠ created_at（GORM 只在 KB 行被改时 bump）。
  const hasDistinctUpdate = (() => {
    const created = str(kbInfo.created_at);
    const updated = str(kbInfo.updated_at);
    if (!updated) return false;
    if (!created) return true;
    return new Date(updated).getTime() !== new Date(created).getTime();
  })();

  const supportedFileTypesSorted = supportedFileTypes?.length ? [...supportedFileTypes].sort() : [];

  type CapabilityTheme = 'primary' | 'success' | 'warning' | 'default';
  const capabilities: Array<{ key: string; label: string; theme: CapabilityTheme }> = [];
  if ((kbInfo.vlm_config as { enabled?: boolean } | null)?.enabled) capabilities.push({ key: 'vlm', label: 'VLM', theme: 'primary' });
  if ((kbInfo.asr_config as { enabled?: boolean } | null)?.enabled) capabilities.push({ key: 'asr', label: 'ASR', theme: 'primary' });
  if ((kbInfo.extract_config as { enabled?: boolean } | null)?.enabled) capabilities.push({ key: 'kg', label: t('knowledgeList.features.knowledgeGraph'), theme: 'success' });
  if ((kbInfo.indexing_strategy as { wiki_enabled?: boolean } | null)?.wiki_enabled) capabilities.push({ key: 'wiki', label: 'Wiki', theme: 'warning' });

  // chunkingStrategyLabel：''/recursive → legacy 键；无翻译回落原文。
  const chunkingStrategyLabel = (() => {
    const raw = str((kbInfo.chunking_config as { strategy?: unknown } | null)?.strategy).toLowerCase();
    const key = raw === '' || raw === 'recursive' ? 'legacy' : raw;
    const path = `knowledgeEditor.chunking.strategies.${key}.label`;
    const translated = t(path);
    return translated === path ? raw : translated;
  })();
  const chunkingRows: Array<{ key: string; label: string; value: string }> = [];
  if (kbType !== 'faq') {
    const cfg = kbInfo.chunking_config as {
      chunk_size?: number; chunk_overlap?: number; enable_parent_child?: boolean;
      parent_chunk_size?: number; child_chunk_size?: number; token_limit?: number;
    } | null;
    if (cfg) {
      if (chunkingStrategyLabel) chunkingRows.push({ key: 'strategy', label: t('knowledgeEditor.chunking.strategyLabel'), value: chunkingStrategyLabel });
      const chars = t('knowledgeEditor.chunking.characters');
      if (typeof cfg.chunk_size === 'number' && cfg.chunk_size > 0) chunkingRows.push({ key: 'size', label: t('knowledgeEditor.chunking.sizeLabel'), value: `${cfg.chunk_size} ${chars}` });
      if (typeof cfg.chunk_overlap === 'number') chunkingRows.push({ key: 'overlap', label: t('knowledgeEditor.chunking.overlapLabel'), value: `${cfg.chunk_overlap} ${chars}` });
      if (cfg.enable_parent_child) {
        const parent = cfg.parent_chunk_size || 4096;
        const child = cfg.child_chunk_size || 384;
        chunkingRows.push({ key: 'parent-child', label: t('knowledgeEditor.chunking.parentChildLabel'), value: `${t('knowledgeBase.infoCard.parentShort')} ${parent} / ${t('knowledgeBase.infoCard.childShort')} ${child}` });
      }
      if (typeof cfg.token_limit === 'number' && cfg.token_limit > 0) chunkingRows.push({ key: 'token-limit', label: t('knowledgeEditor.chunking.tokenLimitLabel'), value: String(cfg.token_limit) });
    }
  }

  const statRows: Array<{ key: string; label: string; value: string }> = [];
  if (kbType === 'faq') {
    if (typeof kbInfo.chunk_count === 'number') statRows.push({ key: 'faq', label: t('knowledgeBase.infoCard.faqCount'), value: String(kbInfo.chunk_count) });
  } else if (typeof kbInfo.knowledge_count === 'number') {
    statRows.push({ key: 'knowledge', label: t('knowledgeBase.infoCard.documentCount'), value: String(kbInfo.knowledge_count) });
  }

  const storageProvider = str((kbInfo.storage_provider_config as { provider?: unknown } | null)?.provider);
  const vectorStoreSource = str(kbInfo.vector_store_source);

  const section = (title: string, children: ReactNode): ReactNode => (
    <section className="setting-drawer__section">
      <h4 className="setting-drawer__section-title">{title}</h4>
      {children}
    </section>
  );
  const row = (label: string, value: ReactNode, valueClass = ''): ReactNode => (
    <div className="kb-info-card-row">
      <span className="kb-info-card-label">{label}</span>
      <span className={'kb-info-card-value' + (valueClass ? ` ${valueClass}` : '')}>{value}</span>
    </div>
  );

  return (
    /* Vue t-tooltip > t-popup > button（tooltip 在外层）。tdesign-react 的
       Tooltip/Popup 都以 cloneElement 向 children 注入事件，两层组件互相
       穿不透——用一个 inline-flex 的 span 锚点隔开（布局零影响）：
       Tooltip 挂 span，Popup 挂 button，两弹层仍 portal 到 body，DOM 结果
       与 Vue 等价；trigger 加 focus 复刻 Vue 点击聚焦后 tooltip 保持显示。 */
    <Tooltip content={t('knowledgeBase.infoCard.tooltip')} placement="top">
      <span className="kb-info-popover-anchor">
        <Popup
          visible={open}
          trigger="click"
          placement="bottom-right"
          overlayStyle={{ padding: 0 }}
          overlayInnerStyle={{ padding: 0 }}
          onVisibleChange={setOpen}
          content={(
          <div className="kb-info-card">
            <div className="kb-info-card-header">{t('knowledgeBase.infoCard.title')}</div>
            <div className="kb-info-card-body">
              {section(t('knowledgeBase.infoCard.basic'), <>
                {row(t('knowledgeBase.infoCard.type'),
                  kbType.toLowerCase() === 'faq' ? t('knowledgeEditor.basic.typeFAQ') : t('knowledgeEditor.basic.typeDocument'))}
                {str(kbInfo.description) ? row(t('knowledgeBase.description'),
                  str(kbInfo.description), 'kb-info-card-value-block') : null}
                {str(kbInfo.created_at) ? row(t('knowledgeBase.infoCard.createdAt'),
                  formatStringDate(str(kbInfo.created_at))) : null}
                {hasDistinctUpdate ? row(t('knowledgeBase.accessInfo.lastUpdated'),
                  formatStringDate(str(kbInfo.updated_at))) : null}
                {supportedFileTypesSorted.length > 0 ? row(t('knowledgeBase.infoCard.supportedFileTypes'),
                  supportedFileTypesSorted.map((fileType) => <span key={fileType} className="kb-info-card-ext">{`.${fileType}`}</span>)) : null}
              </>)}
              {section(t('knowledgeBase.infoCard.access'), <>
                {row(t('knowledgeBase.accessInfo.myRole'), <>
                  <Tag size="small" theme={roleTagTheme}>{accessRoleLabel}</Tag>
                  <span className="kb-info-card-hint">{accessPermissionSummary}</span>
                </>)}
                {sharedKb ? row(t('knowledgeBase.accessInfo.fromOrg'),
                  `「${sharedKb.orgName}」 · ${t('knowledgeBase.accessInfo.sharedAt')} ${formatStringDate(sharedKb.sharedAt)}`) : null}
                {!sharedKb && effectivePermission ? row(t('knowledgeBase.infoCard.source'),
                  t('knowledgeList.detail.sourceTypeAgent')) : null}
                {(typeof kbInfo.share_count === 'number' ? kbInfo.share_count : shareCount ?? 0) > 0 ? row(t('knowledgeBase.infoCard.sharedTo'),
                  t('knowledgeList.sharedToOrgs', { count: (kbInfo.share_count as number) ?? shareCount ?? 0 })) : null}
              </>)}
              {capabilities.length > 0 ? section(t('knowledgeBase.infoCard.capabilities'),
                row(t('knowledgeBase.infoCard.enabled'),
                  capabilities.map((cap) => <Tag key={cap.key} size="small" variant="light" theme={cap.theme}>{cap.label}</Tag>))) : null}
              {chunkingRows.length > 0 ? section(t('knowledgeBase.infoCard.chunking'), <>
                {chunkingRows.map((entry) => row(entry.label, entry.value))}
              </>) : null}
              {statRows.length > 0 ? section(t('knowledgeBase.infoCard.stats'), <>
                {statRows.map((entry) => row(entry.label, <span className="kb-info-card-value-number">{entry.value}</span>))}
              </>) : null}
              {vectorStoreSource || storageProvider ? section(t('knowledgeBase.infoCard.binding'), <>
                {vectorStoreSource ? row(t('knowledgeBase.infoCard.vectorStore'),
                  <VectorStoreBadge t={t} source={vectorStoreSource} name={str(kbInfo.vector_store_name)} engineType={str(kbInfo.vector_store_engine_type)} status={str(kbInfo.vector_store_status)} />) : null}
                {storageProvider ? row(t('knowledgeBase.infoCard.fileStorage'),
                  <span className="kb-info-card-value-mono">{storageProvider}</span>) : null}
              </>) : null}
            </div>
          </div>
        )}
        >
          <button
            type="button"
            className={'kb-info-button' + (str(kbInfo.vector_store_status) === 'unavailable' ? ' has-warning' : '')}
          >
            <TIcon name="info-circle" size="16px" />
          </button>
        </Popup>
      </span>
    </Tooltip>
  );
}
