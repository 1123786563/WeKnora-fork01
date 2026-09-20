ALTER TABLE {{schema}}.operations ADD COLUMN stage TEXT;
UPDATE {{schema}}.operations SET stage = phase WHERE stage IS NULL;
ALTER TABLE {{schema}}.operations ALTER COLUMN stage SET NOT NULL;
ALTER TABLE {{schema}}.operations
    ADD CONSTRAINT operations_lease_token_uint64_check
    CHECK (lease_token >= 0 AND lease_token <= 18446744073709551615);
