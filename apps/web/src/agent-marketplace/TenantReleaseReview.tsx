import { useEffect, useState } from 'react';
import type { AgentRelease, ReleaseSubmission } from '@weknora/contracts';
import type { ReviewReleaseInput } from '@weknora/api-client';
import type { AgentMarketplaceApi } from './agent-marketplace-api.ts';
import './am-u.css';

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
    <section aria-label="Tenant Release review" data-tenant-release-review className="wk-amr-1">
      <div>
        <h3 className="wk-amr-2">Tenant Release review</h3>
        <p className="wk-amr-3">Review the fixed portable payload, Manifest, Dependency Lock, license, and digest before choosing a decision.</p>
      </div>
      {loading ? <p role="status">Loading Tenant Release review queue…</p> : null}
      {error ? <p role="alert" className="wk-amr-4">{error}</p> : null}
      {!loading && submissions.length === 0 ? <p className="wk-amr-5">No Tenant Releases are awaiting review.</p> : null}
      <div className="wk-amr-6">
        {submissions.map((submission) => {
          const decision = decisionByID[submission.id];
          const reason = reasonByID[submission.id] ?? '';
          return (
            <article key={submission.id} data-review-submission={submission.id} className="wk-amr-7">
              <header>
                <h4 className="wk-amr-8">{String(submission.manifest.display_name ?? submission.semantic_version)}</h4>
                <p className="wk-amr-9">Submission {submission.id} · Agent Version {submission.agent_version_id}</p>
              </header>
	              <div className="wk-amr-6 wk-amr-6-md">
	                <section><h5 className="wk-amr-10">Portable payload</h5><pre data-review-payload className="wk-amr-11">{json(submission.payload)}</pre></section>
	                <section><h5 className="wk-amr-10">Manifest</h5><pre className="wk-amr-11">{json(submission.manifest)}</pre></section>
                <section><h5 className="wk-amr-10">Dependency Lock</h5><pre className="wk-amr-11">{json(submission.dependency_lock)}</pre></section>
              </div>
              <p className="wk-amr-12">License: <strong>{String(submission.manifest.license_id ?? '—')}</strong></p>
              <p className="wk-amr-13">Reviewed digest
                <code data-review-digest className="wk-amr-14">{submission.bundle_digest}</code>
              </p>
              <div className="wk-amr-15" aria-label={`Review decision for ${submission.id}`}>
                <button type="button" data-review-decision="approved" aria-pressed={decision === 'approved'} disabled={busyID !== ''} onClick={() => setDecisionByID((current) => ({ ...current, [submission.id]: 'approved' }))}>Approve</button>
                <button type="button" data-review-decision="rejected" aria-pressed={decision === 'rejected'} disabled={busyID !== ''} onClick={() => setDecisionByID((current) => ({ ...current, [submission.id]: 'rejected' }))}>Reject</button>
                <button type="button" data-review-decision="changes_requested" aria-pressed={decision === 'changes_requested'} disabled={busyID !== ''} onClick={() => setDecisionByID((current) => ({ ...current, [submission.id]: 'changes_requested' }))}>Request changes</button>
              </div>
              {decision && decision !== 'approved' ? (
                <label className="wk-amr-16">Reason required for this decision
                  <textarea data-review-reason value={reason} onInput={(event) => { const value = event.currentTarget.value; setReasonByID((current) => ({ ...current, [submission.id]: value })); }} rows={3} className="wk-amr-17" />
                </label>
              ) : null}
              <button type="button" data-review-submit disabled={!decision || (decision !== 'approved' && reason.trim() === '') || busyID !== ''} onClick={() => void review(submission)} className="wk-amr-18 wk-amr-self-start">
                {busyID === submission.id ? 'Saving review…' : decision === 'approved' ? 'Approve and publish Release' : decision ? 'Submit review decision' : 'Choose a review decision'}
              </button>
            </article>
          );
        })}
      </div>
      {Object.entries(results).map(([submissionID, result]) => (
        <p key={submissionID} role="status" data-review-result={submissionID} className="wk-amr-12">
          Review {result.reviewID} recorded.{result.release ? ` Published immutable Release ${result.release.id} (${result.release.bundle_digest}).` : ' No Release was created.'}
        </p>
      ))}
    </section>
  );
}
