#!/bin/bash
# Issue #83 复验轮环境探测（run_in_background 差异定位，无密钥输出，只打印长度）。
set -euo pipefail
echo "shell=$0 bash=$BASH_VERSION"
echo "step1-source"
source ~/.zcode/issue72-stripe.env
echo "step2-keylen=${#STRIPE_SECRET_KEY}"
read -r L < <(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys limit 1" | tr -d "[:space:]")
echo "step3-lagolen=${#L}"
echo "step4-cwd=$(pwd)"
which go docker python3
