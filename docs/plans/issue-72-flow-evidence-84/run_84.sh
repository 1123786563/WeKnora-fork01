#!/usr/bin/env bash
# Issue #84 真实流程验证编排总入口（文档型：四幕的具体断言命令见 README.md
# 与各 leg*.txt 的实跑记录——它们由本会话的交互式真栈轮执行并留档；本脚本
# 固化起栈顺序与凭据纪律口径，供复验者重建同一环境）。
#
# 凭据纪律：全部密钥只经运行时 env 注入——
#   ~/.zcode/issue72-stripe.env            Stripe TEST key（source 注入）
#   ${TMPDIR}issue84-keys                   v2_gen_keys.sh 生成的一次性本地密钥
#   ${TMPDIR}issue84-secrets.env            0600 仓库外文件（Lago key + Stripe
#                                           key，行首 export——source 后须导出）
# Lago API key 只读取自 82r5 栈 DB（docker exec psql），webhook secret 经
# rails runner 模型层读取；两者只进 shell 变量，不落任何产出文件。
set -euo pipefail
cd "$(dirname "$0")/../../.."
export FLOW84_KEYS="${FLOW84_KEYS:-${TMPDIR}issue84-keys}"
export FLOW84_SECRETS_ENV="${FLOW84_SECRETS_ENV:-${TMPDIR}issue84-secrets.env}"
export FLOW84_PW="${FLOW84_PW:?set FLOW84_PW (operator-chosen, never committed)}"
export FLOW84_DB="${FLOW84_DB:-data/issue84-flow.db}"

echo "== 0. 端口预检（8096/5197/8298/8299 空闲；被占整体 +10 顺延并同步脚本 env）=="
for p in 8096 5197 8298 8299; do
  lsof -iTCP:$p -sTCP:LISTEN >/dev/null 2>&1 && { echo "port $p occupied"; exit 1; }
done

echo "== 1. 一次性本地密钥（复用 83 轮生成器）=="
bash docs/plans/issue-72-flow-evidence-83/v2_gen_keys.sh "$FLOW84_KEYS" >/dev/null

echo "== 2. stub 副本冒烟（Step 0 通过判据）=="
WECHAT_STUB_PORT=8298 WECHAT_STUB_APP_ID=wx84flow WECHAT_STUB_MCH_ID=1900000084 \
WECHAT_STUB_PLATFORM_SERIAL=PLAT-SERIAL-84 \
WECHAT_STUB_NOTIFY_URL=http://127.0.0.1:8096/api/v1/commercial/callbacks/wechat \
WECHAT_STUB_KEY_DIR="$FLOW84_KEYS" \
  python3 docs/plans/issue-72-flow-evidence-84/wechat_native_stub_anomaly.py --selftest

echo "== 3. 渠道 stub（:8298 微信 anomaly 副本 / :8299 支付宝）=="
bash docs/plans/issue-72-flow-evidence-84/up_stubs_84.sh

echo "== 4. WeKnora 后端（:8096）=="
nohup bash docs/plans/issue-72-flow-evidence-84/up_backend_84.sh \
  >> "${TMPDIR}issue84-backend.log" 2>&1 &
echo "backend pid $!（就绪等待 ~25s：curl http://127.0.0.1:8096/api/v1/auth/login 应答 4xx 即活）"

echo "== 5. 前端（:5197，代理 8096）=="
(cd apps/web && VITE_DEV_PROXY_TARGET=http://localhost:8096 \
  nohup pnpm dev --port 5197 --strictPort >> "${TMPDIR}issue84-frontend.log" 2>&1 &)

echo "== 6. 种子（pad + 主角 + pro 9900 发布 + Lago 断言）=="
sleep 25
bash docs/plans/issue-72-flow-evidence-84/seed_84.sh

echo "== 7. 四幕执行 =="
echo "见 README.md：第一幕（错金额/币种 stub 编排 + 幂等终态 + attention）、"
echo "第二幕（正常付款→settle→webhook→active + 重投×3 幂等 + Lago 基线）、"
echo "第三幕（换交易号第二笔→over_payment→drain→处置面）、第四幕（恢复路径实收比对）。"
echo "权威侧断言（Lago API key / webhook secret 的读取口径）见 README「凭据纪律」。"
