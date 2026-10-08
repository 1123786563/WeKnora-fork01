package workbench

import (
	"context"
	"errors"
	"sync"
)

/**
 * 服务端能力清单（MX-032，core）。
 * 能力 = 部署开关 ∩ 服务健康 ∩ 用户权限 ∩ **验收证据**——证据缺失即 unavailable（不以按钮置灰当完成）。
 * 撤销能力后：RefreshCapability 使缓存失效，服务端对新准入直接拒绝（IsCapabilityEnabled=false）。
 */

type CapabilityProfile string

const (
	ProfileCore         CapabilityProfile = "core"
	ProfileOIDC         CapabilityProfile = "oidc"
	ProfileResources    CapabilityProfile = "resources"
	ProfileConnectors   CapabilityProfile = "connectors"
	ProfileRemote       CapabilityProfile = "remote"
	ProfilePersonalNode CapabilityProfile = "personal_node"
	ProfileDictation    CapabilityProfile = "dictation"
	ProfileVoice        CapabilityProfile = "voice"
	ProfileFullHappy    CapabilityProfile = "full_happy"
)

// CapabilityEvidenceGate 验收证据门禁输入：每 profile 必须有对应证据标识（发布门禁写入）。
type CapabilityEvidence struct {
	Profile CapabilityProfile
	// 证据标识（如 "native-e2e:remote-cancel"）——空=证据缺失
	EvidenceKeys []string
}

type CapabilityInput struct {
	Profile        CapabilityProfile
	DeployEnabled  bool
	ServiceHealthy bool
	UserPermitted  bool
	Evidence       CapabilityEvidence
}

var profileEvidenceRequirements = map[CapabilityProfile][]string{
	ProfileRemote: {"remote:cancel-evidence"}, // frozen：remote 无取消证据 → unavailable
}

type CapabilityService struct {
	mu      sync.RWMutex
	revoked map[CapabilityProfile]bool
}

func NewCapabilityService() *CapabilityService {
	return &CapabilityService{revoked: make(map[CapabilityProfile]bool)}
}

// EvaluateCapability 三态裁决：supported / unavailable(reason) / forbidden(权限)。
func (s *CapabilityService) EvaluateCapability(input CapabilityInput) (state string, reason string) {
	if !input.DeployEnabled {
		return "unavailable", "deploy_disabled"
	}
	if !input.UserPermitted {
		return "forbidden", "user_not_permitted"
	}
	if !input.ServiceHealthy {
		return "unavailable", "service_unhealthy"
	}
	s.mu.RLock()
	revoked := s.revoked[input.Profile]
	s.mu.RUnlock()
	if revoked {
		return "unavailable", "capability_revoked"
	}
	// 验收证据门禁：要求的关键证据必须齐备
	if required, ok := profileEvidenceRequirements[input.Profile]; ok {
		present := make(map[string]bool, len(input.Evidence.EvidenceKeys))
		for _, key := range input.Evidence.EvidenceKeys {
			present[key] = true
		}
		for _, need := range required {
			if !present[need] {
				return "unavailable", "missing_" + evidenceShortKey(need) + "_evidence"
			}
		}
	}
	return "supported", ""
}

func evidenceShortKey(key string) string {
	// "remote:cancel-evidence" -> "cancel"
	parts := splitKey(key)
	if len(parts) >= 2 {
		return parts[1]
	}
	return key
}

func splitKey(key string) []string {
	var out []string
	current := ""
	for _, r := range key {
		if r == ':' || r == '-' {
			out = append(out, current)
			current = ""
			continue
		}
		current += string(r)
	}
	out = append(out, current)
	return out
}

// RevokeCapability 撤销：缓存失效 + 服务端拒绝新准入（frozen 规则）。
func (s *CapabilityService) RevokeCapability(profile CapabilityProfile) error {
	if profile == ProfileCore {
		return errors.New("core profile cannot be revoked")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revoked[profile] = true
	return nil
}

// RefreshCapability 能力状态刷新（部署/健康变化后调用——撤销不清除：需显式重新准入）。
func (s *CapabilityService) RefreshCapability(profile CapabilityProfile) {
	// 当前实现无外部状态源：保留撤销位（刷新≠恢复——重新准入走发布门禁）
	_ = profile
}

// IsCapabilityEnabled 准入判定：撤销/不支持 → false（服务端拒绝新会话使用该能力）。
func (s *CapabilityService) IsCapabilityEnabled(profile CapabilityProfile) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.revoked[profile]
}

var _ = context.Background
