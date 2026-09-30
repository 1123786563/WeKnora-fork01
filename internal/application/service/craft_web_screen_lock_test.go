package service

// The pinned screen-asset digest must stay derived-from (not parallel-to)
// docker/craft/web/toolchain.lock.json: upgrading the toolchain without
// updating the constant would reject every legitimate web capture, and
// "updating" the constant from staged content would void the pin. This
// drift test pins the FIRST failure mode loudly.
import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCraftScreenPinnedAssetDigestMatchesToolchainLock(t *testing.T) {
	lockPath := filepath.Join("..", "..", "..", "docker", "craft", "web", "toolchain.lock.json")
	raw, err := os.ReadFile(lockPath)
	require.NoError(t, err, "toolchain.lock.json must exist beside the pinned digest")
	var lock struct {
		Dependencies []struct {
			Name   string `json:"name"`
			SHA256 string `json:"sha256"`
		} `json:"dependencies"`
	}
	require.NoError(t, json.Unmarshal(raw, &lock))
	entryName, entrySHA := "", ""
	for _, dep := range lock.Dependencies {
		if dep.Name == "craft-web.js" {
			entryName, entrySHA = dep.Name, dep.SHA256
		}
	}
	require.Equal(t, "craft-web.js", entryName, "craft-web.js pin present in the lock")
	require.NotEmpty(t, entrySHA)
	require.Equal(t, entrySHA, craftScreenPinnedAssetDigest,
		"the pinned digest drifted from toolchain.lock.json — update the constant ONLY from the lock")
}
