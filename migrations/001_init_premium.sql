-- Premium fields and tickets
ALTER TABLE profile
  ADD COLUMN IF NOT EXISTS premium_until TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS premium_tickets (
  id SERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  login TEXT NOT NULL,
  email TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS premium_tickets_user_id_idx ON premium_tickets(user_id);
CREATE INDEX IF NOT EXISTS premium_tickets_status_idx ON premium_tickets(status);
