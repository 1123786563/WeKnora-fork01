package types

import "time"

// Agent Fork lineage and license registry value types (T32, Ticket #62).
//
// spec §9: 只修改本地资源和策略属于 Mapping；修改 portable core 时创建
// Agent Fork。Fork 保留来源、许可和修改说明；许可证允许时才能作为新的
// Release 重新提交审核。The lineage columns live on the submission/release
// rows; the license registry is deployment-scoped (a self-hosted
// deployment is one governance domain — see plan-t62 差异记录 #4).

// AgentForkDerivation is the adoption lineage of a variant-published local
// agent: the latest AgentAdoptionVariantEntity whose LocalAgentID matches,
// its Adoption's Listing and the pinned source Release (local row first,
// the #60 introduction ledger as fallback). Release is nil only when the
// pinned release cannot be resolved — callers must fail closed.
type AgentForkDerivation struct {
	Variant   AgentAdoptionVariantEntity
	ListingID string
	Release   *AgentReleaseEntity
}

// AgentLicenseEntity is one deployment license term. ID is the license_id
// slug ReleaseMetadata declares; AllowsRedistribution is the flag the
// re-submission gate consults. Re-registering (upsert) updates the flags —
// that is how a license flip propagates to later submissions.
type AgentLicenseEntity struct {
	ID                   string `gorm:"type:varchar(64);primaryKey"`
	Name                 string `gorm:"type:varchar(255);not null;default:''"`
	AllowsRedistribution bool   `gorm:"not null;default:false"`
	CreatedBy            string `gorm:"type:varchar(255);not null;default:''"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (AgentLicenseEntity) TableName() string { return "agent_licenses" }
