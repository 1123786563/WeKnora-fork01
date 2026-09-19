/**
 * 28-question MBTI test modal (Octop M1 port — no Vue baseline). Opened from
 * PersonaSection's "take the test" button: intro (with the for-entertainment
 * disclaimer) → questions (zh/en by app locale, A/B choices, progress counter,
 * all answers required before submit) → result (code + profile card + the
 * Task 8 DimensionBar semantics) whose "apply" button hands the code back to
 * the agent editor. The UI gates submit on ALL questions being answered
 * (stricter than the backend's >=20 minimum, per plan) so the empty-code
 * result path is unreachable.
 */
import { useEffect, useState } from 'react';
import type { MbtiQuestion, MbtiScore, WeKnoraClient } from '@weknora/api-client';
import { Dialog } from '@weknora/ui';
import { usePreferredLocale } from '../locale.ts';
import type { Translate } from './agent-editor.ts';
import { DimensionBar, MBTI_AXES } from './PersonaSection.tsx';

// Mirrors PersonaSection's private constants (which in turn mirror
// AgentEditorModal.tsx FIELD_BASE / AE_BTN) so the dialog controls match the
// editor styling without widening those modules' exports.
const AE_BTN = 'cursor-pointer rounded-md px-4 py-1.5 text-[14px] disabled:cursor-not-allowed disabled:opacity-60';

export interface MbtiTestModalProps {
  open: boolean;
  onClose: () => void;
  onApply: (code: string) => void;
  client: WeKnoraClient;
  t: Translate;
}

type TestStage = 'intro' | 'questions' | 'result';

export function MbtiTestModal({ open, onClose, onApply, client, t }: MbtiTestModalProps) {
  const locale = usePreferredLocale();
  const zh = locale === 'zh-CN';
  const [stage, setStage] = useState<TestStage>('intro');
  const [questions, setQuestions] = useState<MbtiQuestion[] | null>(null);
  const [questionsFailed, setQuestionsFailed] = useState(false);
  const [questionAttempt, setQuestionAttempt] = useState(0);
  const [answers, setAnswers] = useState<Record<string, 'A' | 'B'>>({});
  const [score, setScore] = useState<MbtiScore | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [submitFailed, setSubmitFailed] = useState(false);

  // Reopening always starts a fresh session (PersonaSection keeps this
  // component mounted while the editor is open).
  useEffect(() => {
    if (open) return;
    setStage('intro');
    setQuestions(null);
    setQuestionsFailed(false);
    setQuestionAttempt(0);
    setAnswers({});
    setScore(null);
    setSubmitting(false);
    setSubmitFailed(false);
  }, [open]);

  // Question load, once per attempt while the questions stage is active;
  // stale responses are dropped via `active` (PersonaSection pattern).
  useEffect(() => {
    if (!open || stage !== 'questions' || questions !== null) return;
    let active = true;
    setQuestionsFailed(false);
    void client.mbti.questions().then((rows) => {
      if (!active) return;
      setQuestions(rows);
    }).catch(() => {
      if (!active) return;
      setQuestionsFailed(true);
    });
    return () => { active = false; };
  }, [open, stage, questions, client, questionAttempt]);

  const total = questions?.length ?? 0;
  const answeredCount = Object.keys(answers).length;
  const allAnswered = questions !== null && answeredCount >= total;

  const choose = (id: number, choice: 'A' | 'B') => {
    setAnswers((current) => ({ ...current, [String(id)]: choice }));
  };

  const handleSubmit = async () => {
    if (!allAnswered || submitting) return;
    setSubmitting(true);
    setSubmitFailed(false);
    try {
      const result = await client.mbti.submit(answers);
      setScore(result);
      setStage('result');
    } catch {
      setSubmitFailed(true);
    } finally {
      setSubmitting(false);
    }
  };

  const renderIntro = () => (
    <div className="flex flex-col gap-3" data-mbti-stage="intro">
      <p className="m-0 text-[13px] leading-6 text-[var(--td-text-color-primary,rgba(0,0,0,0.9))]">{t('agentEditor.personalization.testIntroDesc')}</p>
      <p className="m-0 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]" data-mbti-disclaimer>{t('agentEditor.personalization.testDisclaimer')}</p>
      <div className="mt-1 flex justify-end gap-2">
        <button
          type="button"
          className={`${AE_BTN} border bg-[var(--td-brand-color,#0052d9)] border-[var(--td-brand-color,#0052d9)] text-white hover:bg-[var(--td-brand-color-hover,#266fe8)]`}
          data-mbti-start
          onClick={() => setStage('questions')}
        >{t('agentEditor.personalization.testStart')}</button>
      </div>
    </div>
  );

  const renderQuestions = () => (
    <div className="flex flex-col gap-4" data-mbti-stage="questions">
      {questions === null && !questionsFailed ? (
        <p className="m-0 text-[13px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]" role="status">{t('common.loading')}</p>
      ) : null}
      {questionsFailed ? (
        <>
          <p className="m-0 text-[13px] text-[var(--td-error-color,#d54941)]" role="alert">{t('agentEditor.personalization.testLoadFailed')}</p>
          <button
            type="button"
            className={`${AE_BTN} self-start border border-[var(--td-component-stroke,#dcdcdc)] bg-[var(--td-bg-color-container,#fff)] text-inherit hover:bg-[var(--td-bg-color-container-hover,#f3f3f3)]`}
            onClick={() => setQuestionAttempt((current) => current + 1)}
          >{t('common.retry')}</button>
        </>
      ) : null}
      {questions !== null ? (
        <>
          <p className="m-0 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]" data-mbti-progress>
            {t('agentEditor.personalization.testProgress', { answered: answeredCount, total })}
          </p>
          <ol className="m-0 flex list-decimal flex-col gap-4 pl-5">
            {questions.map((question) => {
              const picked = answers[String(question.id)];
              return (
                <li key={question.id} className="text-[13px] leading-6 text-[var(--td-text-color-primary,rgba(0,0,0,0.9))]" data-mbti-q={question.id}>
                  <p className="m-0 mb-1.5">{zh ? question.question_zh : question.question_en}</p>
                  <div className="flex flex-col gap-1.5">
                    {(['A', 'B'] as const).map((choice) => {
                      const active = picked === choice;
                      return (
                        <button
                          key={choice}
                          type="button"
                          className={`cursor-pointer rounded-md border px-3 py-1.5 text-left text-[13px] transition-[border] hover:bg-[var(--td-bg-color-container-hover,#f3f3f3)] ${active ? 'border-[var(--td-brand-color,#0052d9)] text-[var(--td-brand-color,#0052d9)]' : 'border-[var(--td-component-stroke,#dcdcdc)] text-inherit'}`}
                          aria-pressed={active ? 'true' : 'false'}
                          data-mbti-option={choice}
                          onClick={() => choose(question.id, choice)}
                        >{zh ? (choice === 'A' ? question.option_a_zh : question.option_b_zh) : (choice === 'A' ? question.option_a_en : question.option_b_en)}</button>
                      );
                    })}
                  </div>
                </li>
              );
            })}
          </ol>
          {submitFailed ? (
            <p className="m-0 text-[13px] text-[var(--td-error-color,#d54941)]" role="alert">{t('agentEditor.personalization.testSubmitError')}</p>
          ) : null}
          <div className="flex justify-end gap-2">
            <button
              type="button"
              className={`${AE_BTN} border bg-[var(--td-brand-color,#0052d9)] border-[var(--td-brand-color,#0052d9)] text-white hover:bg-[var(--td-brand-color-hover,#266fe8)] disabled:hover:bg-[var(--td-brand-color,#0052d9)]`}
              data-mbti-submit
              disabled={!allAnswered || submitting}
              onClick={() => void handleSubmit()}
            >{submitting ? t('common.loading') : t('agentEditor.personalization.testSubmit')}</button>
          </div>
        </>
      ) : null}
    </div>
  );

  const renderResult = () => {
    if (!score) return null;
    const profile = score.profile;
    return (
      <div className="flex flex-col gap-3" data-mbti-stage="result">
        <div className="flex items-baseline gap-2.5">
          <span className="text-[20px] font-semibold" style={{ color: profile.color }} data-mbti-result-code>{score.code}</span>
          <span className="text-[14px] font-medium">{zh ? profile.name_zh : profile.name_en}</span>
          <span className="text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{profile.nickname_zh}</span>
        </div>
        <div className="flex flex-col gap-2" data-mbti-result-bars>
          {MBTI_AXES.map(({ key, letters }) => (
            <DimensionBar
              key={key}
              axis={{ pole: score.dimensions[key].dominant, percent: score.dimensions[key].percent }}
              letters={letters}
              color={profile.color}
            />
          ))}
        </div>
        <p className="m-0 text-[13px] leading-6 text-[var(--td-text-color-primary,rgba(0,0,0,0.9))]">{zh ? profile.summary_zh : profile.summary_en}</p>
        <div className="flex justify-end gap-2">
          <button
            type="button"
            className={`${AE_BTN} border bg-[var(--td-brand-color,#0052d9)] border-[var(--td-brand-color,#0052d9)] text-white hover:bg-[var(--td-brand-color-hover,#266fe8)]`}
            data-mbti-apply
            onClick={() => onApply(score.code)}
          >{t('agentEditor.personalization.testApply')}</button>
        </div>
      </div>
    );
  };

  return (
    <Dialog
      open={open}
      title={stage === 'result' ? t('agentEditor.personalization.testResultTitle') : t('agentEditor.personalization.testTitle')}
      onClose={onClose}
      closeLabel={t('common.close')}
    >
      {stage === 'intro' ? renderIntro() : stage === 'questions' ? renderQuestions() : renderResult()}
    </Dialog>
  );
}
