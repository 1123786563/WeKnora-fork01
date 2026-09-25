Review complete: 1 finding(s) across 8 selected item(s).

─── internal/modules/craft/citation.go:210-216 ───
[security · medium] ValidateWebCitationView 对不可信（模型生成的）HTML 做纯文本级正则校验，作为标记绑定的唯一防线可被绕过：

1. `webCitationMarkerRe`（data-craft-citation="…"）会匹配 HTML 中任意位置的字符串，包括注释（`<!--
data-craft-citation="kc_…" -->`）、script 字符串或纯文本。因此 manifest 中声明的事实可以完全不实际渲染，仅靠一条隐藏注释即可通过 "declared
facts must be rendered" 检查。
2. `webCitationInferenceRe` 同样只按出现次数比较，一条注释即可凑足推断标记数量。
3. `webCitationMixedAttrRe` 用 `[^>]*` 连接两类标记，遇到属性值中含 `>` 即截断漏检，且完全不匹配单引号形式
`data-craft-inference='true'`，同一元素混合标记的检查可被绕过。

组合效果：一条推断内容可携带某个已声明事实的 data-craft-citation
标记被呈现为"来源事实"（该真实事实不必渲染，其标记由隐藏注释满足；推断标记计数由另一处注释满足），恰好同时绕过本函数注释中声称强制的两条规则（"declared facts must be
rendered"、"an inference element must never carry a source citation marker"）。上游 build.py 的 denylist
只拦截外链/脚本标签，不约束标记语义，故此处是唯一防线。

建议：用结构化 HTML 解析（如 golang.org/x/net/html）将标记匹配限定到元素属性、排除注释与 script/style 文本后再做绑定校验；或至少在校验前剥离 HTML 注释与
script/style 块，并让 mixed-attr 检查基于解析后的元素属性集合而非正则。


