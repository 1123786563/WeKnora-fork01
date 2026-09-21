/**
 * 28-question MBTI test modal (Octop M1 port — no Vue baseline). Opened from
 * PersonaSection's "take the test" button: intro (with the for-entertainment
 * disclaimer) → questions (zh/en by app locale, A/B choices, progress counter,
 * all answers required before submit) → result (code + profile card + the
 * Task 8 DimensionBar semantics) whose "apply" button hands the code back to
 * the agent editor. The UI gates submit on ALL questions being answered
 * (stricter than the backend's >=20 minimum, per plan) so the empty-code
 * result path is unreachable.
 *
 * TDesign 同构迁移（Task 9）：@weknora/ui Dialog → 本地 overlay（样式在
 * agents.td.css 的 wk-ae-mbti 段），按钮换 tdesign-react Button。
 */
import { useEffect, useState } from 'react';
import type { MbtiQuestion, MbtiScore, WeKnoraClient } from '@weknora/api-client';
import { Button } from 'tdesign-react';
import { usePreferredLocale } from '../locale.ts';
import type { Translate } from './agent-editor.ts';
import { DimensionBar, MBTI_AXES } from './PersonaSection.tsx';

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

  // The modal owns the Escape key while stacked above the agent editor
  // (AgentEditorModal's global handler defers to .wk-dialog-backdrop layers).
  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [open, onClose]);

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
    <div className="wk-ae-mbti-stage" data-mbti-stage="intro">
      <p className="wk-ae-mbti-lead">{t('agentEditor.personalization.testIntroDesc')}</p>
      <p className="wk-ae-mbti-meta" data-mbti-disclaimer>{t('agentEditor.personalization.testDisclaimer')}</p>
      <div className="wk-ae-mbti-footer wk-ae-mbti-footer--inset">
        <Button theme="primary" data-mbti-start onClick={() => setStage('questions')}>{t('agentEditor.personalization.testStart')}</Button>
      </div>
    </div>
  );

  const renderQuestions = () => (
    <div className="wk-ae-mbti-stage" data-mbti-stage="questions">
      {questions === null && !questionsFailed ? (
        <p className="wk-ae-mbti-meta" role="status">{t('common.loading')}</p>
      ) : null}
      {questionsFailed ? (
        <>
          <p className="wk-ae-mbti-meta" role="alert" style={{ color: 'var(--td-error-color)' }}>{t('agentEditor.personalization.testLoadFailed')}</p>
          <Button variant="outline" onClick={() => setQuestionAttempt((current) => current + 1)}>{t('common.retry')}</Button>
        </>
      ) : null}
      {questions !== null ? (
        <>
          <p className="wk-ae-mbti-meta" data-mbti-progress>
            {t('agentEditor.personalization.testProgress', { answered: answeredCount, total })}
          </p>
          <ol className="wk-ae-mbti-questions">
            {questions.map((question) => {
              const picked = answers[String(question.id)];
              return (
                <li key={question.id} className="wk-ae-mbti-question" data-mbti-q={question.id}>
                  <p className="wk-ae-mbti-question-text">{zh ? question.question_zh : question.question_en}</p>
                  <div className="wk-ae-mbti-options">
                    {(['A', 'B'] as const).map((choice) => {
                      const active = picked === choice;
                      return (
                        <button
                          key={choice}
                          type="button"
                          className={`wk-ae-mbti-option${active ? ' wk-ae-mbti-option--active' : ''}`}
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
            <p className="wk-ae-mbti-meta" role="alert" style={{ color: 'var(--td-error-color)' }}>{t('agentEditor.personalization.testSubmitError')}</p>
          ) : null}
          <div className="wk-ae-mbti-footer">
            <Button theme="primary" data-mbti-submit disabled={!allAnswered || submitting} onClick={() => void handleSubmit()}>
              {submitting ? t('common.loading') : t('agentEditor.personalization.testSubmit')}
            </Button>
          </div>
        </>
      ) : null}
    </div>
  );

  const renderResult = () => {
    if (!score) return null;
    const profile = score.profile;
    return (
      <div className="wk-ae-mbti-stage" data-mbti-stage="result">
        <div className="wk-ae-mbti-result-head">
          <span className="wk-ae-mbti-result-code" style={{ color: profile.color }} data-mbti-result-code>{score.code}</span>
          <span className="wk-ae-mbti-result-name">{zh ? profile.name_zh : profile.name_en}</span>
          <span className="wk-ae-mbti-meta">{profile.nickname_zh}</span>
        </div>
        <div className="wk-ae-persona-dimensions" data-mbti-result-bars>
          {MBTI_AXES.map(({ key, letters }) => (
            <DimensionBar
              key={key}
              axis={{ pole: score.dimensions[key].dominant, percent: score.dimensions[key].percent }}
              letters={letters}
              color={profile.color}
            />
          ))}
        </div>
        <p className="wk-ae-mbti-lead">{zh ? profile.summary_zh : profile.summary_en}</p>
        <div className="wk-ae-mbti-footer">
          <Button theme="primary" data-mbti-apply onClick={() => onApply(score.code)}>{t('agentEditor.personalization.testApply')}</Button>
        </div>
      </div>
    );
  };

  if (!open) return null;
  return (
    <div className="wk-ae-mbti-overlay wk-dialog-backdrop" onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}>
      <div className="wk-ae-mbti-modal" role="dialog" aria-modal="true" aria-label={t('agentEditor.personalization.testTitle')}>
        <div className="wk-ae-mbti-header">
          <h3 className="wk-ae-mbti-title">{stage === 'result' ? t('agentEditor.personalization.testResultTitle') : t('agentEditor.personalization.testTitle')}</h3>
          <Button variant="text" theme="default" shape="square" aria-label={t('common.close')} onClick={onClose}>✕</Button>
        </div>
        <div className="wk-ae-mbti-body">
          {stage === 'intro' ? renderIntro() : stage === 'questions' ? renderQuestions() : renderResult()}
        </div>
      </div>
    </div>
  );
}
