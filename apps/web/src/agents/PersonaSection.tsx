/**
 * Personalization section of the agent editor (Octop M1 port — no Vue
 * baseline). Renders the 16-type MBTI catalog from client.mbti.types(),
 * writes the persona_mbti / persona_style config keys that buildAgentPayload
 * carries to the backend, and shows dimension bars + summary for the chosen
 * type. The "take the test" button opens the MbtiTestModal whose apply
 * action writes the scored code straight into persona_mbti.
 *
 * TDesign 同构迁移（Task 9）：控件换 tdesign-react（Textarea/Button），布局
 * 值从 Tailwind utility 平移到 agents.td.css（wk-ae-persona-* 段）。
 */
import { useEffect, useMemo, useState } from 'react';
import type { MbtiAxis, MbtiProfile, WeKnoraClient } from '@weknora/api-client';
import { Button, Textarea } from 'tdesign-react';
import { usePreferredLocale } from '../locale.ts';
import type { AgentConfigForm, Translate } from './agent-editor.ts';
import { MbtiTestModal } from './MbtiTestModal.tsx';

// 编辑器 Vue setting-row 结构的局部复刻（无 Vue 事实源，随 agents.td.css 平移段走）。
function Row({ label, desc, htmlFor, children }: {
  label: string; desc?: string; htmlFor?: string; children: React.ReactNode;
}) {
  return (
    <div className="setting-row setting-row-vertical">
      <div className="setting-info">
        <label htmlFor={htmlFor}>{label}</label>
        {desc ? <p className="desc">{desc}</p> : null}
      </div>
      <div className="setting-control setting-control-full">{children}</div>
    </div>
  );
}

// ei/sn/tf/jp axes with their two pole letters, in display order. Exported for
// MbtiTestModal, which scores the same axes from MbtiScore.dimensions.
export const MBTI_AXES: ReadonlyArray<{ key: 'ei' | 'sn' | 'tf' | 'jp'; letters: readonly [string, string] }> = [
  { key: 'ei', letters: ['E', 'I'] },
  { key: 'sn', letters: ['S', 'N'] },
  { key: 'tf', letters: ['T', 'F'] },
  { key: 'jp', letters: ['J', 'P'] },
];

// Exported for MbtiTestModal's result stage so both surfaces render the
// dominant-pole percent semantics identically (Task 8 regression contract).
export function DimensionBar({ axis, letters, color }: { axis: MbtiAxis; letters: readonly [string, string]; color: string }) {
  const dominant = axis.pole.toUpperCase();
  const leftDominant = dominant === letters[0];
  const percent = Math.min(100, Math.max(0, axis.percent));
  const width = `${percent}%`;
  return (
    <div className="wk-ae-persona-bar">
      <span
        className={`wk-ae-persona-pole${leftDominant ? ' wk-ae-persona-pole--strong' : ''}`}
        style={leftDominant ? { color } : undefined}
      >
        {letters[0]}{leftDominant ? ` ${Math.round(percent)}%` : ''}
      </span>
      <div
        className="wk-ae-persona-track"
        role="img"
        aria-label={`${letters[0]} / ${letters[1]}: ${dominant} ${Math.round(percent)}%`}
      >
        {/* bar color is the runtime profile color — controlled inline style */}
        <div
          className="wk-ae-persona-bar-fill"
          style={leftDominant ? { width, left: 0, backgroundColor: color } : { width, right: 0, backgroundColor: color }}
        />
      </div>
      <span
        className={`wk-ae-persona-pole wk-ae-persona-pole--right${leftDominant ? '' : ' wk-ae-persona-pole--strong'}`}
        style={leftDominant ? undefined : { color }}
      >
        {/* percent is the DOMINANT pole's strength — the dominant side shows it verbatim */}
        {letters[1]}{leftDominant ? '' : ` ${Math.round(percent)}%`}
      </span>
    </div>
  );
}

export interface PersonaSectionProps {
  config: AgentConfigForm;
  patchConfig: <K extends keyof AgentConfigForm>(key: K, value: AgentConfigForm[K]) => void;
  client: WeKnoraClient;
  t: Translate;
}

export function PersonaSection({ config, patchConfig, client, t }: PersonaSectionProps) {
  const locale = usePreferredLocale();
  const zh = locale === 'zh-CN';
  const [types, setTypes] = useState<MbtiProfile[] | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [testOpen, setTestOpen] = useState(false);

  // Catalog load, once per mount; stale responses are dropped via `active`.
  useEffect(() => {
    let active = true;
    setLoadFailed(false);
    void client.mbti.types().then((rows) => {
      if (!active) return;
      setTypes(rows);
    }).catch(() => {
      if (!active) return;
      setLoadFailed(true);
    });
    return () => { active = false; };
  }, [client, attempt]);

  const selectedCode = config.persona_mbti ?? '';
  const selected = useMemo(
    () => (types ?? []).find((row) => row.code === selectedCode) ?? null,
    [types, selectedCode],
  );

  const renderCard = (code: string, name: string, nickname: string | undefined, color: string | undefined, onSelect: () => void) => {
    const active = selectedCode === code;
    return (
      <button
        key={code}
        type="button"
        className={`wk-ae-persona-card${active ? ' wk-ae-persona-card--active' : ''}`}
        style={active && color ? { borderColor: color, boxShadow: `0 0 0 2px ${color}` } : undefined}
        aria-pressed={active ? 'true' : 'false'}
        data-mbti-code={code}
        onClick={onSelect}
      >
        {code ? <span className="wk-ae-persona-code" style={active && color ? { color } : undefined}>{code}</span> : null}
        <span className="wk-ae-persona-name">{name}</span>
        {nickname ? <span className="wk-ae-persona-nickname">{nickname}</span> : null}
      </button>
    );
  };

  return (
    <div className="section" data-editor-section="personalization">
      <div className="section-header">
        <div className="section-header-title"><h2>{t('agentEditor.personalization.title')}</h2></div>
        <p className="section-description">{t('agentEditor.personalization.desc')}</p>
      </div>
      <div className="settings-group">
        <Row label={t('agentEditor.personalization.mbtiLabel')} desc={t('agentEditor.personalization.mbtiDesc')}>
          {types === null && !loadFailed ? (
            <p className="desc" role="status">{t('common.loading')}</p>
          ) : null}
          {loadFailed ? (
            <>
              <p className="desc" style={{ color: 'var(--td-error-color)' }} role="alert">{t('agentEditor.personalization.loadFailed')}</p>
              <Button variant="outline" onClick={() => setAttempt((current) => current + 1)}>{t('common.retry')}</Button>
            </>
          ) : null}
          {types !== null ? (
            <div className="wk-ae-persona-grid">
              {renderCard('', t('agentEditor.personalization.noneLabel'), t('agentEditor.personalization.noneDesc'), undefined, () => patchConfig('persona_mbti', ''))}
              {types.map((row) => renderCard(row.code, zh ? row.name_zh : row.name_en, row.nickname_zh, row.color, () => patchConfig('persona_mbti', row.code)))}
            </div>
          ) : null}
          {selectedCode !== '' && types !== null && !selected ? (
            <p className="desc" style={{ color: 'var(--td-warning-color)' }}>{t('agentEditor.personalization.unknownType', { code: selectedCode })}</p>
          ) : null}
          {/* Test modal: apply writes the scored code into persona_mbti and closes */}
          <Button variant="outline" className="wk-ae-persona-test-btn" data-mbti-take-test onClick={() => setTestOpen(true)}>
            {t('agentEditor.personalization.takeTest')}
          </Button>
          <MbtiTestModal
            open={testOpen}
            onClose={() => setTestOpen(false)}
            onApply={(code) => {
              patchConfig('persona_mbti', code);
              setTestOpen(false);
            }}
            client={client}
            t={t}
          />
        </Row>
        {selected ? (
          <>
            <Row label={t('agentEditor.personalization.dimensionsLabel')}>
              <div className="wk-ae-persona-dimensions" data-mbti-dimensions={selected.code}>
                {MBTI_AXES.map(({ key, letters }) => (
                  <DimensionBar key={key} axis={selected.dimensions[key]} letters={letters} color={selected.color} />
                ))}
              </div>
              <p className="wk-ae-persona-summary" data-mbti-summary>{zh ? selected.summary_zh : selected.summary_en}</p>
            </Row>
            <Row label={t('agentEditor.personalization.styleLabel')} desc={t('agentEditor.personalization.styleDesc')} htmlFor="wk-ae-persona-style">
              <Textarea
                id="wk-ae-persona-style"
                data-field="persona_style"
                className="wk-ae-persona-style"
                value={config.persona_style ?? ''}
                placeholder={t('agentEditor.personalization.stylePlaceholder')}
                autosize={{ minRows: 3, maxRows: 8 }}
                onChange={(value) => patchConfig('persona_style', String(value))}
              />
            </Row>
          </>
        ) : null}
      </div>
    </div>
  );
}
