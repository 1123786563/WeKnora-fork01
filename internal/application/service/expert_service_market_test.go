package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/agent/experts"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// installedExpertFake is one market-installed expert as the ExpertSource
// seam would hand it over (the concrete source scans the tenant's
// expert-market directory).
func installedExpertFake(id string) *experts.Expert {
	return &experts.Expert{
		Manifest: experts.ExpertManifest{
			ID:          id,
			Label:       experts.LocaleText{"zh": "市场专家", "en": "Market Expert"},
			Description: experts.LocaleText{"zh": "来自市场", "en": "From the market"},
			PromptFiles: []string{"SOUL.md"},
			Skills:      []string{"pdf-extract"},
			AgentConfig: experts.ExpertAgentConfig{AgentMode: types.AgentModeSmartReasoning},
		},
		PersonaFiles: map[string][]byte{"SOUL.md": []byte("market soul")},
	}
}

// fakeExpertSource records which tenants asked and answers from a fixed map.
type fakeExpertSource struct {
	byTenant map[uint64][]*experts.Expert
	asked    []uint64
}

func (f *fakeExpertSource) InstalledExperts(_ context.Context, tenantID uint64) []*experts.Expert {
	f.asked = append(f.asked, tenantID)
	return f.byTenant[tenantID]
}

func tenantCtx(tenantID uint64) context.Context {
	return context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
}

func TestExpertServiceMergesInstalledExperts(t *testing.T) {
	builtin := fixtureExpert()
	installed := installedExpertFake("skillhub-skillset-pdf-tools")
	src := &fakeExpertSource{byTenant: map[uint64][]*experts.Expert{7: {installed}}}
	svc := NewExpertServiceWithSource(func() []*experts.Expert { return []*experts.Expert{builtin} },
		&expertAgentsFake{}, nil, src)

	listed, err := svc.ListExperts(tenantCtx(7))
	require.NoError(t, err)
	require.Len(t, listed, 2)
	require.Equal(t, builtin, listed[0], "builtin experts keep precedence in the merge order")
	require.Equal(t, installed, listed[1])
	require.Equal(t, []uint64{7}, src.asked)

	got, err := svc.GetExpert(tenantCtx(7), "skillhub-skillset-pdf-tools")
	require.NoError(t, err)
	require.Equal(t, installed, got)
}

func TestExpertServiceMergeKeepsBuiltinPrecedence(t *testing.T) {
	builtin := fixtureExpert() // id "stock-assistant"
	shadow := installedExpertFake("stock-assistant")
	src := &fakeExpertSource{byTenant: map[uint64][]*experts.Expert{7: {shadow}}}
	svc := NewExpertServiceWithSource(func() []*experts.Expert { return []*experts.Expert{builtin} },
		&expertAgentsFake{}, nil, src)

	listed, err := svc.ListExperts(tenantCtx(7))
	require.NoError(t, err)
	require.Len(t, listed, 1, "an installed expert cannot shadow a builtin of the same ID")
	require.Equal(t, builtin, listed[0])

	got, err := svc.GetExpert(tenantCtx(7), "stock-assistant")
	require.NoError(t, err)
	require.Equal(t, builtin, got, "the builtin wins the lookup too")
}

func TestExpertServiceWithoutTenantContextServesBuiltinOnly(t *testing.T) {
	src := &fakeExpertSource{byTenant: map[uint64][]*experts.Expert{7: {installedExpertFake("skillhub-skillset-pdf-tools")}}}
	svc := NewExpertServiceWithSource(func() []*experts.Expert { return nil },
		&expertAgentsFake{}, nil, src)

	listed, err := svc.ListExperts(context.Background())
	require.NoError(t, err)
	require.Empty(t, listed)
	require.Empty(t, src.asked, "no tenant in ctx: the source is never consulted")

	_, err = svc.GetExpert(context.Background(), "skillhub-skillset-pdf-tools")
	require.ErrorIs(t, err, ErrExpertNotFound)
}

func TestInstantiateFromInstalledExpertStampsMarketProvenance(t *testing.T) {
	installed := installedExpertFake("skillhub-skillset-pdf-tools")
	src := &fakeExpertSource{byTenant: map[uint64][]*experts.Expert{7: {installed}}}
	agents := &expertAgentsFake{}
	svc := NewExpertServiceWithSource(func() []*experts.Expert { return nil }, agents, nil, src)

	res, err := svc.Instantiate(tenantCtx(7), 7, "skillhub-skillset-pdf-tools", interfaces.InstantiateRequest{})
	require.NoError(t, err)
	require.Equal(t, "agent-1", res.Agent.ID)
	require.NotNil(t, agents.created[0].Config.ExpertSource)
	require.Equal(t, "skillhub", agents.created[0].Config.ExpertSource.Source)
	require.Equal(t, "pdf-tools", agents.created[0].Config.ExpertSource.Slug,
		"the skillset slug rides along for same-ID disambiguation")
}

func TestInstantiateBuiltinExpertStillStampsBuiltin(t *testing.T) {
	builtin := fixtureExpert()
	src := &fakeExpertSource{byTenant: map[uint64][]*experts.Expert{7: {installedExpertFake("skillhub-skillset-pdf-tools")}}}
	agents := &expertAgentsFake{}
	svc := NewExpertServiceWithSource(func() []*experts.Expert { return []*experts.Expert{builtin} }, agents, nil, src)

	_, err := svc.Instantiate(tenantCtx(7), 7, "stock-assistant", interfaces.InstantiateRequest{})
	require.NoError(t, err)
	require.Equal(t, "builtin", agents.created[0].Config.ExpertSource.Source)
	require.Equal(t, "", agents.created[0].Config.ExpertSource.Slug)
}

func TestInstantiateUsesExplicitTenantIDNotContext(t *testing.T) {
	installed := installedExpertFake("skillhub-skillset-pdf-tools")
	src := &fakeExpertSource{byTenant: map[uint64][]*experts.Expert{7: {installed}}}
	agents := &expertAgentsFake{}
	svc := NewExpertServiceWithSource(func() []*experts.Expert { return nil }, agents, nil, src)

	// expertCtx carries no tenant: Instantiate must still resolve the
	// installed expert through its EXPLICIT tenantID parameter.
	res, err := svc.Instantiate(expertCtx("zh-CN"), 7, "skillhub-skillset-pdf-tools", interfaces.InstantiateRequest{})
	require.NoError(t, err)
	require.Equal(t, "agent-1", res.Agent.ID)
	require.Equal(t, []uint64{7}, src.asked, "the explicit tenantID drives the installed lookup")
}

// marketSourceOverDir proves the production source wiring: a materialized
// expert written under a market root is visible to ExpertService through
// NewMarketExpertSource.
func TestMarketExpertSourceScansRoot(t *testing.T) {
	// Reuse the experts package writer through its public surface: build a
	// minimal installed tree by hand (the full materialize round-trip lives
	// in the experts package tests).
	root := t.TempDir()
	dir := filepath.Join(root, "7", "pdf-tools")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "skills", "pdf-extract"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(
		"id: skillhub-skillset-pdf-tools\nlabel:\n  zh: 市场专家\n  en: Market Expert\nprompt_files:\n- SOUL.md\nskills:\n- pdf-extract\nagent_config:\n  agent_mode: smart-reasoning\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("market soul"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skills", "pdf-extract", "SKILL.md"), []byte("# pdf-extract"), 0o644))

	src := NewMarketExpertSource(root)
	got := src.InstalledExperts(context.Background(), 7)
	require.Len(t, got, 1)
	require.Equal(t, "skillhub-skillset-pdf-tools", got[0].Manifest.ID)
	require.Empty(t, src.InstalledExperts(context.Background(), 8))

	svc := NewExpertServiceWithSource(func() []*experts.Expert { return nil }, &expertAgentsFake{}, nil, src)
	listed, err := svc.ListExperts(tenantCtx(7))
	require.NoError(t, err)
	require.Len(t, listed, 1)
}
