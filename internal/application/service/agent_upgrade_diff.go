package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
)

// ErrAgentUpgradeInvalidInput marks a diff request whose releases are
// missing (nil). 临时哨兵：Task 4 落盘 agent_upgrade.go 哨兵组时移入该文件并从此处删除
// （plan-t61.md Task 3 Step 4 授权的过渡形态）。
var ErrAgentUpgradeInvalidInput = errors.New("invalid agent upgrade proposal request")

// diffUpgradeBundles computes the four review dimensions (behavior /
// dependencies / security / license — Issue #61 AC2) between the currently
// accepted Release and the proposed Release. Deterministic: fixed field
// order, sorted list joins, dependency keys sorted; identical releases
// produce byte-identical (all-empty) diffs. Malformed stored JSON fails
// closed with an error — the caller must not materialize a half-built
// proposal.
func diffUpgradeBundles(from, to *types.AgentReleaseEntity) (types.AgentUpgradeDiff, error) {
	diff := types.AgentUpgradeDiff{
		Behavior:     []types.UpgradeFieldChange{},
		Dependencies: []types.UpgradeDependencyChange{},
		Security:     []types.UpgradeFieldChange{},
		License:      []types.UpgradeLicenseChange{},
	}
	if from == nil || to == nil {
		return diff, ErrAgentUpgradeInvalidInput
	}
	fromManifest, fromPayload, fromLock, err := decodeUpgradeBundleParts(from)
	if err != nil {
		return diff, err
	}
	toManifest, toPayload, toLock, err := decodeUpgradeBundleParts(to)
	if err != nil {
		return diff, err
	}

	// 行为维：便携 payload 字段级差异（列表字段排序拼接后比较）。
	appendFieldChange(&diff.Behavior, "agent_mode", fromPayload.AgentMode, toPayload.AgentMode)
	appendFieldChange(&diff.Behavior, "system_prompt", fromPayload.SystemPrompt, toPayload.SystemPrompt)
	appendFieldChange(&diff.Behavior, "persona_mbti", fromPayload.PersonaMBTI, toPayload.PersonaMBTI)
	appendFieldChange(&diff.Behavior, "persona_style", fromPayload.PersonaStyle, toPayload.PersonaStyle)
	appendListChange(&diff.Behavior, "allowed_tools", fromPayload.AllowedTools, toPayload.AllowedTools)
	appendListChange(&diff.Behavior, "skills", fromPayload.Skills, toPayload.Skills)
	appendListChange(&diff.Behavior, "subagents", fromPayload.Subagents, toPayload.Subagents)
	appendListChange(&diff.Behavior, "starter_prompts", fromPayload.StarterPrompts, toPayload.StarterPrompts)

	// 安全维：Manifest 声明面的扩大（数据类别 / 外部副作用 / 能力需求）。
	appendListChange(&diff.Security, "capability_requirements", fromManifest.CapabilityRequirements, toManifest.CapabilityRequirements)
	appendListChange(&diff.Security, "data_categories", fromManifest.DataCategories, toManifest.DataCategories)
	appendListChange(&diff.Security, "external_side_effects", fromManifest.ExternalSideEffects, toManifest.ExternalSideEffects)

	// 依赖维：DependencyLock 按 (type, id) 对齐；键序确定。
	fromDeps := make(map[string]types.AgentReleaseDependency, len(fromLock.Dependencies))
	toDeps := make(map[string]types.AgentReleaseDependency, len(toLock.Dependencies))
	keys := make([]string, 0, len(fromLock.Dependencies)+len(toLock.Dependencies))
	for _, dep := range fromLock.Dependencies {
		key := dep.Type + "\x00" + dep.ID
		fromDeps[key] = dep
		keys = append(keys, key)
	}
	for _, dep := range toLock.Dependencies {
		key := dep.Type + "\x00" + dep.ID
		toDeps[key] = dep
		if _, ok := fromDeps[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		fromDep, hadFrom := fromDeps[key]
		toDep, hasTo := toDeps[key]
		switch {
		case !hadFrom:
			diff.Dependencies = append(diff.Dependencies, types.UpgradeDependencyChange{
				Type: toDep.Type, ID: toDep.ID, Change: "added", ToVersion: toDep.Version,
			})
			diff.License = append(diff.License, types.UpgradeLicenseChange{
				Scope: "dependency", ID: toDep.ID, From: "", To: toDep.LicenseID,
			})
		case !hasTo:
			diff.Dependencies = append(diff.Dependencies, types.UpgradeDependencyChange{
				Type: fromDep.Type, ID: fromDep.ID, Change: "removed", FromVersion: fromDep.Version,
			})
			diff.License = append(diff.License, types.UpgradeLicenseChange{
				Scope: "dependency", ID: fromDep.ID, From: fromDep.LicenseID, To: "",
			})
		default:
			if fromDep.Version != toDep.Version {
				diff.Dependencies = append(diff.Dependencies, types.UpgradeDependencyChange{
					Type: fromDep.Type, ID: fromDep.ID, Change: "version_changed",
					FromVersion: fromDep.Version, ToVersion: toDep.Version,
				})
			}
			if fromDep.Digest != toDep.Digest {
				diff.Dependencies = append(diff.Dependencies, types.UpgradeDependencyChange{
					Type: fromDep.Type, ID: fromDep.ID, Change: "digest_changed",
					FromVersion: fromDep.Version, ToVersion: toDep.Version,
				})
			}
			if fromDep.LicenseID != toDep.LicenseID {
				diff.License = append(diff.License, types.UpgradeLicenseChange{
					Scope: "dependency", ID: fromDep.ID, From: fromDep.LicenseID, To: toDep.LicenseID,
				})
			}
		}
	}

	// 许可维收尾：Release 自身许可（依赖许可已按键序并入其前）。
	if fromManifest.LicenseID != toManifest.LicenseID {
		diff.License = append(diff.License, types.UpgradeLicenseChange{
			Scope: "release", ID: "license_id", From: fromManifest.LicenseID, To: toManifest.LicenseID,
		})
	}
	return diff, nil
}

// decodeUpgradeBundleParts decodes the three stored projections of one
// immutable release: ManifestJSON → AgentReleaseManifest, Bundle → payload
// envelope, DependencyLockJSON → DependencyLock.
func decodeUpgradeBundleParts(release *types.AgentReleaseEntity) (types.AgentReleaseManifest, types.AgentReleasePayload, types.DependencyLock, error) {
	var manifest types.AgentReleaseManifest
	if err := json.Unmarshal([]byte(release.ManifestJSON), &manifest); err != nil {
		return manifest, types.AgentReleasePayload{}, types.DependencyLock{}, fmt.Errorf("decode release %s manifest: %w", release.ID, err)
	}
	var envelope struct {
		Payload types.AgentReleasePayload `json:"payload"`
	}
	if err := json.Unmarshal(release.Bundle, &envelope); err != nil {
		return manifest, types.AgentReleasePayload{}, types.DependencyLock{}, fmt.Errorf("decode release %s bundle payload: %w", release.ID, err)
	}
	var lock types.DependencyLock
	if err := json.Unmarshal([]byte(release.DependencyLockJSON), &lock); err != nil {
		return manifest, types.AgentReleasePayload{}, types.DependencyLock{}, fmt.Errorf("decode release %s dependency lock: %w", release.ID, err)
	}
	return manifest, envelope.Payload, lock, nil
}

func sortedJoin(values []string) string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

func appendFieldChange(changes *[]types.UpgradeFieldChange, field, from, to string) {
	if from == to {
		return
	}
	*changes = append(*changes, types.UpgradeFieldChange{Field: field, From: from, To: to})
}

func appendListChange(changes *[]types.UpgradeFieldChange, field string, from, to []string) {
	appendFieldChange(changes, field, sortedJoin(from), sortedJoin(to))
}
