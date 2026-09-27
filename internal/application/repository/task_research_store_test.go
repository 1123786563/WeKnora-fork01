package repository_test

// T17 (#47) store evidence: delegations and annotations on the real migrated
// sqlite database. The CAS on delegation completion and the append-only
// annotation insert are the two durable invariants AC1/AC2 stand on.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func researchDelegationFixture(id string) types.TaskResearchDelegation {
	return types.TaskResearchDelegation{
		TenantID: 1, ID: id, SessionID: "s1", ParentRunID: "r1",
		Objective: "survey retrieval baselines", SourcesJSON: `["kb-1","kb-2"]`,
		Status: types.TaskResearchAssigned, CreatedBy: "u1",
	}
}

func TestTaskResearchStoreDelegationRoundTrip(t *testing.T) {
	db := openTaskGrantDB(t)
	store := repository.NewTaskResearchStore(db)
	ctx := context.Background()

	d1 := researchDelegationFixture("d1")
	require.NoError(t, store.CreateDelegation(ctx, &d1))
	// B5-F74: GORM writes the persisted timestamps back through the pointer.
	require.False(t, d1.CreatedAt.IsZero(), "落库时间戳必须回写到调用方实体")

	got, err := store.GetDelegation(ctx, 1, "d1")
	require.NoError(t, err)
	require.Equal(t, "survey retrieval baselines", got.Objective)
	require.Equal(t, []string{"kb-1", "kb-2"}, got.Sources())
	require.Equal(t, types.TaskResearchAssigned, got.Status)
	require.WithinDuration(t, time.Now().UTC(), got.CreatedAt, 5*time.Second)

	// Cross-tenant read is one uniform miss: the probe learns nothing.
	_, err = store.GetDelegation(ctx, 2, "d1")
	require.ErrorIs(t, err, types.ErrTaskResearchNotFound)

	list, err := store.ListDelegationsBySession(ctx, 1, "s1", 50, "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	list, err = store.ListDelegationsBySession(ctx, 2, "s1", 50, "")
	require.NoError(t, err)
	require.Empty(t, list, "跨租户列表必须为空（AC1 探测面）")
}

func TestTaskResearchStoreCompleteDelegationCASMovesAssignedOnly(t *testing.T) {
	db := openTaskGrantDB(t)
	store := repository.NewTaskResearchStore(db)
	ctx := context.Background()
	d1 := researchDelegationFixture("d1")
	require.NoError(t, store.CreateDelegation(ctx, &d1))

	done, err := store.CompleteDelegation(ctx, 1, "d1", "3 findings, all cited")
	require.NoError(t, err)
	require.Equal(t, types.TaskResearchCompleted, done.Status)
	require.Equal(t, "3 findings, all cited", done.Summary)

	// Replaying the completion is a state conflict, not a second write: the
	// recorded summary is immutable once completed (durable checkpoint).
	_, err = store.CompleteDelegation(ctx, 1, "d1", "retry summary")
	require.ErrorIs(t, err, types.ErrTaskResearchState)

	got, err := store.GetDelegation(ctx, 1, "d1")
	require.NoError(t, err)
	require.Equal(t, "3 findings, all cited", got.Summary, "重放完成不得改写既有摘要")

	// Completing an unknown delegation is the same uniform miss.
	_, err = store.CompleteDelegation(ctx, 1, "d-missing", "x")
	require.ErrorIs(t, err, types.ErrTaskResearchNotFound)
}

func TestTaskAnnotationStoreAppendOnlyAndMaterialIndex(t *testing.T) {
	db := openTaskGrantDB(t)
	store := repository.NewTaskAnnotationStore(db)
	ctx := context.Background()

	base := types.TaskArtifactAnnotation{
		TenantID: 1, ID: "an1", SessionID: "s1", RunID: "r1",
		MaterialID: "m1:0", BaseVersion: "9a2f1c3d4e5f6a7b",
		Body: "结论第三段缺引用", AuthorID: "u3",
	}
	require.NoError(t, store.CreateAnnotation(ctx, &base))

	list, err := store.ListAnnotationsBySession(ctx, 1, "s1", 50, "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "9a2f1c3d4e5f6a7b", list[0].BaseVersion)

	byMaterial, err := store.ListAnnotationsForMaterial(ctx, 1, "s1", "m1:0")
	require.NoError(t, err)
	require.Len(t, byMaterial, 1)
	byMaterial, err = store.ListAnnotationsForMaterial(ctx, 1, "s1", "m2:0")
	require.NoError(t, err)
	require.Empty(t, byMaterial)

	_, err = store.ListAnnotationsBySession(ctx, 2, "s1", 50, "")
	require.NoError(t, err)
	// 跨租户返回空集（同委派列表口径），绝不泄漏其它租户批注。
	other, err := store.ListAnnotationsBySession(ctx, 2, "s1", 50, "")
	require.NoError(t, err)
	require.Empty(t, other)

	// Validation fails closed before any durable write.
	bad := base
	bad.ID = "an2"
	bad.Body = ""
	require.ErrorIs(t, store.CreateAnnotation(ctx, &bad), types.ErrTaskAnnotationInvalid)
	bad.ID = "an3"
	bad.Body = "x"
	bad.BaseVersion = ""
	require.ErrorIs(t, store.CreateAnnotation(ctx, &bad), types.ErrTaskAnnotationInvalid)
	bad.ID = "an4"
	bad.BaseVersion = string(make([]rune, 129))
	for i := range bad.BaseVersion {
		bad.BaseVersion = bad.BaseVersion[:i] + "a" + bad.BaseVersion[i+1:]
	}
	require.ErrorIs(t, store.CreateAnnotation(ctx, &bad), types.ErrTaskAnnotationInvalid)
}

// B5-F67: the keyset pagination must resume strictly after the cursor row on
// the real database (row-value comparison over (created_at, id)).
func TestTaskAnnotationStoreKeysetPagination(t *testing.T) {
	db := openTaskGrantDB(t)
	store := repository.NewTaskAnnotationStore(db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		row := types.TaskArtifactAnnotation{
			TenantID: 1, ID: fmt.Sprintf("an-%d", i), SessionID: "s1", RunID: "r1",
			MaterialID: "m1:0", BaseVersion: "v", Body: "b", AuthorID: "u3",
			CreatedAt: time.Unix(int64(1_700_000_000+i), 0).UTC(),
		}
		require.NoError(t, store.CreateAnnotation(ctx, &row))
	}

	page1, err := store.ListAnnotationsBySession(ctx, 1, "s1", 2, "")
	require.NoError(t, err)
	require.Len(t, page1, 2)
	require.Equal(t, "an-0", page1[0].ID)
	require.Equal(t, "an-1", page1[1].ID)

	page2, err := store.ListAnnotationsBySession(ctx, 1, "s1", 2, page1[1].ID)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	require.Equal(t, "an-2", page2[0].ID)

	// An unknown cursor is the uniform empty page, never an error or a leak.
	pageMiss, err := store.ListAnnotationsBySession(ctx, 1, "s1", 2, "an-missing")
	require.NoError(t, err)
	require.Empty(t, pageMiss)
}
