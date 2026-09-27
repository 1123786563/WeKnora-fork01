-- T23 (#53): explicit per-actor grants on SPACE connections. CONTEXT.md
-- 代码平台连接: 个人连接只能由其所有者使用；空间连接按仓库、成员角色和
-- 操作策略授权。Personal connections never carry a row here — the A02
-- authorizer only consults this table for Kind='space' connections.
CREATE TABLE app_space_connection_grants (
    tenant_id     BIGINT       NOT NULL,
    connection_id VARCHAR(64)  NOT NULL,
    actor_id      VARCHAR(512) NOT NULL,
    granted_by    VARCHAR(512) NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, connection_id, actor_id)
);
