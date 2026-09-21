package datasource

import (
	"slices"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

// TestConnectorMetadataRegistryMatchesImplementedSet pins the registry to
// exactly the connector set the container registers (container.go
// initConnectorRegistry). A metadata entry without an implementation is a
// ghost: it surfaces fake options in the edit-mode type dropdown.
func TestConnectorMetadataRegistryMatchesImplementedSet(t *testing.T) {
	want := []string{
		types.ConnectorTypeFeishu, types.ConnectorTypeLark,
		types.ConnectorTypeFeishuDrive, types.ConnectorTypeLarkDrive,
		types.ConnectorTypeNotion, types.ConnectorTypeYuque,
		types.ConnectorTypeIMA, types.ConnectorTypeRSS,
		types.ConnectorTypeGitLab, types.ConnectorTypeConfluence,
		types.ConnectorTypeDingTalk,
	}
	got := make([]string, 0, len(ConnectorMetadataRegistry))
	for source := range ConnectorMetadataRegistry {
		got = append(got, source)
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		t.Fatalf("registry/implementation drift:\n want %v\n got  %v", want, got)
	}
}
