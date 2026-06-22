-- Garmin sync-run log (per add-garmin-connect-and-sync-status). One row per
-- bridge `/sync` invocation: the bridge opens a run (status='running') before
-- fetching and closes it (success|error) after. This is the backend's
-- authoritative record of when Kazper last pulled from Garmin — distinct from
-- devices.last_sync_at, which is the watch's own field. Single-user, so the
-- table is unscoped. The dominant read is "the latest run", hence the
-- started_at DESC index.
CREATE TABLE sync_runs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ NULL,
    status      TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'success', 'error')),
    window_from DATE NULL,
    window_to   DATE NULL,
    error       TEXT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sync_runs_started_at_idx ON sync_runs (started_at DESC);
