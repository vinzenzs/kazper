-- Android push notifications (per add-garmin-relogin-push). Two single-user
-- primitives:
--
-- push_tokens — the FCM registration tokens the mobile companion registers so
-- the backend can deliver a notification. Tokens rotate; registration upserts
-- by the opaque token string, so a refreshed-identical token is a no-op and a
-- rotated token is a new row. Device identifiers, not nutrition data.
CREATE TABLE push_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token      TEXT NOT NULL,
    platform   TEXT NOT NULL DEFAULT 'android',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX push_tokens_token_idx ON push_tokens (token);

-- relogin_latch — a single-row guard so a Garmin relogin-needed notification is
-- sent once per outage, not on every failed sync. Set when a relogin push is
-- sent; cleared when a fresh token is stored (PUT /garmin/token) or a sync run
-- closes success. Single user ⇒ exactly one row (id = 1), seeded here.
CREATE TABLE relogin_latch (
    id          SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    notified    BOOLEAN NOT NULL DEFAULT false,
    notified_at TIMESTAMPTZ NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO relogin_latch (id, notified) VALUES (1, false);
