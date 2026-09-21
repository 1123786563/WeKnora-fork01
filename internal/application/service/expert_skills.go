package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/experts"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/skills"
	"github.com/Tencent/WeKnora/internal/types"
)

// ExpertSkillInstaller is the narrow install surface the expert skill
// resolver needs from the tenant skill service: start one async install and
// hand back the job ID for progress polling. Declared here (rather than
// depending on the full TenantSkillService) so the resolver cannot reach the
// progress, removal or config surfaces. *TenantSkillService satisfies it.
type ExpertSkillInstaller interface {
	InstallSkill(ctx context.Context, tenantID uint64, configID string, archive []byte) (string, error)
}

// bundledSkillResolver is the real ExpertSkillResolver behind expert
// instantiation: it pins each bundled skill under the install name its
// SKILL.md declares and, when the skill is not yet usable on the target
// sandbox config, bridges into the tenant install flow by zipping the
// bundled directory and starting an async install.
//
// Degrade, never fail: every per-skill problem (unreadable directory,
// oversize tree, install refusal, lookup error) leaves the skill selected
// under its best-known name and pending, because a missing optional skill
// must not abort the agent the user asked for.
type bundledSkillResolver struct {
	installer ExpertSkillInstaller
	lister    installedSkillLister
}

// NewBundledSkillResolver builds the production ExpertSkillResolver. The
// lister is the same installed-skill enumeration the session path uses
// (repository.TenantSkillRepository satisfies it); the installer is the
// tenant skill service.
func NewBundledSkillResolver(installer ExpertSkillInstaller, lister installedSkillLister) ExpertSkillResolver {
	return bundledSkillResolver{installer: installer, lister: lister}
}

// ResolveExpertSkills implements ExpertSkillResolver.
//
// "Installed" follows what the runtime actually serves: the tenant skill
// rows of the given sandbox config, kept when Enabled and Ready — the same
// row filter effectiveTenantSkills applies when a chat turn decides which
// skills a session can invoke. The config's image-activity check is
// deliberately not duplicated here: an inactive image is a config-level
// condition (typically rotated credentials) that re-installing the same
// archive cannot fix, and the install path skips a ready archive without
// booting a sandbox, so treating such rows as installed costs nothing.
//
// Pending carries manifest slugs (the stable reference an operator can map
// back to the template); Selected carries resolved install names (what the
// runtime matches AllowedSkills against). A skill whose install was started
// is pending too: it is not usable until the async job lands.
func (r bundledSkillResolver) ResolveExpertSkills(
	ctx context.Context, tenantID uint64, sandboxConfigID string, e *experts.Expert,
) (ExpertSkillResolution, error) {
	var res ExpertSkillResolution
	if e == nil || len(e.Manifest.Skills) == 0 {
		return res, nil
	}

	installed := r.installedSkillNames(ctx, tenantID, sandboxConfigID)

	for _, slug := range e.Manifest.Skills {
		dir := e.SkillDirs[slug]
		name := bundledSkillInstallName(slug, dir)
		res.Selected = append(res.Selected, name)
		if installed[name] {
			continue
		}
		res.Pending = append(res.Pending, slug)

		if sandboxConfigID == "" || r.installer == nil {
			// No sandbox to grow (or no installer wired): the skill stays
			// selected-and-pending; the agent records it for later.
			continue
		}
		if dir == "" {
			logger.Warnf(ctx,
				"[expert] skill %q of expert %q has no bundled directory; leaving it pending",
				slug, e.Manifest.ID)
			continue
		}
		archive, err := experts.ZipSkillDir(dir)
		if err != nil {
			logger.Warnf(ctx, "[expert] zip bundled skill %q of expert %q failed: %v",
				slug, e.Manifest.ID, err)
			continue
		}
		installID, err := r.installer.InstallSkill(ctx, tenantID, sandboxConfigID, archive)
		if err != nil {
			logger.Warnf(ctx, "[expert] install bundled skill %q of expert %q on config %s failed: %v",
				slug, e.Manifest.ID, sandboxConfigID, err)
			continue
		}
		res.InstallIDs = append(res.InstallIDs, installID)
	}
	return res, nil
}

// installedSkillNames answers which install names this tenant can already
// invoke on the given config. Any lookup failure degrades to "nothing
// installed" (logged): the worst case is one redundant install attempt,
// which the install path skips for a ready archive without billing a
// sandbox, and a failed instantiation is a strictly worse outcome.
func (r bundledSkillResolver) installedSkillNames(
	ctx context.Context, tenantID uint64, configID string,
) map[string]bool {
	names := make(map[string]bool)
	if configID == "" || r.lister == nil {
		return names
	}
	rows, err := r.lister.ListSkillsByConfig(ctx, tenantID, configID)
	if err != nil {
		logger.Warnf(ctx, "[expert] list installed skills of config %s failed: %v", configID, err)
		return names
	}
	for _, row := range rows {
		if row != nil && row.Enabled && row.Status == types.SkillStatusReady {
			names[row.Name] = true
		}
	}
	return names
}

// bundledSkillInstallName resolves the install name a bundled skill
// directory declares: the SKILL.md frontmatter name after the skills
// package's slug/slugify fallbacks (display titles become installable ids).
// A missing or unparseable SKILL.md falls back to the manifest slug — the
// name the agent pins, and the one an operator can still install by hand.
func bundledSkillInstallName(slug, dir string) string {
	if dir == "" {
		return slug
	}
	content, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return slug
	}
	skill, err := skills.ParseSkillFile(string(content))
	if err != nil || skill == nil {
		return slug
	}
	if name := strings.TrimSpace(skill.Name); name != "" {
		return name
	}
	return slug
}
