-- A populated user-state boundary must never be removed by a rollback.
CREATE TABLE native_user_state_down_guard (
    must_be_empty INTEGER NOT NULL CHECK (must_be_empty = 1)
);
INSERT INTO native_user_state_down_guard (must_be_empty)
SELECT CASE WHEN EXISTS (SELECT 1 FROM native_user_state) THEN 0 ELSE 1 END;
DROP TABLE native_user_state_down_guard;

DROP TABLE native_user_state;
