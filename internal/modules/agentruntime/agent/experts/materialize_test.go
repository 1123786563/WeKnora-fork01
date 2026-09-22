package experts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/skills/skillhub"
	"github.com/Tencent/WeKnora/internal/types"
)

// fixtureSkillset mirrors a SkillHub skillset listing with full zh/en metadata.
func fixtureSkillset() skillhub.SkillsetSummary {
	return skillhub.SkillsetSummary{
		Slug:        "pdf-tools",
		Name:        "PDF 工具箱",
		Description: "一套覆盖拆分、合并与抽取的 PDF 处理技能",
		SkillSlugs:  []string{"pdf-extract", "pdf-merge"},
		Raw: map[string]any{
			"slug":          "pdf-tools",
			"displayName":   "PDF 工具箱",
			"displayNameEn": "PDF Toolkit",
			"summary":       "一套覆盖拆分、合并与抽取的 PDF 处理技能",
			"summaryEn":     "Split, merge and extract PDFs",
			"scene":         "tech",
		},
	}
}

// fixtureSkillFiles is one validated per-skill package: a root SKILL.md plus
// one nested resource, the shape ParsePackage emits.
func fixtureSkillFiles(name string) skillhub.ZipFiles {
	return skillhub.ZipFiles{
		{Name: "SKILL.md", Content: []byte("---\nname: " + name + "\n---\n# " + name)},
		{Name: "references/notes.md", Content: []byte("notes for " + name)},
	}
}

func fetchingSkills(files map[string]skillhub.ZipFiles, calls *[]string) func(string) (skillhub.ZipFiles, error) {
	return func(slug string) (skillhub.ZipFiles, error) {
		if calls != nil {
			*calls = append(*calls, slug)
		}
		f, ok := files[slug]
		if !ok {
			return nil, os.ErrNotExist
		}
		return f, nil
	}
}

func TestMaterializeSkillsetBuildsManifestAndPersona(t *testing.T) {
	me, err := MaterializeSkillset(fixtureSkillset(), fetchingSkills(map[string]skillhub.ZipFiles{
		"pdf-extract": fixtureSkillFiles("pdf-extract"),
		"pdf-merge":   fixtureSkillFiles("pdf-merge"),
	}, nil))
	require.NoError(t, err)

	m := me.Manifest
	require.Equal(t, "skillhub-skillset-pdf-tools", m.ID)
	require.Equal(t, expertsLocale{"zh": "PDF 工具箱", "en": "PDF Toolkit"}, m.Label)
	require.Equal(t, expertsLocale{
		"zh": "一套覆盖拆分、合并与抽取的 PDF 处理技能",
		"en": "Split, merge and extract PDFs",
	}, m.Description)
	require.Equal(t, []string{"SOUL.md"}, m.PromptFiles)
	require.Equal(t, []string{"pdf-extract", "pdf-merge"}, m.Skills)
	require.Equal(t, types.AgentModeSmartReasoning, m.AgentConfig.AgentMode)

	soul := string(me.PersonaFiles["SOUL.md"])
	require.Contains(t, soul, "# PDF 工具箱")
	require.Contains(t, soul, "来源于 SkillHub skillset `pdf-tools`")
	require.Contains(t, soul, "一套覆盖拆分、合并与抽取的 PDF 处理技能")
	require.Contains(t, soul, "- `pdf-extract`")
	require.Contains(t, soul, "- `pdf-merge`")

	require.Len(t, me.SkillFiles, 2)
	require.Equal(t, fixtureSkillFiles("pdf-extract"), me.SkillFiles["pdf-extract"])
	require.Equal(t, fixtureSkillFiles("pdf-merge"), me.SkillFiles["pdf-merge"])
}

// expertsLocale is a local alias so require.Equal diffs read as maps.
type expertsLocale = LocaleText

func TestMaterializeSkillsetPassthroughWithoutLocalizedMetadata(t *testing.T) {
	ss := fixtureSkillset()
	ss.Raw = map[string]any{"slug": "pdf-tools"} // no displayName(En)/summary(En)
	me, err := MaterializeSkillset(ss, fetchingSkills(map[string]skillhub.ZipFiles{
		"pdf-extract": fixtureSkillFiles("pdf-extract"),
		"pdf-merge":   fixtureSkillFiles("pdf-merge"),
	}, nil))
	require.NoError(t, err)
	require.Equal(t, expertsLocale{"zh": "PDF 工具箱", "en": "PDF 工具箱"}, me.Manifest.Label,
		"localized metadata missing: the typed summary fields pass through to both locales")
	require.Equal(t, expertsLocale{"zh": "一套覆盖拆分、合并与抽取的 PDF 处理技能", "en": "一套覆盖拆分、合并与抽取的 PDF 处理技能"}, me.Manifest.Description)
}

func TestMaterializeSkillsetDedupesSlugsAndFetchesOnce(t *testing.T) {
	var calls []string
	ss := fixtureSkillset()
	ss.SkillSlugs = []string{"pdf-extract", " pdf-extract ", "pdf-merge", "pdf-extract"}
	me, err := MaterializeSkillset(ss, fetchingSkills(map[string]skillhub.ZipFiles{
		"pdf-extract": fixtureSkillFiles("pdf-extract"),
		"pdf-merge":   fixtureSkillFiles("pdf-merge"),
	}, &calls))
	require.NoError(t, err)
	require.Equal(t, []string{"pdf-extract", "pdf-merge"}, me.Manifest.Skills)
	require.Equal(t, []string{"pdf-extract", "pdf-merge"}, calls, "one download per unique slug, in order")
}

func TestMaterializeSkillsetRejectsBadInput(t *testing.T) {
	fetch := fetchingSkills(map[string]skillhub.ZipFiles{"a": fixtureSkillFiles("a")}, nil)

	t.Run("empty slug", func(t *testing.T) {
		ss := fixtureSkillset()
		ss.Slug = ""
		_, err := MaterializeSkillset(ss, fetch)
		require.ErrorIs(t, err, skillhub.ErrInvalidSkillsetSlug)
	})

	t.Run("unsafe slug", func(t *testing.T) {
		ss := fixtureSkillset()
		ss.Slug = "../escape"
		_, err := MaterializeSkillset(ss, fetch)
		require.ErrorIs(t, err, skillhub.ErrInvalidSkillsetSlug)
	})

	t.Run("unsafe skill slug", func(t *testing.T) {
		ss := fixtureSkillset()
		ss.SkillSlugs = []string{"a/../b"}
		_, err := MaterializeSkillset(ss, fetch)
		require.Error(t, err)
		require.Contains(t, err.Error(), "a/../b")
	})

	t.Run("no skills", func(t *testing.T) {
		ss := fixtureSkillset()
		ss.SkillSlugs = nil
		_, err := MaterializeSkillset(ss, fetch)
		require.Error(t, err)
		require.Contains(t, err.Error(), "pdf-tools")
	})

	t.Run("fetch failure names the slug", func(t *testing.T) {
		ss := fixtureSkillset()
		ss.SkillSlugs = []string{"pdf-extract"}
		_, err := MaterializeSkillset(ss, func(string) (skillhub.ZipFiles, error) {
			return nil, os.ErrNotExist
		})
		require.ErrorIs(t, err, os.ErrNotExist)
		require.Contains(t, err.Error(), "pdf-extract")
	})

	t.Run("skill package without root SKILL.md", func(t *testing.T) {
		ss := fixtureSkillset()
		ss.SkillSlugs = []string{"pdf-extract"}
		_, err := MaterializeSkillset(ss, func(string) (skillhub.ZipFiles, error) {
			return skillhub.ZipFiles{{Name: "docs/only.md", Content: []byte("x")}}, nil
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "pdf-extract")
		require.Contains(t, err.Error(), "SKILL.md")
	})
}

func TestMaterializedExpertSnapshotSHA256(t *testing.T) {
	files := map[string]skillhub.ZipFiles{
		"pdf-extract": fixtureSkillFiles("pdf-extract"),
		"pdf-merge":   fixtureSkillFiles("pdf-merge"),
	}
	first, err := MaterializeSkillset(fixtureSkillset(), fetchingSkills(files, nil))
	require.NoError(t, err)
	second, err := MaterializeSkillset(fixtureSkillset(), fetchingSkills(files, nil))
	require.NoError(t, err)
	require.Equal(t, first.SnapshotSHA256(), second.SnapshotSHA256())
	require.NotEqual(t, "", first.SnapshotSHA256())

	changed := fixtureSkillset()
	changed.SkillSlugs = []string{"pdf-extract"}
	third, err := MaterializeSkillset(changed, fetchingSkills(files, nil))
	require.NoError(t, err)
	require.NotEqual(t, first.SnapshotSHA256(), third.SnapshotSHA256())
}

func TestWriteMaterializedExpertRoundTripsThroughScanExperts(t *testing.T) {
	me, err := MaterializeSkillset(fixtureSkillset(), fetchingSkills(map[string]skillhub.ZipFiles{
		"pdf-extract": fixtureSkillFiles("pdf-extract"),
		"pdf-merge":   fixtureSkillFiles("pdf-merge"),
	}, nil))
	require.NoError(t, err)

	root := t.TempDir()
	installDir := MarketInstallDir(root, 7, "pdf-tools")
	require.NoError(t, WriteMaterializedExpert(installDir, me))

	scanned, err := ScanExperts(filepath.Dir(installDir))
	require.NoError(t, err)
	require.Len(t, scanned, 1)

	e := scanned[0]
	require.Equal(t, "skillhub-skillset-pdf-tools", e.Manifest.ID)
	require.Equal(t, LocaleText{"zh": "PDF 工具箱", "en": "PDF Toolkit"}, e.Manifest.Label)
	require.Equal(t, []string{"pdf-extract", "pdf-merge"}, e.Manifest.Skills)
	require.Equal(t, types.AgentModeSmartReasoning, e.Manifest.AgentConfig.AgentMode)
	require.Equal(t, me.PersonaFiles["SOUL.md"], e.PersonaFiles["SOUL.md"])

	require.DirExists(t, filepath.Join(installDir, "skills", "pdf-extract"))
	skillDoc, err := os.ReadFile(filepath.Join(installDir, "skills", "pdf-extract", "SKILL.md"))
	require.NoError(t, err)
	require.Equal(t, fixtureSkillFiles("pdf-extract")[0].Content, skillDoc)
	nested, err := os.ReadFile(filepath.Join(installDir, "skills", "pdf-extract", "references", "notes.md"))
	require.NoError(t, err)
	require.Equal(t, "notes for pdf-extract", string(nested))
	require.Equal(t, filepath.Join(installDir, "skills", "pdf-extract"), e.SkillDirs["pdf-extract"])
}

func TestWriteMaterializedExpertReplacesPreviousInstall(t *testing.T) {
	both := map[string]skillhub.ZipFiles{
		"pdf-extract": fixtureSkillFiles("pdf-extract"),
		"pdf-merge":   fixtureSkillFiles("pdf-merge"),
	}
	first, err := MaterializeSkillset(fixtureSkillset(), fetchingSkills(both, nil))
	require.NoError(t, err)
	updated := fixtureSkillset()
	updated.Raw["summaryEn"] = "Refreshed"
	updated.SkillSlugs = []string{"pdf-extract"}
	second, err := MaterializeSkillset(updated, fetchingSkills(both, nil))
	require.NoError(t, err)

	root := t.TempDir()
	installDir := MarketInstallDir(root, 7, "pdf-tools")
	require.NoError(t, WriteMaterializedExpert(installDir, first))
	require.NoError(t, WriteMaterializedExpert(installDir, second))

	scanned, err := ScanExperts(filepath.Dir(installDir))
	require.NoError(t, err)
	require.Len(t, scanned, 1, "the second install replaces, never duplicates")
	require.Equal(t, LocaleText{"zh": "PDF 工具箱", "en": "PDF Toolkit"}, scanned[0].Manifest.Label)
	require.Equal(t, []string{"pdf-extract"}, scanned[0].Manifest.Skills)
	require.NoDirExists(t, filepath.Join(installDir, "skills", "pdf-merge"),
		"a skill dropped from the new snapshot is not left behind")

	entries, err := os.ReadDir(filepath.Dir(installDir))
	require.NoError(t, err)
	for _, entry := range entries {
		require.Equal(t, "pdf-tools", entry.Name(),
			"the tenant scan directory must hold only install directories, found %q", entry.Name())
	}
	// Scratch and backup live one level up, outside the scan root, and must
	// be cleaned up on every path.
	scratchEntries, err := os.ReadDir(filepath.Dir(filepath.Dir(installDir)))
	require.NoError(t, err)
	for _, entry := range scratchEntries {
		require.False(t, strings.HasPrefix(entry.Name(), ".expert-market-"),
			"scratch/backup must be cleaned up, found %q", entry.Name())
	}
}

func TestLeftoverScratchDoesNotBreakCatalogScan(t *testing.T) {
	me, err := MaterializeSkillset(fixtureSkillset(), fetchingSkills(map[string]skillhub.ZipFiles{
		"pdf-extract": fixtureSkillFiles("pdf-extract"),
		"pdf-merge":   fixtureSkillFiles("pdf-merge"),
	}, nil))
	require.NoError(t, err)

	root := t.TempDir()
	require.NoError(t, WriteMaterializedExpert(MarketInstallDir(root, 7, "pdf-tools"), me))

	// Simulate a crash between the tree write and the swap: scratch and
	// backup trees sit next to the tenant directory, each with a full
	// manifest.yaml. The tenant scan must stay clean.
	crashStaging := filepath.Join(root, ".expert-market-staging-123-1")
	crashBackup := filepath.Join(root, ".expert-market-backup-123-2")
	for _, dir := range []string{crashStaging, crashBackup} {
		require.NoError(t, WriteMaterializedExpert(dir, me))
	}

	installed := InstalledExperts(root, 7)
	require.Len(t, installed, 1)
	require.Equal(t, "skillhub-skillset-pdf-tools", installed[0].Manifest.ID)
}

func TestWriteMaterializedExpertRejectsEscapingMember(t *testing.T) {
	me, err := MaterializeSkillset(fixtureSkillset(), fetchingSkills(map[string]skillhub.ZipFiles{
		"pdf-extract": fixtureSkillFiles("pdf-extract"),
		"pdf-merge":   fixtureSkillFiles("pdf-merge"),
	}, nil))
	require.NoError(t, err)
	// Simulate a validated-looking package smuggling an absolute member name
	// (defense in depth: the writer never joins unclean paths).
	me.SkillFiles["pdf-extract"] = append(me.SkillFiles["pdf-extract"], skillhub.ZipFiles{
		{Name: "/etc/passwd", Content: []byte("x")},
	}...)
	err = WriteMaterializedExpert(MarketInstallDir(t.TempDir(), 7, "pdf-tools"), me)
	require.Error(t, err)
}

func TestInstalledExpertsScansTenantMarketDir(t *testing.T) {
	files := map[string]skillhub.ZipFiles{
		"pdf-extract": fixtureSkillFiles("pdf-extract"),
		"pdf-merge":   fixtureSkillFiles("pdf-merge"),
	}
	me, err := MaterializeSkillset(fixtureSkillset(), fetchingSkills(files, nil))
	require.NoError(t, err)
	root := t.TempDir()
	require.NoError(t, WriteMaterializedExpert(MarketInstallDir(root, 7, "pdf-tools"), me))

	installed := InstalledExperts(root, 7)
	require.Len(t, installed, 1)
	require.Equal(t, "skillhub-skillset-pdf-tools", installed[0].Manifest.ID)

	require.Empty(t, InstalledExperts(root, 8), "another tenant sees no installs")
	require.Empty(t, InstalledExperts(filepath.Join(root, "missing"), 7), "a missing root is the empty case, not an error")
}

func TestMarketDirLayout(t *testing.T) {
	require.Equal(t, filepath.Join("root", "7"), MarketTenantDir("root", 7))
	require.Equal(t, filepath.Join("root", "7", "pdf-tools"), MarketInstallDir("root", 7, "pdf-tools"))
}

func TestMarketDataRootFollowsLocalStorageBaseDir(t *testing.T) {
	t.Setenv("LOCAL_STORAGE_BASE_DIR", "/srv/weknora/data ")
	require.Equal(t, "/srv/weknora/data/expert-market", filepath.ToSlash(MarketDataRoot()),
		"a trailing space in the env value is trimmed")
	t.Setenv("LOCAL_STORAGE_BASE_DIR", "")
	require.Equal(t, "/data/files/expert-market", filepath.ToSlash(MarketDataRoot()),
		"unset base dir falls back to the server-local default /data/files")
}

func TestMarketExpertSlugFromID(t *testing.T) {
	slug, ok := MarketExpertSlugFromID("skillhub-skillset-pdf-tools")
	require.True(t, ok)
	require.Equal(t, "pdf-tools", slug)
	_, ok = MarketExpertSlugFromID("general-assistant")
	require.False(t, ok)
}
