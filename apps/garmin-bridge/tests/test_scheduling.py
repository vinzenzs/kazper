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
    # workout-library management + export (garmin-workout-library-mgmt)
    stub.calls["delete"] = []
    stub.calls["hydration"] = []
    stub.delete_workout = lambda api, wid: (stub.calls["delete"].append(wid), True)[1]
    stub.get_workouts = lambda api, start, limit: {"workouts": [{"workoutId": 1}], "start": start, "limit": limit}
    stub.get_workout_by_id = lambda api, wid: {"workoutId": wid, "name": "Library Workout"}
    stub.add_hydration_data = lambda api, ml, date: stub.calls["hydration"].append((ml, date))
    stub.download_activity = lambda api, aid, fmt: b"FITBYTES"
    return stub


def test_delete_workout_returns_deleted(config):
    gc = _gc_stub()
    app = create_app(config, gc=gc, backend_factory=lambda: FakeBackend())
    client = TestClient(app)
    resp = client.delete("/workouts/gw-7")
    assert resp.status_code == 200
    assert resp.json() == {"deleted": True}
    assert gc.calls["delete"] == ["gw-7"]


def test_delete_workout_already_absent(config):
    gc = _gc_stub()
    gc.delete_workout = lambda api, wid: False
    app = create_app(config, gc=gc, backend_factory=lambda: FakeBackend())
    client = TestClient(app)
    resp = client.delete("/workouts/gw-gone")
    assert resp.status_code == 200
    assert resp.json() == {"deleted": False, "already_absent": True}


def test_list_and_get_workouts_passthrough(config):
    app = create_app(config, gc=_gc_stub(), backend_factory=lambda: FakeBackend())
    client = TestClient(app)
    lst = client.get("/workouts", params={"start": 5, "limit": 3})
    assert lst.status_code == 200
    assert lst.json()["start"] == 5 and lst.json()["limit"] == 3
    one = client.get("/workouts/gw-7")
    assert one.status_code == 200
    assert one.json()["workoutId"] == "gw-7"


def test_push_hydration(config):
    gc = _gc_stub()
    app = create_app(config, gc=gc, backend_factory=lambda: FakeBackend())
    client = TestClient(app)
    resp = client.post("/hydration", json={"value_ml": 750.0, "date": "2026-06-13"})
    assert resp.status_code == 200
    assert resp.json()["pushed"] is True
    assert gc.calls["hydration"] == [(750.0, "2026-06-13")]


def test_export_activity_base64_envelope(config):
    import base64

    app = create_app(config, gc=_gc_stub(), backend_factory=lambda: FakeBackend())
    client = TestClient(app)
    resp = client.get("/activity/act-1/export")
    assert resp.status_code == 200
    env = resp.json()
    assert env["activity_id"] == "act-1"
    assert env["format"] == "fit"
    assert env["filename"] == "act-1.fit"
    assert base64.b64decode(env["content_base64"]) == b"FITBYTES"


def test_export_activity_missing_token_409(config):
    app = create_app(config, gc=_gc_stub(), backend_factory=lambda: FakeBackend(token=None))
    client = TestClient(app)
    assert client.get("/activity/act-1/export").status_code == 409


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
