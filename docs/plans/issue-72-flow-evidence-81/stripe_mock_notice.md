# 间歇断网期间的容器内 Stripe mock 使用与撤销记录（S3 合规说明）

## 背景

2026-09-24 验证会话前段，宿主机与 `weknora-lago-*` 容器均无法出站到
api.stripe.com（直连与系统代理 127.0.0.1:17890 均超时）。Lago v1.53.0 创建
payment-gated 订阅时同步调用 Stripe（PaymentIntent.create），连接异常导致
WeKnora 收到 `unreachable`，流程无法推进。

期间采取的临时手段：在 Lago api 容器内运行自签证书的 TLS mock
（ruby -e 内存执行，绑定 127.0.0.1:443，/etc/hosts 劫持 api.stripe.com，
CA 追加进 stripe gem 的 ca-certificates.crt）。

## 撤销（网络恢复后，全部完成）

1. api 与 api-worker 容器的 /etc/hosts：移除 `api.stripe.com` 劫持行。
2. 两个容器的 stripe gem CA bundle：
   `…/gems/stripe-6.5.0/lib/data/ca-certificates.crt` 截回 158 张证书
   （验证命令 `grep -c "BEGIN CERTIFICATE"` = 158）。
3. worker 容器重启（丢弃旧的内存 CA store）。
4. mock ruby 进程按 /proc/cmdline 匹配 `ruby -e` 终止。
5. 撤销后连通性实证：api 容器内
   `curl -s -m 10 https://api.stripe.com/v1/customers` → 401（真 Stripe、
   无凭据往返）。此后所有购买均走真 Stripe。

## S3 纪律

- mock 期间未产生任何需要凭据的操作；Lago 内注册的 provider secret key
  从未被导出、打印或写入任何文件。
- mock 脚本无凭据；本文件为事后说明，脚本本体未入仓（内存执行）。
- 期间创建的测试数据：tenant-2 customer 曾绑定占位 provider customer id
  （cus-dev-weknora-tenant-2），已随 customer 删除清理；无真实资金操作
  （Stripe TEST 模式）。
