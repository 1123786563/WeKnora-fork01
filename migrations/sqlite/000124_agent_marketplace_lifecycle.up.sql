-- T33 Variant 退役 / Adoption 终止 / Listing 下架 / Release 弃用（#63, spec §9-§10）：
-- 生命周期只加列不改内容——Release 的 bundle/manifest/lock 恒不可变，退出不删行。
-- SQLite twin of versioned migration 000203.
ALTER TABLE agent_marketplace_listings ADD COLUMN unlisted_at DATETIME;
ALTER TABLE agent_marketplace_listings ADD COLUMN unlisted_by VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN deprecated_at DATETIME;
ALTER TABLE agent_releases ADD COLUMN deprecated_by VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN successor_release_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_adoptions ADD COLUMN ended_at DATETIME;
ALTER TABLE agent_adoptions ADD COLUMN ended_by VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE agent_adoption_variants ADD COLUMN retired_at DATETIME;
ALTER TABLE agent_adoption_variants ADD COLUMN retired_by VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE agent_adoptions ADD COLUMN end_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_adoption_variants ADD COLUMN retirement_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_marketplace_listings ADD COLUMN unlist_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN deprecation_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN replacement_release_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE public_marketplace_listings ADD COLUMN unlisted_by VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE public_marketplace_listings ADD COLUMN unlisted_at DATETIME;
ALTER TABLE public_marketplace_listings ADD COLUMN unlist_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE public_agent_releases ADD COLUMN deprecated_by VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE public_agent_releases ADD COLUMN deprecated_at DATETIME;
ALTER TABLE public_agent_releases ADD COLUMN deprecation_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE public_agent_releases ADD COLUMN replacement_release_id VARCHAR(36) NOT NULL DEFAULT '';
