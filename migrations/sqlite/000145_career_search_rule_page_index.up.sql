CREATE INDEX idx_career_search_rule_page ON career_search_rules (tenant_id, user_id, updated_at DESC, id ASC);
