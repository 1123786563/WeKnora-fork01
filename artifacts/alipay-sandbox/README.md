# Alipay 沙箱联验准备（ALI-01/03/04/05）

## 一次性准备（约 10 分钟）
1. 打开 https://open.alipay.com → 用支付宝账号登录 → 控制台 → **沙箱环境**（默认可用，无需创建应用审核）
2. 记下两个值：
   - **APPID**（沙箱应用 ID，2021000 开头）
   - **卖家账号 PID / 商户账号**（沙箱页面上方"卖家"一栏，2088 开头）
3. 在沙箱页「自定义密钥」处：
   - 选择**公钥模式**（非证书模式）
   - 用「支付宝密钥生成器」或 openssl 生成 RSA2 (2048) 密钥对
   - 把**应用公钥**粘贴进沙箱页面 → 沙箱会显示**支付宝公钥**（注意：是"支付宝公钥"，不是你自己生成的公钥！）
4. 把两段裸 base64 分别存到本机纯文本文件，然后包成 PEM：
   - 支付宝公钥 →   python3 artifacts/alipay-sandbox/wrap_keys.py <支付宝公钥.b64> artifacts/alipay-sandbox/alipay_public.pem public
   - 你的应用私钥 → python3 artifacts/alipay-sandbox/wrap_keys.py <应用私钥.b64> artifacts/alipay-sandbox/merchant_private.pem private

## 运行自动部分（ALI-01/03/04：下单/查单/关单/未付退款/同键重试）
cd .worktrees/saas-billing-connectors
export ALIPAY_SANDBOX_APPID=2021000xxxxxxxxx
export ALIPAY_SANDBOX_SELLER_ID=2088xxxxxxxxxxxx
export ALIPAY_PUBLIC_KEY_PATH=$PWD/artifacts/alipay-sandbox/alipay_public.pem
export ALIPAY_MERCHANT_KEY_PATH=$PWD/artifacts/alipay-sandbox/merchant_private.pem
go test ./internal/payment -run TestAlipaySandbox -count=1 -v

## 运行真实付款链路（ALI-05，需要沙箱买家钱包扫码）
# 手机装"支付宝沙箱版钱包"（沙箱页面有下载入口），登录沙箱买家账号（沙箱页面提供）
export ALIPAY_SANDBOX_INTERACTIVE=1
go test ./internal/payment -run TestAlipaySandboxInteractivePaidRefundFlow -count=1 -v -timeout 10m
# 测试会打印二维码链接，用沙箱钱包扫它支付 0.01 元，随后自动全额退款并核对

## 安全
- 密钥文件只放本机 artifacts/alipay-sandbox/（未跟踪目录），永远不要把内容贴进对话
- 凭据只经环境变量注入测试进程；Go 配置只持路径引用
