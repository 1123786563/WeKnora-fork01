package file

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfiguredProviderCareerObjectKeysAreStableAndTenantScoped(t *testing.T) {
	name := "career_source_request-1.pdf"
	require.True(t, isCareerStableName(name))
	require.Equal(t, "career_source_request-1.pdf", careerObjectName(name))
	// Provider key layouts preserve their configured prefixes and tenant scope.
	for _, prefix := range []string{"root", "root/"} {
		firstAttempt := careerExportObjectKey(prefix, 7, name)
		retry := careerExportObjectKey(prefix, 7, name)
		require.Equal(t, "root/7/exports/"+name, firstAttempt)
		require.Equal(t, firstAttempt, retry)
	}
	require.Equal(t, "7/exports/"+name, careerExportObjectKey("", 7, name))
	require.False(t, isCareerStableName("resume.pdf"))
}
