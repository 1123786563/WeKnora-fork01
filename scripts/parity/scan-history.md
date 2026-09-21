# Parity 扫描历史（git 管理镜像；auto-scan/INDEX.md 屡遭并行清理，此为权威副本）

- 2026-09-20 基线（R492，11 页）：8.72 → 修复轮①-⑤：4.49（chat 17.3→5.19、kb-wiki 10.5→4.39、kb-demo→4.86、kb-faq→2.90、kb-list→2.89 等，内容区普遍 <1%）
- 2026-09-21 清单扩展：11→60 条目（29 设置 section、KB tab、免登录、重定向、dev/markdown、12 交互态），新口径基线 15.08
- 2026-09-21 修复轮⑥：login 17.9→4.03、register 31.1→4.04、dev-markdown 71.8→11.26、ix-faq-retrieval 69.7→5.72、integration-cli 43.4→37.7、chrome→30.1、claw→36.7；交互 warn 10→0
- 2026-09-21T06-20-18（修复轮⑦起点）：60 条目，平均 14.68；Top 欠账：ix-kb-doc-detail 53.3、ix-kb-settings 48.3、ix-kb-list-create 45.4、integration-cli 37.7、claw 36.7、weknoracloud 35.8、chrome 30.1、runtime-queues 29.9、members 21.8、ix-faq-import 18.0、parser 17.8、system-global/redirect-system 17.0、ollama 15.8、envvars 15.5
- 豁免不可修：settings 套餐与额度/会话偏好传导的结构偏移、侧栏技能市场传导偏移、TDesign 字形/字体 AA 亚像素残差
| 2026-09-21T07-22-29 | 60 | 60 | 9.20 | 修复轮⑦（6 子代理）：dev-markdown 11.26→0.27（达标✓）、ix-kb-doc-detail 53.3→7.82、ix-kb-settings 48.3→27.8、ix-kb-list-create 45.4→23.0、runtime-queues 29.9→11.26、integration-cli 37.7→12.2、claw 36.7→14.7、chrome 30.1→12.7、weknoracloud 35.8→13.0、faq-import 18.0→9.69、chat 5.19→5.06、settings 全域普降（envvars/chathistory/ollama 等）|
| 2026-09-21T08-21-26 | 60 | 60 | 9.20→8.00 | 修复轮⑧（6 子代理）：system-global/redirect-system 16.7→11.6（SettingsPage 侧栏行高 29/32 双态+系统全局面板重排）、ix-kb-settings 27.8→16.6、ix-kb-list-create 23.0→19.3、integration-cli 12.2→8.13、claw 14.7→10.7、chrome 12.7→10.5、weknoracloud 13.0→11.2、platform-api-keys 13.1→11.2、faq-import 9.7→7.92、members 21.1→20.3、parser 16.8→13.4；chat 交互态 menu 7.0→6.05/mention 5.8→4.57；测试全域绿（chat+platform 522、faq+documents 376、settings 各面板等）|
| 2026-09-21T08-46-02 | 60 | 60 | 8.00 | 每日巡检：与上轮逐条持平（收敛态确认）；无恶化/异常。残差结构=豁免项传导（套餐块/技能市场）+TDesign 字形 AA 底噪，均为红线不可修项——已收敛，不派子代理 |