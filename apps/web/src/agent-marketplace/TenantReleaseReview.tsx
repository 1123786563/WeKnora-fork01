import { useEffect, useState } from 'react';
import type { AgentRelease, ReleaseSubmission } from '@weknora/contracts';
import type { ReviewReleaseInput } from '@weknora/api-client';
import type { AgentMarketplaceApi } from './agent-marketplace-api.ts';

type ReviewerRole = 'viewer' | 'contributor' | 'admin' | 'owner';
type Decision = ReviewReleaseInput['decision'];

interface TenantReleaseReviewProps {
  api: AgentMarketplaceApi;
  role: ReviewerRole;
}

const canReview = (role: ReviewerRole): boolean => role === 'admin' || role === 'owner';
const json = (value: unknown): string => JSON.stringify(value, null, 2);

export function TenantReleaseReview({ api, role }: TenantReleaseReviewProps) {
  const [submissions, setSubmissions] = useState<ReleaseSubmission[]>([]);
  const [loading, setLoading] = useState(canReview(role));
  const [error, setError] = useState('');
  const [decisionByID, setDecisionByID] = useState<Record<string, Decision | undefined>>({});
  const [reasonByID, setReasonByID] = useState<Record<string, string | undefined>>({});
  const [busyID, setBusyID] = useState('');
  const [results, setResults] = useState<Record<string, { reviewID: string; release: AgentRelease | null }>>({});

  useEffect(() => {
    if (!canReview(role)) return;
    let active = true;
    void api.releases.listReviewQueue().then((queue) => {
      if (active) setSubmissions(queue);
    }).catch((cause: unknown) => {
      if (active) setError(cause instanceof Error ? cause.message : 'Could not load Tenant Release reviews');
    }).finally(() => {
      if (active) setLoading(false);
    });
    return () => { active = false; };
  }, [api, role]);

  const review = async (submission: ReleaseSubmission) => {
    const decision = decisionByID[submission.id];
    const reason = reasonByID[submission.id]?.trim() ?? '';
    if (!decision || (decision !== 'approved' && reason === '') || busyID !== '') return;
    setBusyID(submission.id);
    setError('');
    try {
      const input: ReviewReleaseInput = decision === 'approved'
        ? { expected_digest: submission.bundle_digest, decision }
        : { expected_digest: submission.bundle_digest, decision, reason };
      const result = await api.releases.review(submission.id, input);
      setResults((current) => ({ ...current, [submission.id]: { reviewID: result.review.id, release: result.release } }));
      setSubmissions((current) => current.filter((item) => item.id !== submission.id));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not review Tenant Release');
    } finally {
      setBusyID('');
    }
  };

  if (!canReview(role)) {
    return <section aria-label="Tenant Release review" data-tenant-release-review><h3>Tenant Release review</h3><p>Tenant Admin or Owner role is required to review Releases.</p></section>;
  }

  return (
    <section aria-label="Tenant Release review" data-tenant-release-review className="mb-7 grid gap-3">
      <div>
        <h3 className="m-0 text-[13px] font-semibold uppercase tracking-[0.04em] text-[rgba(23,26,29,0.45)]">Tenant Release review</h3>
        <p className="m-0 mt-1 text-[13px] text-[rgba(23,26,29,0.6)]">Review the fixed portable payload, Manifest, Dependency Lock, license, and digest before choosing a decision.</p>
      </div>
      {loading ? <p role="status">Loading Tenant Release review queue…</p> : null}
      {error ? <p role="alert" className="m-0 text-[13px] text-[#d54941]">{error}</p> : null}
      {!loading && submissions.length === 0 ? <p className="m-0 text-[13px] text-[rgba(23,26,29,0.6)]">No Tenant Releases are awaiting review.</p> : null}
      <div className="grid gap-3">
        {submissions.map((submission) => {
          const decision = decisionByID[submission.id];
          const reason = reasonByID[submission.id] ?? '';
          return (
            <article key={submission.id} data-review-submission={submission.id} className="grid gap-3 rounded-lg border border-[#e7e7ea] bg-surface p-4">
              <header>
                <h4 className="m-0 text-[15px] font-semibold">{String(submission.manifest.display_name ?? submission.semantic_version)}</h4>
                <p className="m-0 mt-1 text-[12px] text-[rgba(23,26,29,0.55)]">Submission {submission.id} · Agent Version {submission.agent_version_id}</p>
              </header>
	              <div className="grid gap-3 md:grid-cols-2">
	                <section><h5 className="m-0 text-[12px] font-semibold">Portable payload</h5><pre data-review-payload className="max-h-48 overflow-auto whitespace-pre-wrap break-words rounded bg-[rgba(127,127,127,0.07)] p-2 text-[11px]">{json(submission.payload)}</pre></section>
	                <section><h5 className="m-0 text-[12px] font-semibold">Manifest</h5><pre className="max-h-48 overflow-auto whitespace-pre-wrap break-words rounded bg-[rgba(127,127,127,0.07)] p-2 text-[11px]">{json(submission.manifest)}</pre></section>
                <section><h5 className="m-0 text-[12px] font-semibold">Dependency Lock</h5><pre className="max-h-48 overflow-auto whitespace-pre-wrap break-words rounded bg-[rgba(127,127,127,0.07)] p-2 text-[11px]">{json(submission.dependency_lock)}</pre></section>
              </div>
              <p className="m-0 text-[13px]">License: <strong>{String(submission.manifest.license_id ?? '—')}</strong></p>
              <p className="m-0 grid gap-1 text-[13px]">Reviewed digest
                <code data-review-digest className="break-all rounded bg-[rgba(127,127,127,0.07)] p-2 text-[12px]">{submission.bundle_digest}</code>
              </p>
              <div className="flex flex-wrap gap-2" aria-label={`Review decision for ${submission.id}`}>
                <button type="button" data-review-decision="approved" aria-pressed={decision === 'approved'} disabled={busyID !== ''} onClick={() => setDecisionByID((current) => ({ ...current, [submission.id]: 'approved' }))}>Approve</button>
                <button type="button" data-review-decision="rejected" aria-pressed={decision === 'rejected'} disabled={busyID !== ''} onClick={() => setDecisionByID((current) => ({ ...current, [submission.id]: 'rejected' }))}>Reject</button>
                <button type="button" data-review-decision="changes_requested" aria-pressed={decision === 'changes_requested'} disabled={busyID !== ''} onClick={() => setDecisionByID((current) => ({ ...current, [submission.id]: 'changes_requested' }))}>Request changes</button>
              </div>
              {decision && decision !== 'approved' ? (
                <label className="grid gap-1 text-[13px]">Reason required for this decision
                  <textarea data-review-reason value={reason} onInput={(event) => { const value = event.currentTarget.value; setReasonByID((current) => ({ ...current, [submission.id]: value })); }} rows={3} className="rounded border border-[#dcdcdc] px-2 py-1.5" />
                </label>
              ) : null}
              <button type="button" data-review-submit disabled={!decision || (decision !== 'approved' && reason.trim() === '') || busyID !== ''} onClick={() => void review(submission)} className="justify-self-start rounded bg-accent px-3 py-1.5 text-white disabled:cursor-not-allowed disabled:opacity-60">
                {busyID === submission.id ? 'Saving review…' : decision === 'approved' ? 'Approve and publish Release' : decision ? 'Submit review decision' : 'Choose a review decision'}
              </button>
            </article>
          );
        })}
      </div>
      {Object.entries(results).map(([submissionID, result]) => (
        <p key={submissionID} role="status" data-review-result={submissionID} className="m-0 text-[13px]">
          Review {result.reviewID} recorded.{result.release ? ` Published immutable Release ${result.release.id} (${result.release.bundle_digest}).` : ' No Release was created.'}
        </p>
      ))}
    </section>
  );
}
