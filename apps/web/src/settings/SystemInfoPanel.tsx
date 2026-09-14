import { formatMessage, type Locale } from '@weknora/i18n';
import { systemInfoRows } from './surface.ts';

/**
 * Read-only system info display list — item D parity port of
 * frontend/src/views/settings/SystemInfo.vue (template lines 21-197): one
 * setting-row per fact with the shared system.* label + help-text keys, the
 * edition/commit/migration tags and the humanized uptime. The wrapper heading
 * (系统信息 + section description) is rendered by SettingsPage via
 * settingsSectionHeading; this panel owns the rows.
 */
export function SystemInfoPanel({ payload, locale }: { payload: unknown; locale: Locale }) {
  const rows = systemInfoRows(payload, { locale });
  const t = (key: string) => formatMessage(locale, key);
  return (
    <div className="system-info" data-testid="system-info-panel">
      <div className="settings-group">
        {rows.map((row) => (
          <div className="setting-row" key={row.labelKey}>
            <div className="setting-info">
              <label>{t(row.labelKey)}</label>
              <p className="desc">{t(row.descriptionKey)}</p>
            </div>
            <div className="setting-control">
              <span className="info-value">
                {row.value}
                {row.commit ? <span className="commit-info"> ({row.commit})</span> : null}
                {row.tag ? <span className={'wk-tag wk-tag--' + (row.tagTone ?? 'default') + ' inline-flex items-center shrink-0 py-[1px]! px-[8px]! leading-[1.6]' + (row.tagTone === 'warning' ? ' text-[#b45309]! bg-[#fffaeb]! border border-solid border-[#fedf89]' : '')}>{row.tag}</span> : null}
              </span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
