package container

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// TestArtifactVersionDownloadHandlerWired pins the W26 production assembly:
// the exact function the dig container Invokes must install a non-nil
// versioned download handler for routes_chat.go to mount. The service
// dependencies are zero-stubs — construction and registration are what this
// test locks; request behavior is covered by the handler package tests.
func TestArtifactVersionDownloadHandlerWired(t *testing.T) {
	registerArtifactVersionHTTPHandlers(
		struct{ interfaces.SessionService }{},
		struct{ interfaces.TenantService }{},
		struct{ interfaces.FileService }{},
		struct{ interfaces.StorageBackendResolver }{},
		repository.NewArtifactVersionStore(nil),
	)
	t.Cleanup(func() { session.RegisterArtifactVersionDownloadHandler(nil) })
	if session.RegisteredArtifactVersionDownloadHandler() == nil {
		t.Fatal("registerArtifactVersionHTTPHandlers must install the versioned download handler for route mounting")
	}
}
