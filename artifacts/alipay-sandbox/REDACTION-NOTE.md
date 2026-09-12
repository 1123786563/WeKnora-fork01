# 证据脱敏说明（ALI-05）

本目录曾提交过一次沙箱联验的原始证据，包含：

- 商户/应用标识（`env.sh` 中的 APPID、卖家 PID）
- 付款用户信息（`paid_order.txt` 中的 buyer_user_id / buyer_logon_id / trade_no）
- 完整签名支付请求（`pay_url.txt`、`pay.html`、`qr.png`）

interface-verification.md §总则明确禁止在证据目录保存真实密钥、付款用户信息或完整凭据；
商户私钥文件也绝不应进入版本库。以上文件已从提交历史中整体移除，仅保留：

- `README.md`：联验准备步骤（不含任何真实标识）
- `wrap_keys.py`：PEM 包装工具
- `env.example.sh`：环境变量模板（占位符）
- `ali05-evidence.redacted.json`：脱敏后的行为证据（标识已打码）

密钥与本地凭据请保存在未跟踪的 `env.sh` / `*.pem`（已在 .gitignore 中忽略）。

**建议轮换**：该沙箱应用的 APPID/卖家 PID 与密钥对曾进入过一次本地提交，
按惯例应在支付宝开放平台重新生成沙箱密钥（自定义密钥 → 重新粘贴应用公钥），
旧密钥对作废后再继续联验。
