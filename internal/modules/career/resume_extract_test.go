package career

import (
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
	require.Equal(t, "education.details", fields[0].Key)
	require.Equal(t, "Education: Example University, computer science", fields[0].Evidence)
	require.Equal(t, "Experience: Acme, 2022-2024, backend engineer", fields[1].Evidence)
	require.Equal(t, fields[1].Key, fields[2].Key)
	require.Contains(t, missing, "education.graduation_year")
	require.Contains(t, flags, "multiple_experience_claims_require_review")
	for _, field := range fields {
		require.NotEmpty(t, field.Value)
		require.NotEmpty(t, field.Evidence)
	}
}

func TestExtractResumeFieldsDoesNotInventUnlabeledOrUnquantifiedFacts(t *testing.T) {
	fields, missing, flags := ExtractResumeFields("A school near Beijing. Managed a large team.\nAchievement: improved customer satisfaction")
	require.Empty(t, fields)
	require.Contains(t, missing, "education")
	require.Contains(t, missing, "experience")
	require.Contains(t, missing, "achievement")
	require.Contains(t, flags, "achievement_without_explicit_number")
}
