"""Effort-analytics streams: extraction + the post-after-bulk sync step."""

from __future__ import annotations

from garmin_bridge import mapping, sync
from tests.conftest import FakeBackend, FakeResponse


def _detail_with_power(power: list[float], speed: list[float] | None = None) -> dict:
    """Build a get_activity_details-shaped dict with power (and optional speed)."""
    descriptors = [{"key": "directPower", "metricsIndex": 0}]
    if speed is not None:
        descriptors.append({"key": "directSpeed", "metricsIndex": 1})
    rows = []
    for i, p in enumerate(power):
        metrics = [p]
        if speed is not None:
            metrics.append(speed[i])
        rows.append({"metrics": metrics})
    return {"metricDescriptors": descriptors, "activityDetailMetrics": rows}


def _raw_with_activity(activity_id: int, *, stream: dict | None) -> dict:
    """A minimal raw day with one completed activity and optional stream detail."""
    return {
        "activities": [
            {
                "activityId": activity_id,
                "activityName": "Ride",
                "startTimeGMT": "2026-06-12 06:00:00",
                "duration": 3600.0,
                "activityType": {"typeKey": "cycling"},
            }
        ],
        "activity_details": {str(activity_id): {"stream": stream} if stream else {}},
    }


# --- extraction -----------------------------------------------------------


def test_extract_streams_pulls_power_and_speed_columns():
    detail = _detail_with_power([100, 200, 300], speed=[5.0, 6.0, 7.0])
    out = mapping._extract_streams(detail)
    assert out["power"] == [100.0, 200.0, 300.0]
    assert out["speed"] == [5.0, 6.0, 7.0]


def test_extract_streams_drops_flat_zero_series():
    # A run with no power meter → power column all zero → dropped.
    detail = _detail_with_power([0, 0, 0])
    assert mapping._extract_streams(detail) == {}


def test_extract_streams_defensive_on_bad_shape():
    assert mapping._extract_streams(None) == {}
    assert mapping._extract_streams({}) == {}
    assert mapping._extract_streams({"metricDescriptors": [], "activityDetailMetrics": []}) == {}


def test_map_workout_streams_keys_by_external_id():
    raw = _raw_with_activity(111, stream=_detail_with_power([100, 200]))
    out = mapping.map_workout_streams(raw)
    assert set(out) == {"garmin:111"}
    assert out["garmin:111"]["power"] == [100.0, 200.0]


# --- sync post-after-bulk -------------------------------------------------


def test_activity_with_power_stream_posts_after_bulk():
    raw = _raw_with_activity(111, stream=_detail_with_power([250] * 10))
    backend = FakeBackend()
    # The bulk upsert returns the minted id joined by index.
    backend.responses["/workouts/bulk"] = FakeResponse(
        200, {"results": [{"index": 0, "id": "wid-111"}]}
    )

    sync.sync_day(backend, raw, "2026-06-12")

    stream_posts = [(p, b) for p, b in backend.posts if p == "/workouts/wid-111/streams"]
    assert len(stream_posts) == 1
    assert stream_posts[0][1]["power"] == [250.0] * 10


def test_activity_without_stream_skips():
    raw = _raw_with_activity(111, stream=None)
    backend = FakeBackend()
    backend.responses["/workouts/bulk"] = FakeResponse(
        200, {"results": [{"index": 0, "id": "wid-111"}]}
    )

    sync.sync_day(backend, raw, "2026-06-12")

    assert not [p for p, _ in backend.posts if p.endswith("/streams")]


def test_rerun_reposts_streams_idempotently():
    raw = _raw_with_activity(111, stream=_detail_with_power([200] * 10))
    for _ in range(2):
        backend = FakeBackend()
        backend.responses["/workouts/bulk"] = FakeResponse(
            200, {"results": [{"index": 0, "id": "wid-111"}]}
        )
        sync.sync_day(backend, raw, "2026-06-12")
        assert [p for p, _ in backend.posts if p == "/workouts/wid-111/streams"]
