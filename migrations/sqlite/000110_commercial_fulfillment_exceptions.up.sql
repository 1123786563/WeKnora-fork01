CREATE TABLE commercial_fulfillment_exceptions (
 event_key TEXT PRIMARY KEY,
 id TEXT NOT NULL UNIQUE,
 tenant_id INTEGER NOT NULL,
 order_id TEXT NOT NULL,
 kind TEXT NOT NULL,
 reason TEXT NOT NULL,
 state TEXT NOT NULL,
 created_at DATETIME NOT NULL,
 updated_at DATETIME NOT NULL,
 resolved_at DATETIME
);
CREATE INDEX idx_commercial_fulfillment_exceptions_tenant ON commercial_fulfillment_exceptions(tenant_id);
CREATE INDEX idx_commercial_fulfillment_exceptions_state ON commercial_fulfillment_exceptions(state);
CREATE INDEX idx_commercial_fulfillment_exceptions_order ON commercial_fulfillment_exceptions(order_id);
