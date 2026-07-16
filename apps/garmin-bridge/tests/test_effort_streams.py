"""Effort-analytics streams: extraction + the post-after-bulk sync step."""

from __future__ import annotations

from garmin_bridge import mapping, sync
from tests.conftest import FakeBackend, FakeResponse


def _detail_with_power(
    power: list[float],
    speed: list[float] | None = None,
    heart_rate: list[float] | None = None,
    cadence: list[float] | None = None,
) -> dict:
    """Build a get_activity_details-shaped dict with power (+ optional speed/HR/cadence)."""
    descriptors = [{"key": "directPower", "metricsIndex": 0}]
    cols: list[list[float]] = [power]
    if speed is not None:
        descriptors.append({"key": "directSpeed", "metricsIndex": len(cols)})
        cols.append(speed)
    if heart_rate is not None:
        descriptors.append({"key": "directHeartRate", "metricsIndex": len(cols)})
        cols.append(heart_rate)
    if cadence is not None:
        descriptors.append({"key": "directBikeCadence", "metricsIndex": len(cols)})
        cols.append(cadence)
    rows = [{"metrics": [c[i] for c in cols]} for i in range(len(power))]
    return {"metricDescriptors": descriptors, "activityDetailMetrics": rows}


def _detail_run(
    speed: list[float],
    double_cadence: list[float] | None = None,
    bike_cadence: list[float] | None = None,
) -> dict:
    """A run-shaped detail: speed plus Garmin's both-feet cadence column.

    Runs carry no power, so this deliberately omits it — the bug this covers is
    that the cadence fallback must fire on a payload that has no bike column.
    """
    descriptors = [{"key": "directSpeed", "metricsIndex": 0}]
    cols: list[list[float]] = [speed]
    if double_cadence is not None:
        descriptors.append({"key": "directDoubleCadence", "metricsIndex": len(cols)})
        cols.append(double_cadence)
    if bike_cadence is not None:
        descriptors.append({"key": "directBikeCadence", "metricsIndex": len(cols)})
        cols.append(bike_cadence)
    rows = [{"metrics": [c[i] for c in cols]} for i in range(len(speed))]
    return {"metricDescriptors": descriptors, "activityDetailMetrics": rows}


def _detail_hr_only(heart_rate: list[float]) -> dict:
    """A get_activity_details-shaped dict carrying only a heart-rate column."""
    descriptors = [{"key": "directHeartRate", "metricsIndex": 0}]
    rows = [{"metrics": [v]} for v in heart_rate]
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


def test_extract_streams_pulls_heart_rate_column():
    detail = _detail_with_power(
        [100, 200, 300], speed=[5.0, 6.0, 7.0], heart_rate=[140, 150, 160]
    )
    out = mapping._extract_streams(detail)
    assert out["power"] == [100.0, 200.0, 300.0]
    assert out["speed"] == [5.0, 6.0, 7.0]
    assert out["heart_rate"] == [140.0, 150.0, 160.0]


def test_extract_streams_heart_rate_only_activity():
    # A run with an HR strap but no power/speed meter → just heart_rate.
    detail = _detail_hr_only([135, 140, 145])
    out = mapping._extract_streams(detail)
    assert out == {"heart_rate": [135.0, 140.0, 145.0]}


def test_extract_streams_no_hr_series_unchanged():
    # Power+speed with no HR column → HR absent, others intact.
    detail = _detail_with_power([100, 200], speed=[5.0, 6.0])
    out = mapping._extract_streams(detail)
    assert "heart_rate" not in out
    assert set(out) == {"power", "speed"}


def test_extract_streams_drops_flat_zero_series():
    # A run with no power meter → power column all zero → dropped.
    detail = _detail_with_power([0, 0, 0])
    assert mapping._extract_streams(detail) == {}


def test_extract_streams_pulls_cadence_column():
    detail = _detail_with_power([100, 200, 300], cadence=[85, 90, 95])
    out = mapping._extract_streams(detail)
    assert out["power"] == [100.0, 200.0, 300.0]
    assert out["cadence"] == [85.0, 90.0, 95.0]


def test_extract_streams_no_cadence_column_unchanged():
    # Power only, no cadence descriptor → cadence absent, others intact.
    detail = _detail_with_power([100, 200])
    out = mapping._extract_streams(detail)
    assert "cadence" not in out
    assert set(out) == {"power"}


def test_extract_streams_drops_flat_zero_cadence():
    # A cadence column present but all zero (no sensor) → dropped.
    detail = _detail_with_power([100, 200, 300], cadence=[0, 0, 0])
    out = mapping._extract_streams(detail)
    assert "cadence" not in out
    assert set(out) == {"power"}


def test_extract_streams_malformed_cadence_shape_ignored():
    # A cadence descriptor whose column index is out of range for the rows →
    # gap-zeros (defensive), so the all-zero series is dropped; sync unaffected.
    detail = {
        "metricDescriptors": [
            {"key": "directPower", "metricsIndex": 0},
            {"key": "directBikeCadence", "metricsIndex": 7},  # beyond the row width
        ],
        "activityDetailMetrics": [{"metrics": [100]}, {"metrics": [200]}],
    }
    out = mapping._extract_streams(detail)
    assert out["power"] == [100.0, 200.0]
    assert "cadence" not in out


def test_extract_streams_drops_flat_zero_heart_rate():
    # HR sensor dropout for the whole activity → all-zero HR column dropped.
    detail = _detail_hr_only([0, 0, 0])
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


def test_activity_with_hr_stream_posts_heart_rate():
    raw = _raw_with_activity(
        111,
        stream=_detail_with_power(
            [250] * 10, speed=[8.0] * 10, heart_rate=[150] * 10
        ),
    )
    backend = FakeBackend()
    backend.responses["/workouts/bulk"] = FakeResponse(
        200, {"results": [{"index": 0, "id": "wid-111"}]}
    )

    sync.sync_day(backend, raw, "2026-06-12")

    stream_posts = [b for p, b in backend.posts if p == "/workouts/wid-111/streams"]
    assert len(stream_posts) == 1
    assert stream_posts[0]["heart_rate"] == [150.0] * 10


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


def test_extract_streams_run_double_cadence_posted_as_spm():
    # Garmin's directDoubleCadence is already steps/min — the familiar ~170.
    # Posted as reported: no halving, no doubling.
    detail = _detail_run([3.2, 3.3, 3.4], double_cadence=[170.0, 172.0, 174.0])
    out = mapping._extract_streams(detail)
    assert out["cadence"] == [170.0, 172.0, 174.0]
    assert out["speed"] == [3.2, 3.3, 3.4]
    assert "power" not in out


def test_extract_streams_bike_cadence_wins_when_both_columns_exist():
    # A ride's own column is the authoritative one; the run fallback must not
    # shadow it.
    detail = _detail_run(
        [8.0, 8.1, 8.2],
        double_cadence=[170.0, 172.0, 174.0],
        bike_cadence=[90.0, 91.0, 92.0],
    )
    out = mapping._extract_streams(detail)
    assert out["cadence"] == [90.0, 91.0, 92.0]


def test_extract_streams_run_without_any_cadence_column_degrades():
    # No recognizable cadence column: the sync proceeds, just without cadence.
    detail = _detail_run([3.2, 3.3, 3.4])
    out = mapping._extract_streams(detail)
    assert "cadence" not in out
    assert out["speed"] == [3.2, 3.3, 3.4]


def test_extract_streams_flat_zero_double_cadence_dropped():
    # An all-non-positive series is no data — same rule as every other column.
    detail = _detail_run([3.2, 3.3, 3.4], double_cadence=[0.0, 0.0, 0.0])
    out = mapping._extract_streams(detail)
    assert "cadence" not in out


def test_extract_streams_flat_zero_bike_cadence_falls_back_to_run_column():
    # A dead bike column must not suppress a live run column: the fallback is
    # gated on usable data, not on mere presence.
    detail = _detail_run(
        [3.2, 3.3, 3.4],
        double_cadence=[170.0, 172.0, 174.0],
        bike_cadence=[0.0, 0.0, 0.0],
    )
    out = mapping._extract_streams(detail)
    assert out["cadence"] == [170.0, 172.0, 174.0]
