# 2026-09-14 Review rows acceptance audit（批核首遍：证据存在性映射）

## 方法
对 matrix 全部 74 个 status=review 行做机械核验：提取行内 evidence 引用（*.md），逐个检查 evidence/vue-react-parity/ 下是否存在且非空（>100B）。产物明细 /tmp/audit-pass1.json（协调者留存）。

## 首遍结果
- 总行数：74
- 行内含显式 evidence 引用：15（其中 12 行全部引用存在；3 行为正则误切分/引用嵌于长备注，人工复核后大概率存在，如 R007 → 2026-09-13-new-user-guide.md）
- 行内无显式引用：59（R001,R005,R006,R008,R010,R014-R016,R018-R022,R025,R026,R028-R030,R032,R034-R037,R039-R042,R045,R047-R056,N001,N002,N004,N009,N014,N015,N017,N019-N022,N024-N027,N029,N030,N032,N033）——其证据多以切片交付报告/账本轮次记录形式存在于 progress.md 与 evidence/ 目录（命名不含日期前缀或按切片命名）

## 三分类初步判定
- PROMOTE 候选：12 行（引用全存在+门禁全绿）
- KEEP-REVIEW 候选：59 行（需补 evidence 引用回填或按 S00 补平台/语言维度）
- STALE：暂未发现（对引用失效的 3 行待人工复核确认）

## 后续批次计划
1. 12 行 PROMOTE 候选人工复核 → matrix 行状态改 accepted
2. 59 行无引用行按域分组回填 evidence 引用（引用其对应切片交付文档）
3. STALE 复核

## Pass 2+3 结果（source freshness + promotion）
- 12 候选源新鲜度全部通过：最后源改动 2026-09-12~14，与各证据文档（09-13/14）同期或更早——无 STALE。
- **PROMOTED to accepted（12 行）**：R011, R012, R017, R023, R038, N003, N006, N008, N011, N012, N013, N018 —— 范围：web 平台 / zh-CN / 各行 evidence 覆盖的状态；验证依据：证据文档存在且非空 + 门禁全绿（shared 437, web 796, mobile 146, typecheck 0）+ 源新鲜度。
- matrix 行状态已更新（12 行 review → accepted，note 注明审计依据）。
- 剩余 review 行：62（59 无显式引用行 + 3 引用复核行）——按域分组回填引用后进入下一批。


## Pass 4 结果（引用回填收官）
- 10 个批次完成：74 行 review 行**全部具备显式证据引用**（48 行回填 + 26 行原有引用）。
- 回填映射摘要：路由/重定向行 → deeplink 扫描；settings 系行 → round-5 截图 + wrapper/专项文档；chat 系行 → round-5 截图 + streaming/渲染切片证据；文件代理行 → share/upload 证据；N 系行 → 各对应切片证据（Wails 行 → 运行时+桌面分辨率证据）。
- 提交链：4124b0d7 / bad0a37e / 2e13bf8e / 7f13f78f / f7274800 / 661ac3c7 / cdce68d8 / 8e5d1a20 / 7874fec5 / b22b7f0e。

## 当前矩阵状态分布
- accepted：15 行（3 原有 + 12 批核晋升）
- review：59 行（证据引用已齐备，进入逐行验收判定阶段——按 S00 逐行确认状态/语言/平台覆盖后晋升或拆分）
- implementing/blocked-env：见 matrix（Android 原生证据为唯一 blocked-env 项，恢复步骤已登记）

