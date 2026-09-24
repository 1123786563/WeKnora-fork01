-- T29 Tenant Adoption, Agent Variant and local capability mapping governance tables.
CREATE TABLE agent_adoptions (
 id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, listing_id VARCHAR(36) NOT NULL,
 accepted_release_id VARCHAR(36) NOT NULL, state VARCHAR(32) NOT NULL DEFAULT 'active',
 created_by VARCHAR(255) NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (listing_id, tenant_id) REFERENCES agent_marketplace_listings(id, tenant_id),
 FOREIGN KEY (accepted_release_id, tenant_id) REFERENCES agent_releases(id, tenant_id)
);
CREATE TABLE agent_adoption_variants (
 id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, adoption_id VARCHAR(36) NOT NULL, release_id VARCHAR(36) NOT NULL,
 name VARCHAR(255) NOT NULL, state VARCHAR(32) NOT NULL DEFAULT 'draft',
 local_agent_id VARCHAR(36), local_agent_version_id VARCHAR(36),
 created_by VARCHAR(255) NOT NULL DEFAULT '', tested_by VARCHAR(255) NOT NULL DEFAULT '', tested_at TIMESTAMPTZ,
 published_by VARCHAR(255) NOT NULL DEFAULT '', published_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (adoption_id, tenant_id) REFERENCES agent_adoptions(id, tenant_id),
 FOREIGN KEY (release_id, tenant_id) REFERENCES agent_releases(id, tenant_id)
);
CREATE TABLE agent_variant_capability_mappings (
 id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, variant_id VARCHAR(36) NOT NULL,
 capability VARCHAR(255) NOT NULL, model_id VARCHAR(255) NOT NULL DEFAULT '',
 knowledge_base_ids TEXT NOT NULL DEFAULT '[]', connection_ids TEXT NOT NULL DEFAULT '[]',
 updated_by VARCHAR(255) NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (variant_id, tenant_id) REFERENCES agent_adoption_variants(id, tenant_id)
);
CREATE INDEX idx_agent_adoption_variants_adoption ON agent_adoption_variants(tenant_id, adoption_id, created_at);
CREATE INDEX idx_agent_variant_capability_variant ON agent_variant_capability_mappings(tenant_id, variant_id);
CREATE UNIQUE INDEX uq_agent_adoptions_scope ON agent_adoptions(tenant_id, listing_id);
CREATE UNIQUE INDEX uq_agent_variant_capability ON agent_variant_capability_mappings(tenant_id, variant_id, capability);
