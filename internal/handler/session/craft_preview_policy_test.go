package session

// CFT-S03-T020: the preview refresh policy — an expired ticket's renewal
// re-authorizes THE SAME version. Even after v2 is published, refreshing a
// v1 ticket yields a v1 capability serving v1 bytes: renewal never creates
// a run, never rebuilds the artifact, and never silently switches to the
// latest version. (The per-assertion suites live in craft_preview_test.go:
// expiry 404 + re-issue, cross-tenant 404, traversal/encoding rejections,
// bound-version serving, unsupported kinds, unconfigured origin.)
import (
	"net/url"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

func TestCraftPreviewPolicyRefreshReauthorizesSameVersion(t *testing.T) {
	env := newPreviewEnv(t)
	v1 := env.publishPreviewVersion(t, "ws-pol", "run-1", [][2]string{{"index.html", "<h1>v1 monthly</h1>"}})
	// v2 lands AFTER the v1 ticket was minted.
	v2 := env.publishPreviewVersion(t, "ws-pol", "run-2", [][2]string{{"index.html", "<h1>v2 quarterly</h1>"}})
	require.NotEqual(t, v1.ID, v2.ID)

	ticket := env.issueTicket(t, v1.ID)
	parsed, err := url.Parse(ticket["url"].(string))
	require.NoError(t, err)

	// The ticket dies; the v1 refresh path mints a NEW ticket for v1 only.
	env.clock.advance(craft.PreviewTicketTTL + time.Second)
	require.Equal(t, 404, env.previewGet(parsed.Path).Code, "the expired ticket is dead")
	refreshed := env.issueTicket(t, v1.ID)
	require.Equal(t, v1.ID, refreshed["version_id"], "refresh re-authorizes the SAME version")

	capPath := env.redeem(t, refreshed["url"].(string))
	body := env.previewGet(capPath)
	require.Equal(t, 200, body.Code)
	require.Contains(t, body.Body.String(), "v1 monthly", "the capability serves v1 bytes — not the latest v2")
	require.NotContains(t, body.Body.String(), "v2 quarterly")

	// And v2 stays independently reachable through its own ticket.
	v2Ticket := env.issueTicket(t, v2.ID)
	require.Equal(t, v2.ID, v2Ticket["version_id"])
}
