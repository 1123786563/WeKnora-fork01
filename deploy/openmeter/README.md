# OpenMeter Docker 集成环境

使用官方 OpenMeter `v1.0.0-beta.232` 发布镜像，不依赖本机 OpenMeter 源码或扩展版本。该版本是 2026-09-10 查询 GitHub `releases/latest` 得到的最新发布版，版本名称仍为 beta。

## 启动与停止

在本目录执行：

```sh
docker compose pull
docker compose up -d --wait --wait-timeout 180
docker compose ps
python3 smoke.py
```

API 地址为 `http://127.0.0.1:48888`。项目名固定为 `weknora-fork01-openmeter`，仅 API 端口映射到宿主机回环地址。依赖服务不发布宿主机端口；现有 6379 Redis 不受影响。

暂停服务并保留数据：

```sh
docker compose stop
```

移除容器并保留数据：

```sh
docker compose down
```

Kafka、ClickHouse、PostgreSQL 和 Redis 使用独立命名卷，Redis 开启 AOF。不要使用 `down -v`，除非确实要删除该环境的全部数据。

## 范围

包含 API、sink/balance/billing workers、notification service、jobs，以及 Kafka、ClickHouse、PostgreSQL、Redis、Svix。配置保留官方样例的请求数、请求耗时及 Token meters。

这是本地开发和集成环境。使用官方示例凭据与单实例依赖，没有公网 API 鉴权、TLS、备份或高可用配置；不应直接作为生产部署。后续 WeKnora 的 Customer 映射、套餐、权益、Usage Outbox、配额执行和 App/Connector 接入仍需单独实现。

`smoke.py` 创建唯一测试 subject，重复投递同一事件，再检查聚合值为 1。测试数据保留在本环境，用于检查实际事件链路和重复投递处理，不证明完整计费正确性。

## 官方来源

- [版本发布](https://github.com/openmeterio/openmeter/releases/tag/v1.0.0-beta.232)
- [该版本 quickstart](https://github.com/openmeterio/openmeter/tree/v1.0.0-beta.232/quickstart)
- [该版本依赖配置](https://github.com/openmeterio/openmeter/blob/v1.0.0-beta.232/docker-compose.base.yaml)

本目录在官方配置基础上固定 OpenMeter 版本、添加数据卷、移除 worker 宿主机端口，并调整重启策略。部署时解析的镜像信息记录在 `images.lock.json`（成功拉取后生成）。

## 本机验证记录（2026-09-10）

- Docker Compose 配置校验通过，11 个容器运行；10 个具有健康检查的服务均为 healthy，jobs 容器运行但没有单独的健康检查。
- 容器内版本：`v1.0.0-beta.232`，源码提交 `887e0cac903ccd06e74d61ed23c651651d10c7a9`。
- 官方 ARM64 OCI manifest：`sha256:0a59132264bc72094c1405ada110c6e810d28f7336bde22aa814409bd58dd6c1`，与 Docker 导入后的 manifest digest 一致。
- 冒烟测试 subject：`weknora-smoke-a3cfcd5f-fb58-4135-b3d2-85e7e8dfd7e4`；同一事件投递 2 次，聚合值为 1。
- 本机网络对 GHCR 拉取出现多次 TLS 超时，最终使用系统已有代理从官方 registry 续传全部镜像层，逐层验证 SHA-256，并保留官方 OCI manifest 导入 Docker。未使用本地旧 OpenMeter 版本，也未使用加速服务提供的镜像。

以上仅证明本地服务和计量冒烟链路可用，不代表 WeKnora 商业接入、真实支付、完整配额执行或生产高可用已经完成。
