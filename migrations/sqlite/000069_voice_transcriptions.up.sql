CREATE TABLE voice_transcriptions (
    tenant_id INTEGER NOT NULL,
    request_id VARCHAR(128) NOT NULL,
    call_id VARCHAR(64) NOT NULL,
    owner_id VARCHAR(255) NOT NULL,
    run_id VARCHAR(64) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL,
    text TEXT NOT NULL DEFAULT '',
    audio_seconds INTEGER NOT NULL DEFAULT 0,
    settled INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (status IN ('succeeded', 'failed')),
    CHECK (audio_seconds >= 0)
);
CREATE UNIQUE INDEX uq_voice_transcription_request ON voice_transcriptions (tenant_id, request_id);
CREATE INDEX idx_voice_transcription_call ON voice_transcriptions (tenant_id, call_id);
