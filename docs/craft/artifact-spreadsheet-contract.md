# Craft 表格类型契约（XLSX）

任务：CFT-S04-T027 · 基线：`.worktrees/craft-cft`

## 交付物与生成链

- `report.xlsx` — 沙箱内 skill 构建工作簿（openpyxl 解析）；**LibreOffice headless 真实重算**后落盘的缓存值才算数
- readiness 规则：`SheetReady` 需要 Recalculated 事实——只写公式（无缓存值）不是就绪表
- manifest 门：recalc + preview + parse 三项全过才发布（admission 前置，失败零发布）

## 数值一致性

- 预览数值 = 存储的 XLSX 缓存值（`TestXLSXRecalculatedCacheRoundTrip300And350`：300/350 轮各自一致；日期/空单元格/千分位/守卫文本类型保真）
- 下载的字节与预览同源（不可变版本文件）

## 公式/外链/宏政策（机器执行）

- 禁止 token：`[` `]`（外部工作簿引用）、`|`（DDE）、`FILE://`/`HTTP://`/`HTTPS://`/`FTP://`
- 禁止函数（词边界 + 参数表）：WEBSERVICE/FILTERXML/RTD/HYPERLINK/EXEC/CALL/REGISTER/SEND.KEYS/DDE（.xlsx 不能带 XLM 宏，公式层面再拒——纵深防御）
- CSV 公式注入：以 `=`/`+`/`-`/`@` 开头的注入文本保持为**纯文本单元格**；空白绕过（`= WEBSERVICE(`）同拒

## 不变性 / 证据

v2 发布后 v1 的 xlsx SHA 不变（不可变发布链 + T011/T019）。真实引擎证据：`testdata/d02_libreoffice_recalc.xlsx`（LibreOffice 重算产物）；本轮命名化 pin `TestCraftSpreadsheetRecalcPins` 三断言 1:1。浏览器级（生成→修改→查看→历史下载）在 T028 执行。

## 不承诺

在线编辑、宏启用、跨工作簿实时链接不在范围。
