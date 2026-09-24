package career

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEvaluateOpportunityHardOutcomesArePinnedToEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, jd, year, want string
	}{
		{name: "2026 graduate fails 2027 only", jd: "仅限2027届", year: "2026", want: EvaluationIneligible},
		{name: "graduation year with 届 suffix is normalized", jd: "仅限2027届", year: "2026届", want: EvaluationIneligible},
		{name: "graduation date is normalized", jd: "仅限2027届", year: "2026-06-30", want: EvaluationIneligible},
		{name: "2027 graduate matches sole recognized rule", jd: "仅限2027届。", year: "2027", want: EvaluationEligible},
		{name: "additional symbol is not ignored as punctuation", jd: "仅限2027届✅", year: "2027", want: EvaluationUnknown},
		{name: "missing graduation year is unknown", jd: "仅限2027届", want: EvaluationUnknown},
		{name: "conflicting batch is unknown", jd: "仅限2027届或2028届", year: "2027", want: EvaluationUnknown},
		{name: "unparsed requirement remains unknown", jd: "仅限2027届，要求本科及以上", year: "2027", want: EvaluationUnknown},
		{name: "explicit ineligible wins over unparsed requirement", jd: "仅限2027届，要求本科及以上", year: "2026", want: EvaluationIneligible},
		{name: "bare batch is not treated as an explicit rule", jd: "2027届", year: "2027", want: EvaluationUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, ctx := newOpportunityOffice(t, "owner", 150)
			if tc.year != "" {
				_, err := o.Confirm(ctx, "education.graduation_year", tc.year, "confirm-year", 0, Source{Kind: "manual"})
				require.NoError(t, err)
			}
			job, err := o.ImportJD(ctx, ImportJDInput{RequestID: "job", RawText: tc.jd})
			require.NoError(t, err)

			got, err := o.EvaluateOpportunity(ctx, EvaluateInput{RequestID: "eval", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID})
			require.NoError(t, err)
			require.Equal(t, "evaluation_created", got.Kind)
			require.Equal(t, tc.want, got.Status)
			read, err := o.Evaluation(ctx, got.EvaluationID)
			require.NoError(t, err)
			require.Equal(t, job.SnapshotID, read.Snapshot.SnapshotID)
			require.Equal(t, job.OpportunityID, read.Snapshot.OpportunityID)
			require.Equal(t, got.ProfileRevision, read.ProfileRevision)
			require.Equal(t, "graduation_year", read.Hard.Rules[0].RuleID)
			require.NotNil(t, read.Hard.Rules[0].JobEvidence)
			evidence := read.Hard.Rules[0].JobEvidence
			require.Equal(t, tc.jd[evidence.SpanStart:evidence.SpanEnd], evidence.QuotedText)
			if tc.year != "" && tc.want != EvaluationUnknown {
				require.NotNil(t, read.Hard.Rules[0].ProfileEvidence)
				require.Equal(t, tc.year, read.Hard.Rules[0].ProfileEvidence.Value)
				require.Equal(t, got.ProfileRevision, read.Hard.Rules[0].ProfileEvidence.Revision)
				require.NotZero(t, read.Hard.Rules[0].ProfileEvidence.ConfirmedAt)
				require.Equal(t, Source{Kind: "manual"}, read.Hard.Rules[0].ProfileEvidence.Source)
				require.Equal(t, "owner", read.Hard.Rules[0].ProfileEvidence.Confirmation.UserID)
			} else if tc.year == "" {
				require.Nil(t, read.Hard.Rules[0].ProfileEvidence)
			}
			encoded, err := json.Marshal(read)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), "score")
			require.NotContains(t, string(encoded), "probability")
		})
	}
}

func TestEvaluationUnknownForUnconfirmedMalformedAndContradictoryGraduationFacts(t *testing.T) {
	t.Run("unconfirmed proposal", func(t *testing.T) {
		o, ctx := newOpportunityOffice(t, "owner", 157)
		_, err := o.Propose(ctx, "graduation_year", "2026", "propose-year", 0, Source{Kind: "resume_extraction"})
		require.NoError(t, err)
		job, err := o.ImportJD(ctx, ImportJDInput{RequestID: "job", RawText: "仅限2027届"})
		require.NoError(t, err)
		receipt, err := o.EvaluateOpportunity(ctx, EvaluateInput{RequestID: "eval", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID})
		require.NoError(t, err)
		require.Equal(t, EvaluationUnknown, receipt.Status)
		require.Nil(t, mustEvaluation(t, o, ctx, receipt.EvaluationID).Hard.Rules[0].ProfileEvidence)
	})
	t.Run("malformed confirmed value", func(t *testing.T) {
		o, ctx := newOpportunityOffice(t, "owner", 158)
		_, err := o.Confirm(ctx, "graduation_year", "2026-invalid", "confirm-year", 0, Source{Kind: "manual"})
		require.NoError(t, err)
		job, err := o.ImportJD(ctx, ImportJDInput{RequestID: "job", RawText: "仅限2027届"})
		require.NoError(t, err)
		receipt, err := o.EvaluateOpportunity(ctx, EvaluateInput{RequestID: "eval", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID})
		require.NoError(t, err)
		require.Equal(t, EvaluationUnknown, receipt.Status)
	})
	t.Run("contradictory confirmed aliases", func(t *testing.T) {
		o, ctx := newOpportunityOffice(t, "owner", 159)
		_, err := o.Confirm(ctx, "graduation_year", "2027", "confirm-short", 0, Source{Kind: "manual"})
		require.NoError(t, err)
		_, err = o.Confirm(ctx, "education.graduation_year", "2026", "confirm-long", 1, Source{Kind: "manual"})
		require.NoError(t, err)
		job, err := o.ImportJD(ctx, ImportJDInput{RequestID: "job", RawText: "仅限2027届"})
		require.NoError(t, err)
		receipt, err := o.EvaluateOpportunity(ctx, EvaluateInput{RequestID: "eval", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID})
		require.NoError(t, err)
		require.Equal(t, EvaluationUnknown, receipt.Status)
	})
}

func TestEvaluationReplayKeepsOriginalProfileVersionAndNewEvaluationIsImmutable(t *testing.T) {
	o, ctx := newOpportunityOffice(t, "owner", 151)
	_, err := o.Confirm(ctx, "graduation_year", "2026", "confirm-old", 0, Source{Kind: "manual"})
	require.NoError(t, err)
	job, err := o.ImportJD(ctx, ImportJDInput{RequestID: "job", RawText: "仅限2027届"})
	require.NoError(t, err)
	input := EvaluateInput{RequestID: "same-eval", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID}
	first, err := o.EvaluateOpportunity(ctx, input)
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "graduation_year", "2027", "confirm-new", 1, Source{Kind: "manual"})
	require.NoError(t, err)
	replayed, err := o.EvaluateOpportunity(ctx, input)
	require.NoError(t, err)
	require.Equal(t, first, replayed)
	require.ErrorIs(t, func() error {
		_, e := o.EvaluateOpportunity(ctx, EvaluateInput{RequestID: input.RequestID, OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID, ProfileRevision: ptrUint64(2)})
		return e
	}(), ErrIdempotencyConflict)
	newInput := EvaluateInput{RequestID: "new-eval", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID}
	second, err := o.EvaluateOpportunity(ctx, newInput)
	require.NoError(t, err)
	require.NotEqual(t, first.EvaluationID, second.EvaluationID)
	require.Equal(t, uint64(2), second.ProfileRevision)
	old, err := o.Evaluation(ctx, first.EvaluationID)
	require.NoError(t, err)
	require.Equal(t, EvaluationIneligible, old.Hard.Overall)
	require.Equal(t, "2026", old.Hard.Rules[0].ProfileEvidence.Value)
	require.Equal(t, EvaluationEligible, mustEvaluation(t, o, ctx, second.EvaluationID).Hard.Overall)
	pinnedOld, err := o.EvaluateOpportunity(ctx, EvaluateInput{RequestID: "pinned-old", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID, ProfileRevision: ptrUint64(1)})
	require.NoError(t, err)
	require.Equal(t, EvaluationIneligible, pinnedOld.Status)
	_, err = o.EvaluateOpportunity(ctx, EvaluateInput{RequestID: "future-pin", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID, ProfileRevision: ptrUint64(3)})
	require.ErrorIs(t, err, ErrRevisionConflict)
}

func TestEvaluationScopesReadsAndReconcilesCanceledAcknowledgement(t *testing.T) {
	o, ctx := newOpportunityOffice(t, "owner", 152)
	job, err := o.ImportJD(ctx, ImportJDInput{RequestID: "job", RawText: "仅限2027届"})
	require.NoError(t, err)
	input := EvaluateInput{RequestID: "cancelled-eval", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID}
	cancelCtx, cancel := context.WithCancel(ctx)
	o.afterEvaluationCommit = func() error { cancel(); return context.Canceled }
	receipt, err := o.EvaluateOpportunity(cancelCtx, input)
	require.NoError(t, err)
	require.Equal(t, "evaluation_created", receipt.Kind)
	recoveryCtx := WithScope(context.Background(), Scope{UserID: "owner", TenantID: 152})
	recovered, err := o.FindEvaluationReceipt(recoveryCtx, input.RequestID)
	require.NoError(t, err)
	require.Equal(t, receipt, recovered)
	otherUser := WithScope(context.Background(), Scope{UserID: "intruder", TenantID: 152})
	_, err = o.Evaluation(otherUser, receipt.EvaluationID)
	require.ErrorIs(t, err, ErrUnauthorized)
	otherTenant := WithScope(context.Background(), Scope{UserID: "owner", TenantID: 153})
	_, err = o.FindEvaluationReceipt(otherTenant, input.RequestID)
	require.ErrorIs(t, err, ErrUnauthorized)
	_, err = o.Evaluation(ctx, "missing-evaluation")
	require.ErrorIs(t, err, ErrEvaluationNotFound)
}

func TestEvaluationUsesOnlyConfirmedSoftEvidenceAndKeepsItSeparate(t *testing.T) {
	o, ctx := newOpportunityOffice(t, "owner", 154)
	_, err := o.Propose(ctx, "skill.go", "Go", "propose-go", 0, Source{Kind: "resume_extraction"})
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "graduation_year", "2026", "confirm-year", 1, Source{Kind: "manual"})
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "skill.python", "Python", "confirm-python", 2, Source{Kind: "manual"})
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "project.compiler", "Compiler", "confirm-project", 3, Source{Kind: "manual"})
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "preference.location", "Hangzhou", "confirm-location", 4, Source{Kind: "manual"})
	require.NoError(t, err)
	job, err := o.ImportJD(ctx, ImportJDInput{RequestID: "job", RawText: "仅限2027届，熟悉Python，Compiler项目优先，意向地点Hangzhou"})
	require.NoError(t, err)
	result, err := o.EvaluateOpportunity(ctx, EvaluateInput{RequestID: "eval-soft", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID})
	require.NoError(t, err)
	read := mustEvaluation(t, o, ctx, result.EvaluationID)
	require.Equal(t, EvaluationIneligible, read.Hard.Overall)
	require.Len(t, read.Soft.Matches, 3)
	for _, match := range read.Soft.Matches {
		require.NotEmpty(t, match.ProfileEvidence.FactKey)
		require.NotZero(t, match.ProfileEvidence.FactRevision)
		require.NotEmpty(t, match.JobEvidence.QuotedText)
		require.Equal(t, job.SnapshotID, match.JobEvidence.SnapshotID)
	}
	encoded, err := json.Marshal(read)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), `"skill.go"`)
	require.Contains(t, string(encoded), `"kind":"project"`)
}

func TestEvaluationHTTPContractAndOwnerScope(t *testing.T) {
	o, ctx := newOpportunityOffice(t, "owner", 155)
	job, err := o.ImportJD(ctx, ImportJDInput{RequestID: "job", RawText: "仅限2027届"})
	require.NoError(t, err)
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "owner", TenantID: 155, Role: types.TenantRoleOwner}}}}
	request := func(method, path, body, user string, tenant uint64) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		requestCtx := context.WithValue(context.Background(), types.UserIDContextKey, user)
		requestCtx = context.WithValue(requestCtx, types.TenantIDContextKey, tenant)
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(requestCtx)
		switch method {
		case "POST":
			h.EvaluateOpportunity(c)
		case "GET":
			if strings.Contains(path, "receipt") {
				h.EvaluationReceipt(c)
			} else {
				c.Params = gin.Params{{Key: "evaluationId", Value: strings.TrimPrefix(path, "/api/v1/career/evaluations/")}}
				h.Evaluation(c)
			}
		}
		return rec
	}
	body := `{"requestId":"http-eval","opportunityId":"` + job.OpportunityID + `","snapshotId":"` + job.SnapshotID + `"}`
	created := request("POST", "/api/v1/career/evaluations", body, "owner", 155)
	require.Equal(t, 200, created.Code, created.Body.String())
	var receipt EvaluationReceipt
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &receipt))
	require.Equal(t, "evaluation_created", receipt.Kind)
	require.Equal(t, job.SnapshotID, receipt.SnapshotID)
	read := request("GET", "/api/v1/career/evaluations/"+receipt.EvaluationID, "", "owner", 155)
	require.Equal(t, 200, read.Code, read.Body.String())
	require.Contains(t, read.Body.String(), job.SnapshotID)
	require.Equal(t, 403, request("GET", "/api/v1/career/evaluations/"+receipt.EvaluationID, "", "intruder", 155).Code)
	require.Equal(t, 200, request("GET", "/api/v1/career/evaluations/receipt?requestId=http-eval", "", "owner", 155).Code)
}

func TestEvaluationHTTPRejectsClientSuppliedAssessment(t *testing.T) {
	o, _ := newOpportunityOffice(t, "owner", 156)
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "owner", TenantID: 156, Role: types.TenantRoleOwner}}}}
	base := context.WithValue(context.WithValue(context.Background(), types.UserIDContextKey, "owner"), types.TenantIDContextKey, uint64(156))
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/api/v1/career/evaluations", strings.NewReader(`{"requestId":"forged","opportunityId":"x","snapshotId":"y","overall":"eligible","facts":[{"key":"graduation_year","value":"2027"}]}`)).WithContext(base)
	h.EvaluateOpportunity(c)
	require.Equal(t, 400, rec.Code, rec.Body.String())
}

func ptrUint64(v uint64) *uint64 { return &v }

func mustEvaluation(t *testing.T, o *Office, ctx context.Context, id string) Evaluation {
	t.Helper()
	got, err := o.Evaluation(ctx, id)
	require.NoError(t, err)
	return got
}
