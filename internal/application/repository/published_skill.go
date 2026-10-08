package repository

import (
	acrepo "github.com/Tencent/WeKnora/internal/agentcatalog/repository"
)

// Pass B (25c) transitional shim — the implementation moved to
// internal/agentcatalog/repository/published_skill.go. Consumers are
// switched to the module package by IB2, after which this file is deleted
// (12-commercial §5.4 / 25a §2.3 pattern). No business logic may be added
// here — remove_at: ib2.

// PublishedSkillRepository is an alias of the module interface definition.
type PublishedSkillRepository = acrepo.PublishedSkillRepository

// NewPublishedSkillRepository forwards to the module constructor.
var NewPublishedSkillRepository = acrepo.NewPublishedSkillRepository
