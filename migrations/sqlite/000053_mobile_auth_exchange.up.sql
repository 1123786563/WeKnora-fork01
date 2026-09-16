CREATE TABLE IF NOT EXISTS mobile_auth_exchanges (
  code_hash TEXT PRIMARY KEY,
  state_hash TEXT NOT NULL,
  redirect_uri TEXT NOT NULL,
  challenge TEXT NOT NULL,
  subject TEXT NOT NULL,
  expires_at DATETIME NOT NULL,
  consumed_at DATETIME NULL
);
CREATE INDEX IF NOT EXISTS idx_mobile_auth_exchanges_expires_at ON mobile_auth_exchanges(expires_at);
