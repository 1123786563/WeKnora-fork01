/**
 * Personalization section of the agent editor (Octop M1 port — no Vue
 * baseline). Renders the 16-type MBTI catalog from client.mbti.types(),
 * writes the persona_mbti / persona_style config keys that buildAgentPayload
 * carries to the backend, and shows dimension bars + summary for the chosen
 * type. The "take the test" button opens the MbtiTestModal whose apply
 * action writes the scored code straight into persona_mbti.
 */
import { useEffect, useMemo, useState } from 'react';
import type { MbtiAxis, MbtiProfile, WeKnoraClient } from '@weknora/api-client';
import { Textarea } from '@weknora/ui';
import { usePreferredLocale } from '../locale.ts';
import type { AgentConfigForm, Translate } from './agent-editor.ts';
import { MbtiTestModal } from './MbtiTestModal.tsx';

// Mirrors the modal's private constants (AgentEditorModal.tsx FIELD_BASE /
// FIELD_TEXTAREA / AE_BTN) so the section matches sibling styling without
// widening the modal's exports.
const FIELD_BASE = 'w-full rounded-md border border-[var(--td-component-stroke,#dcdcdc)] px-2.5 py-1.5 font-[family-name:inherit] text-[14px] text-inherit bg-[var(--td-bg-color-container,#fff)]';
const FIELD_WIDE = 'max-w-[460px]';
const FIELD_DISABLED_BG = 'disabled:bg-[var(--td-bg-color-secondarycontainer,#f2f3f5)]';
const FIELD_TEXTAREA = `${FIELD_BASE} ${FIELD_WIDE} resize-y ${FIELD_DISABLED_BG}`;
const AE_BTN = 'cursor-pointer rounded-md px-4 py-1.5 text-[14px]';

// The modal's Row layout with only the pieces this section needs.
function Row({ label, desc, htmlFor, children }: {
  label: string; desc?: string; htmlFor?: string; children: React.ReactNode;
}) {
  return (
    <div className="flex items-start gap-6">
      <div className="w-[260px] shrink-0">
        <label className="text-sm font-medium" htmlFor={htmlFor}>{label}</label>
        {desc ? <p className="m-0 mt-1 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{desc}</p> : null}
      </div>
      <div className="flex min-w-0 flex-1 flex-col items-start gap-1.5">{children}</div>
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
    <div className="flex w-full max-w-[460px] items-center gap-2.5">
      <span className={`w-11 shrink-0 text-[13px] ${leftDominant ? 'font-semibold' : 'font-normal text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]'}`} style={leftDominant ? { color } : undefined}>
        {letters[0]}{leftDominant ? ` ${Math.round(percent)}%` : ''}
      </span>
      <div
        className="relative h-[6px] flex-1 overflow-hidden rounded-[3px] bg-[var(--td-bg-color-secondarycontainer,#f2f3f5)]"
        role="img"
        aria-label={`${letters[0]} / ${letters[1]}: ${dominant} ${Math.round(percent)}%`}
      >
        {/* bar color is the runtime profile color — controlled inline style */}
        <div
          className="absolute top-0 h-full rounded-[3px]"
          style={leftDominant ? { width, left: 0, backgroundColor: color } : { width, right: 0, backgroundColor: color }}
        />
      </div>
      <span className={`w-11 shrink-0 text-right text-[13px] ${leftDominant ? 'font-normal text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]' : 'font-semibold'}`} style={leftDominant ? undefined : { color }}>
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
        className={`flex cursor-pointer flex-col items-start gap-0.5 rounded-lg border px-2.5 py-2 text-left text-[13px] transition-[border] hover:bg-[var(--td-bg-color-container-hover,#f3f3f3)] ${active ? (color ? '' : 'border-[var(--td-brand-color,#0052d9)] ring-2 ring-[var(--td-brand-color,#0052d9)] ring-inset') : 'border-[var(--td-component-stroke,#e7e7e7)]'}`}
        style={active && color ? { borderColor: color, boxShadow: `0 0 0 2px ${color}` } : undefined}
        aria-pressed={active ? 'true' : 'false'}
        data-mbti-code={code}
        onClick={onSelect}
      >
        {code ? <span className="text-[15px] font-semibold" style={active && color ? { color } : undefined}>{code}</span> : null}
        <span className="font-medium">{name}</span>
        {nickname ? <span className="text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{nickname}</span> : null}
      </button>
    );
  };

  return (
    <section className="wk-ae-section" data-editor-section="personalization">
      <header className="[&_h2]:m-0 [&_h2]:mb-1 [&_h2]:text-[16px]">
        <h2>{t('agentEditor.personalization.title')}</h2>
        <p className="m-0 mb-4 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agentEditor.personalization.desc')}</p>
      </header>
      <div className="flex flex-col gap-[18px]">
        <Row label={t('agentEditor.personalization.mbtiLabel')} desc={t('agentEditor.personalization.mbtiDesc')}>
          {types === null && !loadFailed ? (
            <p className="m-0 text-[13px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]" role="status">{t('common.loading')}</p>
          ) : null}
          {loadFailed ? (
            <>
              <p className="m-0 text-[13px] text-[var(--td-error-color,#d54941)]" role="alert">{t('agentEditor.personalization.loadFailed')}</p>
              <button
                type="button"
                className={`${AE_BTN} border border-[var(--td-component-stroke,#dcdcdc)] bg-[var(--td-bg-color-container,#fff)] text-inherit hover:bg-[var(--td-bg-color-container-hover,#f3f3f3)]`}
                onClick={() => setAttempt((current) => current + 1)}
              >{t('common.retry')}</button>
            </>
          ) : null}
          {types !== null ? (
            <div className="grid w-full grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-2">
              {renderCard('', t('agentEditor.personalization.noneLabel'), t('agentEditor.personalization.noneDesc'), undefined, () => patchConfig('persona_mbti', ''))}
              {types.map((row) => renderCard(row.code, zh ? row.name_zh : row.name_en, row.nickname_zh, row.color, () => patchConfig('persona_mbti', row.code)))}
            </div>
          ) : null}
          {selectedCode !== '' && types !== null && !selected ? (
            <p className="m-0 text-[12px] text-[var(--td-warning-color,#e37318)]">{t('agentEditor.personalization.unknownType', { code: selectedCode })}</p>
          ) : null}
          {/* Test modal: apply writes the scored code into persona_mbti and closes */}
          <button
            type="button"
            className={`${AE_BTN} border border-[var(--td-brand-color,#0052d9)] bg-[var(--td-bg-color-container,#fff)] text-[var(--td-brand-color,#0052d9)] hover:bg-[var(--td-bg-color-container-hover,#f3f3f3)] disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:bg-[var(--td-bg-color-container,#fff)]`}
            data-mbti-take-test
            onClick={() => setTestOpen(true)}
          >{t('agentEditor.personalization.takeTest')}</button>
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
              <div className="flex w-full flex-col gap-2" data-mbti-dimensions={selected.code}>
                {MBTI_AXES.map(({ key, letters }) => (
                  <DimensionBar key={key} axis={selected.dimensions[key]} letters={letters} color={selected.color} />
                ))}
              </div>
              <p className="m-0 max-w-[460px] text-[13px] leading-6 text-[var(--td-text-color-primary,rgba(0,0,0,0.9))]" data-mbti-summary>{zh ? selected.summary_zh : selected.summary_en}</p>
            </Row>
            <Row label={t('agentEditor.personalization.styleLabel')} desc={t('agentEditor.personalization.styleDesc')} htmlFor="wk-ae-persona-style">
              <Textarea
                id="wk-ae-persona-style"
                className={`wk-ae-textarea ${FIELD_TEXTAREA}`}
                rows={3}
                value={config.persona_style ?? ''}
                placeholder={t('agentEditor.personalization.stylePlaceholder')}
                onChange={(event) => patchConfig('persona_style', event.target.value)}
              />
            </Row>
          </>
        ) : null}
      </div>
    </section>
  );
}
