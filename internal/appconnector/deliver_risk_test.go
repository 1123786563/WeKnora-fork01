package appconnector

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// RiskDeliver 永远需要逐次人工审批：任何 write 预授权都不覆盖它
// （NeedsExplicitApproval 的 default 分支），这是「交付前必须审阅
// Diff/提交」的结构性保证。
func TestRiskDeliverAlwaysNeedsExplicitApproval(t *testing.T) {
	require.Equal(t, "deliver", RiskDeliver)
	require.True(t, NeedsExplicitApproval(RiskDeliver, false))
	require.True(t, NeedsExplicitApproval(RiskDeliver, true), "a general write grant must never cover delivery")
}
