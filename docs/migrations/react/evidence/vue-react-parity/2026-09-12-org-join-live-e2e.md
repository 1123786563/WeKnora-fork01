# Organizations join live E2E（2026-09-12，Round 25）

隔离后端实测：
- 创建组织（owner）→ invite_code 签发；直接 join（POST /organizations/join {invite_code}）成功（require_approval:false 组织）。
- 创建带 require_approval:true 的组织：**后端忽略了该字段**（返回的组织 require_approval 仍为 false），join 直接成功而非进入审批队列。即：审批门控分支当前在真实后端上不可达——join.ts 的 require_approval 分支保留（对齐 Vue 语义，将来后端支持即生效），但登记为后端行为待决项：审批门控组织当前无法通过 API 创建。
- 搜索接口 /organizations/search 未返回非 searchable 组织（行为待与 Vue 对齐确认）。
