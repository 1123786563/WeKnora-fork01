-- SP1/G4: ReservationRow carries an owner column (budget.go) used by lease
-- takeover (budget_lease.go SET owner = ?), but the 000116 DDL never created
-- it — Reserve INSERTs failed on production Postgres. Additive, backfilled ''.
ALTER TABLE commercial_reservations ADD COLUMN IF NOT EXISTS owner VARCHAR(255) NOT NULL DEFAULT '';
