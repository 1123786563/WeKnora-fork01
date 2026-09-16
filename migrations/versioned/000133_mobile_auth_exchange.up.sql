CREATE TABLE IF NOT EXISTS mobile_auth_exchanges (
  code_hash VARCHAR(64) PRIMARY KEY,
  state_hash VARCHAR(64) NOT NULL,
  redirect_uri VARCHAR(2048) NOT NULL,
  challenge VARCHAR(128) NOT NULL,
  subject VARCHAR(255) NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ NULL
);
CREATE INDEX IF NOT EXISTS idx_mobile_auth_exchanges_expires_at ON mobile_auth_exchanges(expires_at);
