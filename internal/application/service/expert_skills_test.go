package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/experts"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// bundledSkillFixture writes a skill directory whose SKILL.md declares name
// (typically different from the manifest slug) and returns its path.
func bundledSkillFixture(t *testing.T, slug, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "---\nname: " + name + "\ndescription: bundled fixture skill\n---\n\nDo the thing.\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "helper.txt"), []byte("helper"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// skillExpert rewrites the shared fixture expert to bundle exactly the given
// slug→dir mapping.
func skillExpert(dirs map[string]string, slugs ...string) *experts.Expert {
	e := fixtureExpert()
	e.Manifest.Skills = slugs
	e.SkillDirs = dirs
	return e
}

type fakeExpertInstallCall struct {
	tenantID uint64
	configID string
	archive  []byte
}

type fakeExpertSkillInstaller struct {
	calls []fakeExpertInstallCall
	retID string
	err   error
}

func (f *fakeExpertSkillInstaller) InstallSkill(
	_ context.Context, tenantID uint64, configID string, archive []byte,
) (string, error) {
	f.calls = append(f.calls, fakeExpertInstallCall{tenantID: tenantID, configID: configID, archive: archive})
	if f.err != nil {
		return "", f.err
	}
	return f.retID, nil
}

type fakeExpertSkillLister struct {
	rows   []*types.TenantSkillEntity
	err    error
	called int
}

func (f *fakeExpertSkillLister) ListSkillsByConfig(
	context.Context, uint64, string,
) ([]*types.TenantSkillEntity, error) {
	f.called++
	return f.rows, f.err
}

func readySkillRow(name string, enabled bool) *types.TenantSkillEntity {
	return &types.TenantSkillEntity{Name: name, Enabled: enabled, Status: types.SkillStatusReady}
}

func TestResolveExpertSkillsInstalledOnly(t *testing.T) {
	dir := bundledSkillFixture(t, "stock-info", "stock-quote")
	e := skillExpert(map[string]string{"stock-info": dir}, "stock-info")
	installer := &fakeExpertSkillInstaller{retID: "install-1"}
	lister := &fakeExpertSkillLister{rows: []*types.TenantSkillEntity{readySkillRow("stock-quote", true)}}

	res, err := NewBundledSkillResolver(installer, lister).
		ResolveExpertSkills(context.Background(), 7, "cfg-1", e)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Selected, []string{"stock-quote"}) {
		t.Errorf("Selected = %v, want resolved frontmatter name", res.Selected)
	}
	if len(res.Pending) != 0 || len(res.InstallIDs) != 0 {
		t.Errorf("installed skill must be neither pending nor installing: %+v", res)
	}
	if len(installer.calls) != 0 {
		t.Errorf("installer called %d times for an installed skill, want 0", len(installer.calls))
	}
	if lister.called != 1 {
		t.Errorf("lister called %d times, want 1", lister.called)
	}
}

func TestResolveExpertSkillsInstallKickedOff(t *testing.T) {
	dir := bundledSkillFixture(t, "stock-info", "stock-quote")
	e := skillExpert(map[string]string{"stock-info": dir}, "stock-info")
	installer := &fakeExpertSkillInstaller{retID: "install-1"}
	lister := &fakeExpertSkillLister{}

	res, err := NewBundledSkillResolver(installer, lister).
		ResolveExpertSkills(context.Background(), 7, "cfg-1", e)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Selected, []string{"stock-quote"}) {
		t.Errorf("Selected = %v, want [stock-quote]", res.Selected)
	}
	if !reflect.DeepEqual(res.InstallIDs, []string{"install-1"}) {
		t.Errorf("InstallIDs = %v, want [install-1]", res.InstallIDs)
	}
	// The install is async: until it lands the skill is still not usable.
	if !reflect.DeepEqual(res.Pending, []string{"stock-info"}) {
		t.Errorf("Pending = %v, want the slug [stock-info]", res.Pending)
	}
	if len(installer.calls) != 1 {
		t.Fatalf("installer called %d times, want 1", len(installer.calls))
	}
	call := installer.calls[0]
	if call.tenantID != 7 || call.configID != "cfg-1" {
		t.Errorf("install call tenant=%d config=%q, want 7/cfg-1", call.tenantID, call.configID)
	}
	// The zipped directory must be a bundle the install path accepts.
	bundle, err := ParseSkillBundle(call.archive)
	if err != nil {
		t.Fatalf("zipped skill is not installable: %v", err)
	}
	if bundle.Name != "stock-quote" {
		t.Errorf("bundle name = %q, want stock-quote", bundle.Name)
	}
	if _, ok := bundle.Files["SKILL.md"]; !ok {
		t.Error("bundle is missing SKILL.md")
	}
	if _, ok := bundle.Files["helper.txt"]; !ok {
		t.Error("bundle is missing helper.txt")
	}
}

func TestResolveExpertSkillsNoSandbox(t *testing.T) {
	dir := bundledSkillFixture(t, "stock-info", "stock-quote")
	e := skillExpert(map[string]string{"stock-info": dir}, "stock-info")
	installer := &fakeExpertSkillInstaller{retID: "install-1"}
	lister := &fakeExpertSkillLister{}

	res, err := NewBundledSkillResolver(installer, lister).
		ResolveExpertSkills(context.Background(), 7, "", e)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Selected, []string{"stock-quote"}) {
		t.Errorf("Selected = %v, want [stock-quote]", res.Selected)
	}
	if !reflect.DeepEqual(res.Pending, []string{"stock-info"}) {
		t.Errorf("Pending = %v, want [stock-info]", res.Pending)
	}
	if len(res.InstallIDs) != 0 || len(installer.calls) != 0 {
		t.Errorf("no sandbox means no install: %+v calls=%d", res, len(installer.calls))
	}
}

func TestResolveExpertSkillsInstallerErrorDegrades(t *testing.T) {
	dir := bundledSkillFixture(t, "stock-info", "stock-quote")
	e := skillExpert(map[string]string{"stock-info": dir}, "stock-info")
	installer := &fakeExpertSkillInstaller{err: errors.New("sandbox unreachable")}
	lister := &fakeExpertSkillLister{}

	res, err := NewBundledSkillResolver(installer, lister).
		ResolveExpertSkills(context.Background(), 7, "cfg-1", e)
	if err != nil {
		t.Fatalf("installer error must never fail instantiation: %v", err)
	}
	if !reflect.DeepEqual(res.Selected, []string{"stock-quote"}) {
		t.Errorf("Selected = %v, want [stock-quote]", res.Selected)
	}
	if !reflect.DeepEqual(res.Pending, []string{"stock-info"}) {
		t.Errorf("Pending = %v, want [stock-info]", res.Pending)
	}
	if len(res.InstallIDs) != 0 {
		t.Errorf("InstallIDs = %v, want none", res.InstallIDs)
	}
}

func TestResolveExpertSkillsOnlyReadyEnabledRowsCount(t *testing.T) {
	dir := bundledSkillFixture(t, "stock-info", "stock-quote")
	e := skillExpert(map[string]string{"stock-info": dir}, "stock-info")
	installer := &fakeExpertSkillInstaller{retID: "install-1"}
	lister := &fakeExpertSkillLister{rows: []*types.TenantSkillEntity{
		readySkillRow("stock-quote", false), // disabled
		{Name: "stock-quote", Enabled: true, Status: types.SkillStatusInstalling},
	}}

	res, err := NewBundledSkillResolver(installer, lister).
		ResolveExpertSkills(context.Background(), 7, "cfg-1", e)
	if err != nil {
		t.Fatal(err)
	}
	if len(installer.calls) != 1 {
		t.Fatalf("non-ready/disabled rows must not count as installed; installer calls = %d", len(installer.calls))
	}
	if !reflect.DeepEqual(res.Selected, []string{"stock-quote"}) {
		t.Errorf("Selected = %v", res.Selected)
	}
}

func TestResolveExpertSkillsMissingDirFallsBackToSlug(t *testing.T) {
	e := skillExpert(nil, "ghost")
	installer := &fakeExpertSkillInstaller{retID: "install-1"}
	lister := &fakeExpertSkillLister{}

	res, err := NewBundledSkillResolver(installer, lister).
		ResolveExpertSkills(context.Background(), 7, "cfg-1", e)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Selected, []string{"ghost"}) {
		t.Errorf("Selected = %v, want slug fallback [ghost]", res.Selected)
	}
	if !reflect.DeepEqual(res.Pending, []string{"ghost"}) {
		t.Errorf("Pending = %v, want [ghost]", res.Pending)
	}
	if len(installer.calls) != 0 {
		t.Errorf("nothing to zip, installer must stay idle; calls = %d", len(installer.calls))
	}
}

func TestResolveExpertSkillsListerErrorDegrades(t *testing.T) {
	dir := bundledSkillFixture(t, "stock-info", "stock-quote")
	e := skillExpert(map[string]string{"stock-info": dir}, "stock-info")
	installer := &fakeExpertSkillInstaller{retID: "install-1"}
	lister := &fakeExpertSkillLister{err: errors.New("db down")}

	res, err := NewBundledSkillResolver(installer, lister).
		ResolveExpertSkills(context.Background(), 7, "cfg-1", e)
	if err != nil {
		t.Fatalf("lister error must not fail instantiation: %v", err)
	}
	// Degrade to "nothing installed": the install path skips a re-upload of a
	// ready archive, so the cost of the wrong guess is bounded.
	if len(installer.calls) != 1 {
		t.Errorf("installer calls = %d, want 1 (install retried)", len(installer.calls))
	}
	if !reflect.DeepEqual(res.Selected, []string{"stock-quote"}) {
		t.Errorf("Selected = %v", res.Selected)
	}
}

func TestResolveExpertSkillsNoSkills(t *testing.T) {
	e := skillExpert(nil)
	installer := &fakeExpertSkillInstaller{}
	lister := &fakeExpertSkillLister{}

	res, err := NewBundledSkillResolver(installer, lister).
		ResolveExpertSkills(context.Background(), 7, "cfg-1", e)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Selected) != 0 || len(res.Pending) != 0 || len(res.InstallIDs) != 0 {
		t.Errorf("skill-less expert must resolve empty, got %+v", res)
	}
	if lister.called != 0 {
		t.Errorf("lister called %d times, want 0", lister.called)
	}
}

// TestInstantiateUsesResolvedSkillNames proves the hand-off constraint end to
// end: the agent is persisted with the RESOLVED skill names (frontmatter), not
// the manifest slugs, and the result reports the install job.
func TestInstantiateUsesResolvedSkillNames(t *testing.T) {
	dir := bundledSkillFixture(t, "stock-info", "stock-quote")
	e := skillExpert(map[string]string{"stock-info": dir}, "stock-info")
	agents := &expertAgentsFake{}
	installer := &fakeExpertSkillInstaller{retID: "install-9"}
	lister := &fakeExpertSkillLister{}
	svc := NewExpertService(func() []*experts.Expert { return []*experts.Expert{e} },
		agents, NewBundledSkillResolver(installer, lister))

	res, err := svc.Instantiate(expertCtx("zh-CN"), 7, "stock-assistant", interfaces.InstantiateRequest{
		SandboxConfigID: "cfg-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(agents.created) != 1 {
		t.Fatalf("CreateAgent called %d times, want 1", len(agents.created))
	}
	sent := agents.created[0]
	if sent.Config.SkillsSelectionMode != "selected" {
		t.Errorf("SkillsSelectionMode = %q, want selected", sent.Config.SkillsSelectionMode)
	}
	if !reflect.DeepEqual(sent.Config.SelectedSkills, []string{"stock-quote"}) {
		t.Errorf("SelectedSkills = %v, want resolved name [stock-quote] (not the slug)", sent.Config.SelectedSkills)
	}
	if !reflect.DeepEqual(res.SkillInstallIDs, []string{"install-9"}) {
		t.Errorf("SkillInstallIDs = %v, want [install-9]", res.SkillInstallIDs)
	}
	if !reflect.DeepEqual(res.PendingSkills, []string{"stock-info"}) {
		t.Errorf("PendingSkills = %v, want slug [stock-info]", res.PendingSkills)
	}
	if len(installer.calls) != 1 || installer.calls[0].configID != "cfg-2" {
		t.Errorf("install calls = %+v, want one on cfg-2", installer.calls)
	}
}
