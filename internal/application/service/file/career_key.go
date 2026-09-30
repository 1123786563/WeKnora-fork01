package file

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

type fileBytesPutHook func(context.Context, string, string, []byte) error

// isCareerStableName identifies caller-selected names whose identity must
// survive a retry after the provider accepted a write but lost its response.
func isCareerStableName(name string) bool {
	return strings.HasPrefix(name, "career_export_") || strings.HasPrefix(name, "career_source_")
}

func careerObjectName(name string) string {
	return filepath.ToSlash(name)
}

func careerExportObjectKey(prefix string, tenantID uint64, name string) string {
	key := fmt.Sprintf("%s/%d/exports/%s", strings.Trim(prefix, "/"), tenantID, careerObjectName(name))
	return strings.TrimPrefix(key, "/")
}
