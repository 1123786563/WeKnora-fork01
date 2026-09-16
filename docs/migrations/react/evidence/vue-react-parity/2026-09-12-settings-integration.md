# /platform/settings 必修 8 项集成复核（2026-09-12，Round 16）

- 子代理交付并经主代理门禁复核：test:shared 255/255、typecheck:shared/web 干净、build:web 成功、web 146/146。
- 集成接线（主代理）：scope-runtime 新增 role() 暴露当前租户角色；main.tsx 向 SettingsPage 传 role（owner/admin/viewer 映射，contributor→viewer 对齐 Vue SETTINGS_SECTION_MIN_ROLE 粒度）；补 vite alias '@weknora/domain/auth/password-policy'（否则构建失败）。
- live 验证（真实登录）：新 ModelManagement section 渲染显式 "尚未迁移（not yet ported）" 提示 + 真实 API value（[]，与 KB 未初始化横幅一致）；角色标签 MINIMUM ADMIN 可见（settings-models-section.png）。general 区视觉未变（本轮只做功能）。
- 提交：d1aa583（14 文件）+ 面板补交（gitignore web/ 规则第三次吞新文件——已在流程中注意每次 git ls-files 核验）。

## 仍开放（settings 维持 review）
- 抽屉形态/分组导航/图标（视觉形态迭代）；旧面板（Resource/Ollama/Cloud/memory/tenant/profile/页面框架）英文文案清扫；settings.notYetPorted 等缺失键补齐；GeneralSettings 本地偏好功能化；tenant 删除危险区。
