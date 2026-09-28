package types

import "time"

// AgentEvaluationEntity is immutable structured evidence for one exact public release.
type AgentEvaluationEntity struct {
	ID               string    `gorm:"type:varchar(36);primaryKey"`
	ReleaseID        string    `gorm:"type:varchar(36);not null"`
	TestSetID        string    `gorm:"type:varchar(255);not null"`
	TestSetVersion   string    `gorm:"type:varchar(128);not null"`
	EnvironmentClass string    `gorm:"type:varchar(64);not null"`
	EvaluatorID      string    `gorm:"type:varchar(255);not null"`
	EvaluatedAt      time.Time `gorm:"not null"`
	ResultsJSON      string    `gorm:"type:text;not null"`
}

func (AgentEvaluationEntity) TableName() string { return "agent_release_evaluations" }
