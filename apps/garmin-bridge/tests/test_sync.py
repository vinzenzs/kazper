"""Sync data-plane: fixture → expected REST calls, idempotency, missing token."""

from __future__ import annotations

import dataclasses
import types

from fastapi.testclient import TestClient

from garmin_bridge import sync
from garmin_bridge.app import create_app
from tests.conftest import FakeBackend, FakeResponse


def _gc_stub(raw_day, *, fail_dates=()):
    """Stub gc whose load_api/fetch_day return the recorded fixture, recording
    the dates fetched and optionally raising for ``fail_dates``."""
    stub = types.SimpleNamespace()
    stub.fetched: list[str] = []
    stub.load_api = lambda token_b64: object()

    def fetch_day(api, date):
        stub.fetched.append(date)
        if date in fail_dates:
            raise RuntimeError(f"garmin boom on {date}")
        return raw_day

    stub.fetch_day = fetch_day
    return stub


def test_sync_day_issues_expected_rest_calls(raw_day):
    backend = FakeBackend()
    summary = sync.sync_day(backend, raw_day, "2026-06-12")

    paths = [p for p, _ in backend.posts]
    assert "/recovery-metrics" in paths
    assert "/fitness-metrics" in paths
    assert "/hydration-balance" in paths
    assert "/weight" in paths
    assert "/workouts/bulk" in paths

    # The bulk body carries both activities with garmin external_ids.
    bulk_body = next(b for p, b in backend.posts if p == "/workouts/bulk")
    ext_ids = {w["external_id"] for w in bulk_body["workouts"]}
    assert ext_ids == {"garmin:1234567", "garmin:1234568", "garmin:1234569"}
    assert all(w["source"] == "garmin" for w in bulk_body["workouts"])
    # The run carries its nested splits + zone detail through the bulk post
    # unchanged (no per-activity round-trips at the sync layer).
    run = next(w for w in bulk_body["workouts"] if w["external_id"] == "garmin:1234567")
    assert len(run["splits"]) == 2
    assert run["secs_in_zone_3"] == 1500

    assert summary["ok"] is True


def test_sync_is_idempotent_on_rerun(raw_day):
    """Re-running the same day produces the same write set — the backend's
    date-upsert + external_id dedup make it safe; the bridge sends identically."""
    first = FakeBackend()
    sync.sync_day(first, raw_day, "2026-06-12")
    second = FakeBackend()
    sync.sync_day(second, raw_day, "2026-06-12")
    assert first.posts == second.posts


def test_partial_failure_does_not_abort_sync(raw_day):
    backend = FakeBackend()
    # recovery endpoint errors; everything else must still be attempted.
    backend.responses["/recovery-metrics"] = FakeResponse(500)
    summary = sync.sync_day(backend, raw_day, "2026-06-12")

    paths = [p for p, _ in backend.posts]
    assert "/fitness-metrics" in paths  # attempted despite recovery failing
    assert "/workouts/bulk" in paths
    assert summary["ok"] is False
    assert "recovery" in summary["errors"]


def test_empty_day_skips_everything():
    backend = FakeBackend()
    summary = sync.sync_day(backend, {"date": "2026-06-12"}, "2026-06-12")
    assert backend.posts == []
    assert summary["ok"] is True
    assert summary["results"]["recovery"].startswith("skipped")


def test_sync_endpoint_missing_token_returns_login_required(config, raw_day):
    backend = FakeBackend(token=None)  # GET /garmin/token → 404
    app = create_app(config, gc=_gc_stub(raw_day), backend_factory=lambda: backend)
    client = TestClient(app)

    resp = client.post("/sync", json={"date": "2026-06-12"})
    assert resp.status_code == 409
    assert resp.json()["error"] == "login_required"
    # Nothing was written.
    assert backend.posts == []


def test_sync_endpoint_explicit_date_syncs_one_day(config, raw_day):
    """An explicit date syncs exactly that day with no lookback window — the
    single-day summary shape, and fetch_day called once for that date."""
    backend = FakeBackend(token=b"stored-blob")
    gc = _gc_stub(raw_day)
    app = create_app(config, gc=gc, backend_factory=lambda: backend)
    client = TestClient(app)

    resp = client.post("/sync", json={"date": "2026-06-12"})
    assert resp.status_code == 200
    body = resp.json()
    assert body["ok"] is True
    assert body["date"] == "2026-06-12"
    assert "days" not in body  # single-day shape, not a window
    assert gc.fetched == ["2026-06-12"]


def test_sync_dateless_syncs_rolling_window(config, raw_day):
    """Dateless sync with the default lookback (2) syncs today + 2 prior days,
    oldest-first, as a per-day window result."""
    backend = FakeBackend(token=b"stored-blob")
    gc = _gc_stub(raw_day)
    app = create_app(
        config, gc=gc, backend_factory=lambda: backend, now=lambda tz: "2026-06-13"
    )
    client = TestClient(app)

    resp = client.post("/sync")
    assert resp.status_code == 200
    body = resp.json()
    assert body["days_total"] == 3
    assert body["days_failed"] == 0
    assert [d["date"] for d in body["days"]] == ["2026-06-11", "2026-06-12", "2026-06-13"]
    assert gc.fetched == ["2026-06-11", "2026-06-12", "2026-06-13"]


def test_sync_window_one_bad_day_does_not_sink_the_rest(config, raw_day):
    """A single failing day is recorded failed and the window still syncs the
    others; the response reports 207 with the per-day outcome."""
    backend = FakeBackend(token=b"stored-blob")
    gc = _gc_stub(raw_day, fail_dates={"2026-06-12"})
    app = create_app(
        config, gc=gc, backend_factory=lambda: backend, now=lambda tz: "2026-06-13"
    )
    client = TestClient(app)

    resp = client.post("/sync")
    assert resp.status_code == 207
    body = resp.json()
    assert body["days_total"] == 3
    assert body["days_failed"] == 1
    bad = next(d for d in body["days"] if d["date"] == "2026-06-12")
    assert bad["ok"] is False
    assert "boom" in bad["error"]
    # The other two days still synced.
    assert {d["date"] for d in body["days"] if d.get("ok")} == {"2026-06-11", "2026-06-13"}


def test_sync_lookback_zero_collapses_to_today(config, raw_day):
    """SYNC_LOOKBACK_DAYS=0 → the dateless window is today only."""
    backend = FakeBackend(token=b"stored-blob")
    gc = _gc_stub(raw_day)
    cfg = dataclasses.replace(config, sync_lookback_days=0)
    app = create_app(
        cfg, gc=gc, backend_factory=lambda: backend, now=lambda tz: "2026-06-13"
    )
    client = TestClient(app)

    resp = client.post("/sync")
    assert resp.status_code == 200
    body = resp.json()
    assert body["days_total"] == 1
    assert [d["date"] for d in body["days"]] == ["2026-06-13"]
    assert gc.fetched == ["2026-06-13"]


# --- sync-run reporting (add-garmin-connect-and-sync-status) ----------------


def test_sync_window_opens_and_closes_run_success(config, raw_day):
    """A dateless window opens a run with [oldest, newest] and closes it success."""
    backend = FakeBackend(token=b"stored-blob")
    app = create_app(
        config, gc=_gc_stub(raw_day), backend_factory=lambda: backend, now=lambda tz: "2026-06-13"
    )
    resp = TestClient(app).post("/sync")
    assert resp.status_code == 200
    assert backend.sync_runs_opened == [("2026-06-11", "2026-06-13")]
    assert backend.sync_runs_closed == [("run-id-1", "success", None)]


def test_sync_explicit_date_opens_run_for_that_day(config, raw_day):
    """An explicit date opens a single-day-window run and closes it success."""
    backend = FakeBackend(token=b"stored-blob")
    app = create_app(config, gc=_gc_stub(raw_day), backend_factory=lambda: backend)
    resp = TestClient(app).post("/sync", json={"date": "2026-06-12"})
    assert resp.status_code == 200
    assert backend.sync_runs_opened == [("2026-06-12", "2026-06-12")]
    assert backend.sync_runs_closed == [("run-id-1", "success", None)]


def test_sync_hard_failure_closes_run_error(config, raw_day):
    """A hard failure (fetch raises) closes the run as error with the message."""
    backend = FakeBackend(token=b"stored-blob")
    gc = _gc_stub(raw_day, fail_dates={"2026-06-12"})
    app = create_app(config, gc=gc, backend_factory=lambda: backend)
    resp = TestClient(app).post("/sync", json={"date": "2026-06-12"})
    assert resp.status_code == 500
    assert backend.sync_runs_opened == [("2026-06-12", "2026-06-12")]
    assert len(backend.sync_runs_closed) == 1
    run_id, status, error = backend.sync_runs_closed[0]
    assert (run_id, status) == ("run-id-1", "error")
    assert "boom" in error


def test_sync_run_reporting_is_best_effort(config, raw_day):
    """When opening a run fails (id None), the sync still completes — reporting
    never aborts the data write."""
    backend = FakeBackend(token=b"stored-blob")
    backend.open_run_returns = None
    app = create_app(
        config, gc=_gc_stub(raw_day), backend_factory=lambda: backend, now=lambda tz: "2026-06-13"
    )
    resp = TestClient(app).post("/sync")
    assert resp.status_code == 200
    assert backend.sync_runs_closed == [(None, "success", None)]
