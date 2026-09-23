import { formatMessage, type Locale } from '@weknora/i18n';
// TDesign 同构迁移（T12c）：SystemInfo.vue 列表域逐节点复刻——t-tag
// edition/告警标签、t-alert 错误态（operation 重试）/迁移失败横幅、
// t-loading 加载态、t-link 文档链接；样式平移至 settings.td.css §16。
import { Alert, Button, Link, Loading, Tag } from 'tdesign-react';
import { systemInfoRows, uiBuild } from './surface.ts';

const EXTERNAL_LINK_REL = { rel: 'noopener noreferrer' } as const;

/**
 * Read-only system info display — parity port of
 * frontend/src/views/settings/SystemInfo.vue (template lines 1-195): one
 * setting-row per fact with the shared system.* label + help-text keys, the
 * edition/commit/migration tags and the humanized uptime. The panel owns its
 * section-header and its loading/error branches (loading-inline with
 * t-loading / error-inline t-alert with the retry operation button,
 * SystemInfo.vue:8-21); the shell keeps feeding the payload (T12a
 * userprofile self-error precedent — no wrapper, no shell banner).
 */
export function SystemInfoPanel({ payload, locale, error = null, loading = false, onRetry }: {
  payload: unknown;
  locale: Locale;
  error?: string | null;
  loading?: boolean;
  onRetry?: () => void;
}) {
  const t = (key: string) => formatMessage(locale, key);
  const info = payload !== null && typeof payload === 'object' && !Array.isArray(payload) ? payload as Record<string, unknown> : null;
  const rows = systemInfoRows(payload, { locale });
  // Vue SystemInfo.vue:57-63 — the frontend row carries a warning tag when the
  // built UI version drifts from the reported backend version.
  const version = typeof info?.version === 'string' ? info.version : '';
  const versionMismatch = Boolean(version && version !== 'unknown' && uiBuild.version !== 'unknown' && version !== uiBuild.version);
  const migrationError = typeof info?.db_migration_error === 'string' ? info.db_migration_error : '';
  const commitId = typeof info?.commit_id === 'string' ? info.commit_id : '';
  const dbVersion = typeof info?.db_version === 'string' ? info.db_version : '';
  const troubleshootingDocsURL = 'https://github.com/Tencent/WeKnora/blob/main/docs/migration-troubleshooting.md';
  // Vue SystemInfo.vue:254-278 — prefilled bug-report link (encoded body with
  // the current migration error).
  const reportIssueURL = (() => {
    const params = new URLSearchParams({ template: 'bug_report.yml', title: '[Bug]: Database migration failed at startup', labels: 'bug' });
    if (migrationError) {
      params.set('body', [
        '### Environment',
        `- WeKnora version: ${version || 'unknown'}`,
        `- Commit: ${commitId || 'unknown'}`,
        `- Frontend version: ${uiBuild.version} (${uiBuild.commit})`,
        `- DB version reported: ${dbVersion || 'unknown'}`,
        '',
        '### Migration error',
        '```',
        migrationError,
        '```',
      ].join('\n'));
    }
    return `https://github.com/Tencent/WeKnora/issues/new?${params.toString()}`;
  })();
  return (
    <div className="system-info" data-testid="system-info-panel">
      <div className="section-header">
        <h2>{t('system.title')}</h2>
        <p className="section-description">{t('system.sectionDescription')}</p>
      </div>
      {loading ? (
        <div className="loading-inline">
          <Loading size="small" />
          <span>{t('system.loadingInfo')}</span>
        </div>
      ) : error ? (
        <div className="error-inline">
          <Alert theme="error" message={error} operation={<Button size="small" onClick={() => onRetry?.()}>{t('system.retry')}</Button>} />
        </div>
      ) : (
        <div className="settings-group">
          {rows.map((row, index) => (
            <div className="setting-row" key={row.labelKey}>
              <div className="setting-info">
                <label>{t(row.labelKey)}</label>
                <p className="desc">{t(row.descriptionKey)}</p>
              </div>
              <div className="setting-control">
                <span className="info-value">
                  {/* Vue 模板空格文本节点复刻（台账 #11 同族）：`{{ value }}` 与
                      t-tag / commit span 之间的换行在 Vue 端凝结为**单个**带尾
                      随空格的文本节点（"unknown "），JSX 若用 {value}{' '} 两个
                      表达式会拆成两个文本节点，跨节点 kerning 使整行宽 ±0.03px、
                      右对齐起点亚像素漂移（实测 52px AA 残差）。必须单表达式
                      拼接成单文本节点；commit span 内部同理（" (hash) "）。 */}
                  {row.value + ' '}
                  {/* Vue SystemInfo.vue:34-44 — edition / db-migration tags sit
                      between the value and the commit id, margin-left 8px. */}
                  {row.tag ? <Tag theme={row.tagTone === 'danger' ? 'danger' : row.tagTone === 'warning' ? 'warning' : 'default'} variant="light" size="small" style={{ marginLeft: '8px' }}>{row.tag}</Tag> : null}
                  {index === 1 && versionMismatch ? <Tag theme="warning" variant="light" size="small" style={{ marginLeft: '8px' }}>{t('system.versionMismatch')}</Tag> : null}
                  {row.commit ? <span className="commit-info">{' (' + row.commit + ') '}</span> : null}
                </span>
              </div>
            </div>
          ))}
          {/* DB migration error full-width banner (Vue SystemInfo.vue:136-158). */}
          {migrationError ? (
            <div className="setting-row migration-error-row">
              <Alert theme="error" title={t('system.dbMigrationFailedTitle')} style={{ width: '100%' }} message={(
                /* Vue default slot 子节点直挂（p/pre/div 三节点，无包裹层）。 */
                <>
                  <p className="migration-error-desc">{t('system.dbMigrationFailedDesc')}</p>
                  <pre className="migration-error-detail">{migrationError}</pre>
                  <div className="migration-error-actions">
                    {/* tdesign-react Link 类型未声明 rel 但运行时透传到 <a>（Vue
                        t-link attr fallthrough 同效）；spread 绕开 excess check。 */}
                    <Link theme="primary" href={troubleshootingDocsURL} target="_blank" {...EXTERNAL_LINK_REL}>{t('system.dbMigrationViewDocs')}</Link>
                    <span className="migration-error-actions-sep">·</span>
                    <Link theme="primary" href={reportIssueURL} target="_blank" {...EXTERNAL_LINK_REL}>{t('system.dbMigrationReportIssue')}</Link>
                  </div>
                </>
              )} />
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}
