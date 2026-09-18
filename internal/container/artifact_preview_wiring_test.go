package container

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// TestArtifactPreviewHandlerWired pins the W27 production assembly: the exact
// function the dig container Invokes must install a non-nil isolated preview
// handler for routes_chat.go / router.go to mount. The service dependencies
// are zero-stubs — construction and registration are what this test locks;
// request behavior is covered by the handler package tests (W26 wiring-test
// precedent).
func TestArtifactPreviewHandlerWired(t *testing.T) {
	registerArtifactPreviewHTTPHandlers(
		struct{ interfaces.SessionService }{},
		struct{ interfaces.TenantService }{},
		struct{ interfaces.FileService }{},
		struct {
			interfaces.StorageBackendResolver
		}{},
		repository.NewArtifactVersionStore(nil),
	)
	t.Cleanup(func() { handler.RegisterArtifactPreviewHandler(nil) })
	if handler.RegisteredArtifactPreviewHandler() == nil {
		t.Fatal("registerArtifactPreviewHTTPHandlers must install the isolated preview handler for route mounting")
	}
}
