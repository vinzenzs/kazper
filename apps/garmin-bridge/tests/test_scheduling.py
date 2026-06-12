"""Bridge scheduling endpoints: /workouts, /schedule, DELETE /schedule, /calendar."""

from __future__ import annotations

import types

from fastapi.testclient import TestClient

from garmin_bridge.app import create_app
from tests.conftest import FakeBackend

_VALID_STEPS = [
    {"type": "step", "intent": "active", "duration": {"kind": "open"}, "target": {"kind": "none"}},
]


def _gc_stub():
    stub = types.SimpleNamespace()
    stub.load_api = lambda token: object()
    stub.create_workout = lambda api, payload: "gw-123"
    stub.schedule_workout = lambda api, wid, date: f"sched-{wid}-{date}"
    stub.calls = {"unschedule": []}
    stub.unschedule_workout = lambda api, sid: stub.calls["unschedule"].append(sid)
    stub.get_calendar = lambda api, f, t: {"from": f, "to": t, "items": [{"date": f, "garminScheduleId": "s1"}]}
    return stub


def test_create_workout_returns_id(config):
    app = create_app(config, gc=_gc_stub(), backend_factory=lambda: FakeBackend())
    client = TestClient(app)
    resp = client.post("/workouts", json={"sport": "run", "name": "Easy", "steps": _VALID_STEPS})
    assert resp.status_code == 200
    assert resp.json() == {"garmin_workout_id": "gw-123"}


def test_create_workout_bad_steps_400(config):
    app = create_app(config, gc=_gc_stub(), backend_factory=lambda: FakeBackend())
    client = TestClient(app)
    resp = client.post("/workouts", json={"sport": "run", "name": "x", "steps": []})
    assert resp.status_code == 400
    assert resp.json()["error"] == "invalid_steps"


def test_create_workout_missing_token_409(config):
    app = create_app(config, gc=_gc_stub(), backend_factory=lambda: FakeBackend(token=None))
    client = TestClient(app)
    resp = client.post("/workouts", json={"sport": "run", "name": "x", "steps": _VALID_STEPS})
    assert resp.status_code == 409
    assert resp.json()["error"] == "login_required"


def test_schedule_returns_schedule_id(config):
    app = create_app(config, gc=_gc_stub(), backend_factory=lambda: FakeBackend())
    client = TestClient(app)
    resp = client.post("/schedule", json={"garmin_workout_id": "gw-9", "date": "2026-06-12"})
    assert resp.status_code == 200
    assert resp.json() == {"garmin_schedule_id": "sched-gw-9-2026-06-12"}


def test_unschedule_calls_garmin(config):
    gc = _gc_stub()
    app = create_app(config, gc=gc, backend_factory=lambda: FakeBackend())
    client = TestClient(app)
    resp = client.request("DELETE", "/schedule", params={"schedule_id": "s-42"})
    assert resp.status_code == 200
    assert resp.json() == {"unscheduled": True}
    assert gc.calls["unschedule"] == ["s-42"]


def test_calendar_requires_range(config):
    app = create_app(config, gc=_gc_stub(), backend_factory=lambda: FakeBackend())
    client = TestClient(app)
    assert client.get("/calendar").status_code == 400


def test_calendar_passthrough(config):
    app = create_app(config, gc=_gc_stub(), backend_factory=lambda: FakeBackend())
    client = TestClient(app)
    resp = client.get("/calendar", params={"from": "2026-06-01", "to": "2026-06-30"})
    assert resp.status_code == 200
    body = resp.json()
    assert body["from"] == "2026-06-01"
    assert body["items"][0]["garminScheduleId"] == "s1"
