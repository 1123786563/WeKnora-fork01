CREATE TABLE voice_sessions (
    tenant_id INTEGER NOT NULL,
    id VARCHAR(64) NOT NULL,
    owner_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(64) NOT NULL DEFAULT '',
    run_id VARCHAR(64) NOT NULL DEFAULT '',
    provider_ref VARCHAR(255) NOT NULL DEFAULT '',
    state VARCHAR(16) NOT NULL DEFAULT 'open',
    deadline DATETIME NOT NULL,
    expires_at DATETIME NOT NULL,
    max_seconds INTEGER NOT NULL DEFAULT 0,
    reservation_key VARCHAR(128) NOT NULL DEFAULT '',
    token_hash VARCHAR(64) NOT NULL DEFAULT '',
    audio_seconds INTEGER NOT NULL DEFAULT 0,
    settled_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (state IN ('open', 'unknown', 'closed')),
    CHECK (max_seconds > 0),
    CHECK (audio_seconds >= 0)
);
CREATE UNIQUE INDEX uq_voice_session_identity ON voice_sessions (tenant_id, id);
CREATE INDEX idx_voice_session_owner ON voice_sessions (tenant_id, owner_id);
CREATE INDEX idx_voice_session_product ON voice_sessions (tenant_id, session_id, state);
