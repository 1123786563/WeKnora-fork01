package career

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEvaluateOpportunityHardOutcomesArePinnedToEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, jd, year, want string
		citation             bool
	}{
		{name: "2026 graduate fails isolated 2027 only", jd: "仅限2027届", year: "2026", want: EvaluationIneligible, citation: true},
		{name: "graduation year with 届 suffix is normalized", jd: "仅限2027届", year: "2026届", want: EvaluationIneligible, citation: true},
		{name: "graduation date is normalized", jd: "仅限2027届", year: "2026-06-30", want: EvaluationIneligible, citation: true},
		{name: "2027 graduate matches sole recognized rule", jd: "仅限2027届。", year: "2027", want: EvaluationEligible, citation: true},
		{name: "ASCII full stop terminates declaration", jd: "仅限 2027 届.", year: "2026", want: EvaluationIneligible, citation: true},
		{name: "surrounding whitespace preserves raw citation", jd: " \n仅限2027届。\r\n", year: "2026", want: EvaluationIneligible, citation: true},
		{name: "fullwidth question is not affirmative", jd: "仅限2027届？", year: "2026", want: EvaluationUnknown},
		{name: "ASCII question is not affirmative", jd: "仅限2027届?", year: "2026", want: EvaluationUnknown},
		{name: "additional symbol is not ignored as punctuation", jd: "仅限2027届✅", year: "2027", want: EvaluationUnknown},
		{name: "missing confirmed graduation year is unknown", jd: "仅限2027届", want: EvaluationUnknown, citation: true},
		{name: "alternative batch is unknown", jd: "仅限2027届或2026届", year: "2026", want: EvaluationUnknown},
		{name: "negated graduation rule is unknown", jd: "并非仅限2027届", year: "2026", want: EvaluationUnknown},
		{name: "unparsed same-line condition is unknown", jd: "仅限2027届，要求本科及以上", year: "2026", want: EvaluationUnknown},
		{name: "skill field makes hard rule unknown", jd: "仅限2027届\n技能：Go", year: "2026", want: EvaluationUnknown},
		{name: "project field makes hard rule unknown", jd: "仅限2027届\n项目：X", year: "2026", want: EvaluationUnknown},
		{name: "skill and project fields make hard rule unknown", jd: "技能：Go\n仅限2027届\n项目：X", year: "2026", want: EvaluationUnknown},
		{name: "implicit exception in skill field makes hard rule unknown", jd: "仅限2027届\n技能：Go，其他批次均可", year: "2026", want: EvaluationUnknown},
		{name: "wrapped alternative on following line is unknown", jd: "仅限2027届\n或2026届", year: "2026", want: EvaluationUnknown},
		{name: "wrapped alternative before candidate is unknown", jd: "2026届或\n仅限2027届", year: "2026", want: EvaluationUnknown},
		{name: "second graduation option on following line is unknown", jd: "仅限2027届\n2026届亦可", year: "2026", want: EvaluationUnknown},
		{name: "negation prefix on prior line is unknown", jd: "并非\n仅限2027届", year: "2026", want: EvaluationUnknown},
		{name: "negation directly before clause is unknown", jd: "非仅限2027届", year: "2026", want: EvaluationUnknown},
		{name: "punctuation separated year list is unknown", jd: "仅限2027届、2028届", year: "2026", want: EvaluationUnknown},
		{name: "CRLF wrapped alternative is unknown", jd: "仅限2027届\r\n或2026届", year: "2026", want: EvaluationUnknown},
		{name: "CRLF prior alternative is unknown", jd: "2026届或\r\n仅限2027届", year: "2026", want: EvaluationUnknown},
		{name: "CRLF skill field makes hard rule unknown", jd: "仅限2027届\r\n技能：Go", year: "2026", want: EvaluationUnknown},
		{name: "later unlabeled graduation alternative makes result unknown", jd: "仅限2027届\n技能：Go\n2026届亦可", year: "2026", want: EvaluationUnknown},
		{name: "embedded alternative in skill field makes result unknown", jd: "仅限2027届\n技能：Go，2026届亦可", year: "2026", want: EvaluationUnknown},
		{name: "CRLF later graduation alternative makes result unknown", jd: "仅限2027届\r\n技能：Go\r\n2026届亦可", year: "2026", want: EvaluationUnknown},
		{name: "CRLF embedded alternative in skill field makes result unknown", jd: "仅限2027届\r\n技能：Go，2026届亦可", year: "2026", want: EvaluationUnknown},
		{name: "labeled note with year makes result unknown", jd: "仅限2027届\n备注：2026届可报", year: "2026", want: EvaluationUnknown},
		{name: "project field with graduation lexeme makes result unknown", jd: "仅限2027届\n项目：2026届亦可", year: "2026", want: EvaluationUnknown},
		{name: "skill field with unrestricted batch wording makes result unknown", jd: "仅限2027届\n技能：不限届别", year: "2026", want: EvaluationUnknown},
		{name: "alternative language in soft field makes result unknown", jd: "仅限2027届\n技能：Go或Java", year: "2026", want: EvaluationUnknown},
		{name: "negation language in soft field makes result unknown", jd: "仅限2027届\n项目：非应届限制", year: "2026", want: EvaluationUnknown},
		{name: "publication year in allowed field makes result unknown", jd: "仅限2027届\n项目：2024年论文", year: "2026", want: EvaluationUnknown},
		{name: "unrecognized continuation makes result unknown", jd: "仅限2027届\n要求应届毕业生", year: "2026", want: EvaluationUnknown},
		{name: "another sentence makes result unknown", jd: "仅限2027届。岗位开放", year: "2026", want: EvaluationUnknown},
		{name: "another punctuation mark makes result unknown", jd: "仅限2027届！", year: "2026", want: EvaluationUnknown},
		{name: "missing graduation condition is unknown", jd: "要求熟悉 Go 并具备项目经验", year: "2026", want: EvaluationUnknown},
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
			if tc.citation {
				require.NotNil(t, read.Hard.Rules[0].JobEvidence)
				evidence := read.Hard.Rules[0].JobEvidence
				require.Equal(t, tc.jd[evidence.SpanStart:evidence.SpanEnd], evidence.QuotedText)
				require.Equal(t, strings.Index(tc.jd, "仅限"), evidence.SpanStart)
				require.Equal(t, strings.Index(tc.jd, "届")+len("届"), evidence.SpanEnd)
			} else {
				require.Nil(t, read.Hard.Rules[0].JobEvidence)
			}
			if tc.year != "" && tc.want != EvaluationUnknown {
				require.NotNil(t, read.Hard.Rules[0].ProfileEvidence)
				require.Equal(t, tc.year, read.Hard.Rules[0].ProfileEvidence.Value)
				require.Equal(t, got.ProfileRevision, read.Hard.Rules[0].ProfileEvidence.Revision)
				require.NotZero(t, read.Hard.Rules[0].ProfileEvidence.ConfirmedAt)
				require.Equal(t, Source{Kind: "manual"}, read.Hard.Rules[0].ProfileEvidence.Source)
				require.Equal(t, "owner", read.Hard.Rules[0].ProfileEvidence.Confirmation.UserID)
			} else if tc.year == "" {
				require.Nil(t, read.Hard.Rules[0].ProfileEvidence)
			} else if tc.want == EvaluationUnknown {
				require.Nil(t, read.Hard.Rules[0].ProfileEvidence, "an ambiguous graduation block cannot use profile evidence")
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

func TestEvaluateOpportunityRecognizesConfirmedCareerFormGraduationAliases(t *testing.T) {
	t.Run("career form key cites original confirmed fact", func(t *testing.T) {
		o, ctx := newOpportunityOffice(t, "owner", 161)
		act, err := o.Act(ctx, "confirm", "", "毕业时间", "2026", "confirm-ui-year", 0, Source{Kind: "manual"})
		require.NoError(t, err)
		job, err := o.ImportJD(ctx, ImportJDInput{RequestID: "job", RawText: "仅限2027届"})
		require.NoError(t, err)
		receipt, err := o.EvaluateOpportunity(ctx, EvaluateInput{RequestID: "eval", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID})
		require.NoError(t, err)
		require.Equal(t, EvaluationIneligible, receipt.Status)
		read := mustEvaluation(t, o, ctx, receipt.EvaluationID)
		require.Equal(t, "毕业时间", read.Hard.Rules[0].ProfileEvidence.FactKey)
		require.Equal(t, act.Revision, read.Hard.Rules[0].ProfileEvidence.FactRevision)
		require.Equal(t, "2026", read.Hard.Rules[0].ProfileEvidence.Value)
	})

	for _, tc := range []struct {
		name, key, value, want string
	}{
		{name: "resume date alias is normalized", key: "education.graduation_date", value: "2026-06-30", want: EvaluationIneligible},
		{name: "resume date alias can match", key: "education.graduation_date", value: "2027-06-30", want: EvaluationEligible},
		{name: "malformed recognized alias is unknown", key: "毕业时间", value: "预计2026", want: EvaluationUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, ctx := newOpportunityOffice(t, "owner", 162)
			_, err := o.Confirm(ctx, tc.key, tc.value, "confirm", 0, Source{Kind: "manual"})
			require.NoError(t, err)
			job, err := o.ImportJD(ctx, ImportJDInput{RequestID: "job", RawText: "仅限2027届"})
			require.NoError(t, err)
			receipt, err := o.EvaluateOpportunity(ctx, EvaluateInput{RequestID: "eval", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID})
			require.NoError(t, err)
			require.Equal(t, tc.want, receipt.Status)
		})
	}

	t.Run("all aliases are checked and equal aliases select deterministically", func(t *testing.T) {
		o, ctx := newOpportunityOffice(t, "owner", 163)
		_, err := o.Confirm(ctx, "毕业时间", "2026", "confirm-ui", 0, Source{Kind: "manual"})
		require.NoError(t, err)
		_, err = o.Confirm(ctx, "education.graduation_date", "2026-05-01", "confirm-date", 1, Source{Kind: "manual"})
		require.NoError(t, err)
		_, err = o.Confirm(ctx, "education.graduation_year", "2026", "confirm-year", 2, Source{Kind: "manual"})
		require.NoError(t, err)
		job, err := o.ImportJD(ctx, ImportJDInput{RequestID: "job", RawText: "仅限2027届"})
		require.NoError(t, err)
		receipt, err := o.EvaluateOpportunity(ctx, EvaluateInput{RequestID: "eval", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID})
		require.NoError(t, err)
		require.Equal(t, EvaluationIneligible, receipt.Status)
		require.Equal(t, "education.graduation_year", mustEvaluation(t, o, ctx, receipt.EvaluationID).Hard.Rules[0].ProfileEvidence.FactKey)
	})

	t.Run("third contradictory alias makes result unknown", func(t *testing.T) {
		o, ctx := newOpportunityOffice(t, "owner", 164)
		for i, fact := range []struct{ key, value string }{{"graduation_year", "2027"}, {"education.graduation_year", "2027"}, {"毕业时间", "2026"}} {
			_, err := o.Confirm(ctx, fact.key, fact.value, "confirm-"+fact.key, uint64(i), Source{Kind: "manual"})
			require.NoError(t, err)
		}
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
	job, err := o.ImportJD(ctx, ImportJDInput{RequestID: "job", RawText: "仅限2027届\n技能：熟悉Python，Compiler项目优先，意向地点Hangzhou"})
	require.NoError(t, err)
	result, err := o.EvaluateOpportunity(ctx, EvaluateInput{RequestID: "eval-soft", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID})
	require.NoError(t, err)
	read := mustEvaluation(t, o, ctx, result.EvaluationID)
	require.Equal(t, EvaluationUnknown, read.Hard.Overall)
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

func TestEvaluationShortLatinSoftMatchRequiresASCIIWordBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, jd string
		matched  bool
		quote    string
	}{
		{name: "embedded token is not a match", jd: "Google provides cloud tools"},
		{name: "standalone token is cited", jd: "Go developer", matched: true, quote: "Go"},
		{name: "token before non-ascii language suffix is cited", jd: "Google, Go语言开发", matched: true, quote: "Go"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, ctx := newOpportunityOffice(t, "owner", 160)
			_, err := o.Confirm(ctx, "skill.go", "Go", "confirm-go", 0, Source{Kind: "manual"})
			require.NoError(t, err)
			job, err := o.ImportJD(ctx, ImportJDInput{RequestID: "job", RawText: tc.jd})
			require.NoError(t, err)
			receipt, err := o.EvaluateOpportunity(ctx, EvaluateInput{RequestID: "eval", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID})
			require.NoError(t, err)
			got := mustEvaluation(t, o, ctx, receipt.EvaluationID)
			if !tc.matched {
				require.Empty(t, got.Soft.Matches)
				return
			}
			require.Len(t, got.Soft.Matches, 1)
			match := got.Soft.Matches[0]
			require.Equal(t, "skill", match.Kind)
			require.Equal(t, tc.quote, match.JobEvidence.QuotedText)
			require.Equal(t, tc.jd[match.JobEvidence.SpanStart:match.JobEvidence.SpanEnd], match.JobEvidence.QuotedText)
			require.Equal(t, uint64(1), match.ProfileEvidence.FactRevision)
			require.Equal(t, Source{Kind: "manual"}, match.ProfileEvidence.Source)
		})
	}
}

func TestConcurrentChangedEvaluationIntentReturnsHTTPConflict(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "evaluation-intent-race.db")
	db, err := gorm.Open(sqlite.Open("file:"+dbPath+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	o, err := NewOffice(db)
	require.NoError(t, err)
	ctx := WithScope(context.Background(), Scope{UserID: "owner", TenantID: 161})
	require.NoError(t, o.ClaimSpace(ctx))
	firstJob, err := o.ImportJD(ctx, ImportJDInput{RequestID: "job-one", RawText: "仅限2027届"})
	require.NoError(t, err)
	secondJob, err := o.ImportJD(ctx, ImportJDInput{RequestID: "job-two", RawText: "仅限2028届"})
	require.NoError(t, err)
	owner := &types.TenantMember{UserID: "owner", TenantID: 161, Role: types.TenantRoleOwner}
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{owner}}}

	missed := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseBarrier := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseBarrier()
	var misses atomic.Int32
	o.afterEvaluationReceiptMiss = func() {
		if misses.Add(1) == 1 {
			close(missed)
			<-release
		}
	}

	request := func(job OpportunityReceipt) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		base := context.WithValue(context.Background(), types.UserIDContextKey, "owner")
		base = context.WithValue(base, types.TenantIDContextKey, uint64(161))
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/career/evaluations", strings.NewReader(`{"requestId":"same-id","opportunityId":"`+job.OpportunityID+`","snapshotId":"`+job.SnapshotID+`"}`)).WithContext(base)
		h.EvaluateOpportunity(c)
		return recorder
	}
	started := make(chan *httptest.ResponseRecorder, 1)
	go func() { started <- request(secondJob) }()
	select {
	case <-missed:
	case <-time.After(2 * time.Second):
		releaseBarrier()
		t.Fatal("second evaluation did not reach the deterministic receipt-miss barrier")
	}
	committed := request(firstJob)
	require.Equal(t, http.StatusOK, committed.Code, committed.Body.String())
	releaseBarrier()
	select {
	case raced := <-started:
		require.Equal(t, http.StatusConflict, raced.Code, raced.Body.String())
		require.Contains(t, raced.Body.String(), `"code":"idempotency_conflict"`)
	case <-time.After(2 * time.Second):
		t.Fatal("racing evaluation did not finish")
	}
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
	missing := request("GET", "/api/v1/career/evaluations/missing-evaluation", "", "owner", 155)
	require.Equal(t, 404, missing.Code, missing.Body.String())
	require.Contains(t, missing.Body.String(), `"code":"not_found"`)
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
