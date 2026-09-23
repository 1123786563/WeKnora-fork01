package career

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractResumeFieldsUsesOnlyExplicitLabelsAndKeepsEvidence(t *testing.T) {
	text := `Education: Example University, computer science
Experience: Acme, 2022-2024, backend engineer
Experience: Acme, 2023-2025, backend engineer
Project: Compiler optimization
Skill: Go, PostgreSQL
Achievement: Reduced p95 latency by 30%
Certificate: Cloud Architect`
	fields, missing, flags := ExtractResumeFields(text)
	require.Len(t, fields, 7)
	require.True(t, strings.HasPrefix(fields[0].Key, "education.item_"))
	require.Equal(t, "Education: Example University, computer science", fields[0].Evidence)
	require.Equal(t, "Experience: Acme, 2022-2024, backend engineer", fields[1].Evidence)
	require.NotEqual(t, fields[1].Key, fields[2].Key)
	require.Contains(t, missing, "education.graduation_year")
	require.Contains(t, flags, "multiple_experience_claims_require_review")
	for _, field := range fields {
		require.NotEmpty(t, field.Value)
		require.NotEmpty(t, field.Evidence)
	}
}

func TestExtractResumeFieldsSameCategoryItemsHaveOrderIndependentKeys(t *testing.T) {
	a := "Experience: Acme, 2022-2024, backend engineer\nExperience: Beta, 2020-2022, analyst"
	b := "Experience: Beta, 2020-2022, analyst\nExperience: Acme, 2022-2024, backend engineer"
	fieldsA, _, _ := ExtractResumeFields(a)
	fieldsB, _, _ := ExtractResumeFields(b)
	require.Len(t, fieldsA, 2)
	require.Len(t, fieldsB, 2)
	keys := map[string]bool{fieldsA[0].Key: true, fieldsA[1].Key: true}
	require.NotEqual(t, fieldsA[0].Key, fieldsA[1].Key)
	require.True(t, keys[fieldsB[0].Key])
	require.True(t, keys[fieldsB[1].Key])
}

func TestExtractResumeFieldsDoesNotInventUnlabeledOrUnquantifiedFacts(t *testing.T) {
	fields, missing, flags := ExtractResumeFields("A school near Beijing. Managed a large team.\nAchievement: improved customer satisfaction")
	require.Empty(t, fields)
	require.Contains(t, missing, "education")
	require.Contains(t, missing, "experience")
	require.Contains(t, missing, "achievement")
	require.Contains(t, flags, "achievement_without_explicit_number")
}
