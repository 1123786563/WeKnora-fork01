package wiki

import "github.com/Tencent/WeKnora/internal/modules/datasource/connector/feishu/core"

func txt(s string) *core.BlockText {
	return &core.BlockText{Elements: []core.TextElement{{TextRun: &core.TextRun{Content: s}}}}
}
