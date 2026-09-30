package types

// Agent Upgrade Proposal portable diff value types (T31, Ticket #61;
// spec §9, Issue #61 AC2).
//
// diffUpgradeBundles（internal/application/service）把两份不可变 Release 的
// 可审阅差异归入四个维度，逐字对应 Issue #61 AC2「升级差异覆盖行为、依赖、
// 安全和许可」：
//   - 行为 Behavior：便携 payload 字段级变化（模式/提示词/工具/技能/子代理/
//     开场提示），列表字段排序后拼接比较；
//   - 依赖 Dependencies：DependencyLock 按 (type, id) 对齐的增/删/版本变化/
//     摘要变化；
//   - 安全 Security：Manifest 声明面的扩大——能力需求、数据类别、外部副作用
//     （扫描类安全状态属 #64，不在本维度）；
//   - 许可 License：Release 许可与依赖许可的变化。
//
// 所有切片在生成时保持确定性顺序（固定字段序 + 排序键），同一对 Release
// 永远产出字节相同的差异。

// UpgradeFieldChange is one scalar or sorted-list field difference.
type UpgradeFieldChange struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// UpgradeDependencyChange is one DependencyLock difference keyed by
// (type, id). Change ∈ added | removed | version_changed | digest_changed.
type UpgradeDependencyChange struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Change      string `json:"change"`
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
}

// UpgradeLicenseChange is one license difference. Scope ∈ release |
// dependency; added/removed dependencies surface their license here with an
// empty From/To respectively.
type UpgradeLicenseChange struct {
	Scope string `json:"scope"`
	ID    string `json:"id"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// AgentUpgradeDiff is the four-dimension reviewable difference between the
// currently accepted Release and the proposed Release.
type AgentUpgradeDiff struct {
	Behavior     []UpgradeFieldChange      `json:"behavior"`
	Dependencies []UpgradeDependencyChange `json:"dependencies"`
	Security     []UpgradeFieldChange      `json:"security"`
	License      []UpgradeLicenseChange    `json:"license"`
}
