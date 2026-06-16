"""Translate the backend's clean step model into a garminconnect structured-
workout payload (``executableStepDTO`` / ``repeatGroupDTO``).

This is the single place Garmin's structured-workout schema lives — the backend
sends only sport + name + our step model and never sees this shape (design D1 of
add-garmin-scheduling). It is pure (no I/O) and exhaustively unit-tested, because
the structured-workout API is the most intricate part of garminconnect: when
Garmin changes an id or key, the fix is here plus a ``pip`` bump.

The numeric ids below are Garmin Connect's published workout vocabulary
(sportTypeId / stepTypeId / conditionTypeId / workoutTargetTypeId).
"""

from __future__ import annotations

from typing import Any

# --- Garmin id/key vocabularies -----------------------------------------

# Our Sport enum → Garmin workout-service (sportTypeId, sportTypeKey).
#
# Garmin validates by sportTypeId and OVERWRITES the key with its own canonical
# one, so the id must be exactly right: a wrong id is silently stored as
# sportTypeId 0 (no sport on the watch) rather than erroring. The full
# workout-service sportType vocabulary, verified live against the API
# (2026-06-15) — keep this list as the reference when adding a sport:
#
#    1  running             7  yoga
#    2  cycling             8  pilates
#    3  other               9  hiit
#    4  swimming           11  mobility
#    5  strength_training  12  walking
#    6  cardio_training    13  rucking
#
# NB: "other" is id 3 — id 9 is hiit (a long-standing mismap pushed "other"
# workouts to the watch as HIIT).
_SPORT = {
    "run": (1, "running"),
    "bike": (2, "cycling"),
    "swim": (4, "swimming"),
    "strength": (5, "strength_training"),
    "yoga": (7, "yoga"),
    "mobility": (11, "mobility"),
    "other": (3, "other"),
}

# Our intent → Garmin stepType. Garmin has no "active"; it maps to "other".
_STEP_TYPE = {
    "warmup": (1, "warmup"),
    "cooldown": (2, "cooldown"),
    "interval": (3, "interval"),
    "recovery": (4, "recovery"),
    "rest": (5, "rest"),
    "active": (7, "other"),
}
_REPEAT_STEP_TYPE = (6, "repeat")

# Duration kind → Garmin endCondition. "open" maps to lap.button (press-lap to
# advance), the closest Garmin concept to an open-ended step.
_END_CONDITION = {
    "time": (2, "time"),
    "distance": (3, "distance"),
    "lap_button": (1, "lap.button"),
    "open": (1, "lap.button"),
}
_ITERATIONS_CONDITION = (7, "iterations")

# Target kind → Garmin workoutTargetType.
_TARGET_TYPE = {
    "none": (1, "no.target"),
    "power_zone": (2, "power.zone"),
    "hr_zone": (4, "heart.rate.zone"),
    "pace": (6, "pace.zone"),
    "swim_pace": (6, "pace.zone"),  # same Garmin pace gate; units differ (sec/100m)
    "hr_bpm": (4, "heart.rate.zone"),
    "power_w": (2, "power.zone"),
    # Garmin's cadence target (id 3, canonical key "cadence" — verified live
    # 2026-06-16 by read-back) is sport-agnostic: the watch reads the rpm/spm
    # range as bike rpm or run spm by the workout's sport — no separate
    # run.cadence target type. Backend gates cadence to bike/run.
    "cadence": (3, "cadence"),
    "rpe": (1, "no.target"),  # Garmin has no RPE target; carry it as untargeted
}


class BuildError(ValueError):
    """Raised when the step model cannot be translated (unknown kind, etc.)."""


def build_payload(sport: str, name: str, steps: list[dict[str, Any]]) -> dict[str, Any]:
    """Build the garminconnect create-workout payload from our step model."""
    if sport not in _SPORT:
        raise BuildError(f"unknown sport {sport!r}")
    sport_id, sport_key = _SPORT[sport]
    sport_obj = {"sportTypeId": sport_id, "sportTypeKey": sport_key}

    order = _Counter()
    workout_steps = [_build_node(node, sport_obj, order) for node in steps]
    if not workout_steps:
        raise BuildError("steps must be non-empty")

    return {
        "sportType": sport_obj,
        "workoutName": name,
        "workoutSegments": [
            {
                "segmentOrder": 1,
                "sportType": sport_obj,
                "workoutSteps": workout_steps,
            }
        ],
    }


# Multisport top-level sportType and the transition (T1/T2) sport, in the Garmin
# workout-service vocabulary. VERIFIED LIVE 2026-06-16 (add-multisport task 3.2)
# via a create+read round-trip: Garmin normalizes a sent sportTypeId to its
# canonical key on store, so the read-back is authoritative. Findings:
#   id 10 → "multi_sport"      id 9 → "hiit"   (a first guess of 9 pushed the
#   workout to the watch as HIIT — the exact silent failure the _SPORT note warns
#   about).
# There is NO "transition" workout sportType. Confirmed by an exhaustive live id
# probe (2026-06-16): the complete valid workout-service vocabulary is ids 1..13
# (run, bike, other, swim, strength, cardio, yoga, pilates, hiit, multi_sport,
# mobility, walking, rucking); id 0 and every id >= 14 store as sportTypeId 0
# (no sport), and none normalizes to "transition" — it exists only as an ACTIVITY
# type, not a workout sportType (hence its absence from garminconnect/garth).
# Garmin accepts a transition segment typed multi_sport(10), so T1/T2 segments
# compile as multi_sport-typed duration segments. Per-segment sport ids come from
# the verified _SPORT map.
_MULTISPORT_SPORT = (10, "multi_sport")
_TRANSITION_SPORT = (10, "multi_sport")


def build_multisport_payload(name: str, segments: list[dict[str, Any]]) -> dict[str, Any]:
    """Build a multisport create-workout payload: one workoutSegments entry per
    segment, each carrying its own sportType, with stepOrder monotonic across the
    whole workout (Garmin numbers every step, across segments). Transition (T1/T2)
    segments use the transition sport and a single duration-bounded step. Requires
    at least two sport (non-transition) segments."""
    if not segments:
        raise BuildError("segments must be non-empty")
    order = _Counter()
    out_segments: list[dict[str, Any]] = []
    sport_count = 0
    for i, seg in enumerate(segments, start=1):
        seg_sport = seg.get("sport")
        if seg_sport == "transition":
            tid, tkey = _TRANSITION_SPORT
            seg_obj = {"sportTypeId": tid, "sportTypeKey": tkey}
            workout_steps = [_transition_step(seg.get("duration") or {}, order)]
        else:
            if seg_sport not in _SPORT:
                raise BuildError(f"unknown segment sport {seg_sport!r}")
            sid, skey = _SPORT[seg_sport]
            seg_obj = {"sportTypeId": sid, "sportTypeKey": skey}
            nodes = seg.get("steps") or []
            workout_steps = [_build_node(node, seg_obj, order) for node in nodes]
            if not workout_steps:
                raise BuildError("segment steps must be non-empty")
            sport_count += 1
        out_segments.append(
            {"segmentOrder": i, "sportType": seg_obj, "workoutSteps": workout_steps}
        )
    if sport_count < 2:
        raise BuildError("multisport needs at least two sport segments")
    msid, mskey = _MULTISPORT_SPORT
    return {
        "sportType": {"sportTypeId": msid, "sportTypeKey": mskey},
        "workoutName": name,
        "workoutSegments": out_segments,
    }


def _transition_step(duration: dict[str, Any], order: _Counter) -> dict[str, Any]:
    """A transition segment carries one duration-bounded, untargeted step (T1/T2
    is athlete-paced — typically lap_button/open). Mirrors _build_step's shape."""
    cond, cond_value = _end_condition(duration)
    cond_id, cond_key = cond
    rest_id, rest_key = _STEP_TYPE["rest"]
    step: dict[str, Any] = {
        "type": "ExecutableStepDTO",
        "stepOrder": order.next(),
        "stepType": {"stepTypeId": rest_id, "stepTypeKey": rest_key},
        "endCondition": {"conditionTypeId": cond_id, "conditionTypeKey": cond_key},
        "endConditionValue": cond_value,
    }
    step.update(_target({"kind": "none"}))
    return step


class _Counter:
    """Monotonic stepOrder allocator (Garmin numbers every step, nested too)."""

    def __init__(self) -> None:
        self._n = 0

    def next(self) -> int:
        self._n += 1
        return self._n


def _build_node(node: dict[str, Any], sport_obj: dict, order: _Counter) -> dict[str, Any]:
    kind = node.get("type")
    if kind == "repeat":
        return _build_repeat(node, sport_obj, order)
    if kind == "step":
        return _build_step(node, sport_obj, order)
    raise BuildError(f"unknown node type {kind!r}")


def _build_repeat(node: dict[str, Any], sport_obj: dict, order: _Counter) -> dict[str, Any]:
    count = node.get("count")
    children = node.get("steps") or []
    if not isinstance(count, int) or count < 2:
        raise BuildError("repeat count must be an integer >= 2")
    if not children:
        raise BuildError("repeat group must have steps")
    step_order = order.next()
    repeat_id, repeat_key = _REPEAT_STEP_TYPE
    cond_id, cond_key = _ITERATIONS_CONDITION
    built_children = []
    for child in children:
        if child.get("type") != "step":
            raise BuildError("repeat groups may only contain single steps")
        built_children.append(_build_step(child, sport_obj, order))
    return {
        "type": "RepeatGroupDTO",
        "stepOrder": step_order,
        "stepType": {"stepTypeId": repeat_id, "stepTypeKey": repeat_key},
        "numberOfIterations": count,
        "smartRepeat": False,
        "endCondition": {"conditionTypeId": cond_id, "conditionTypeKey": cond_key},
        "endConditionValue": count,
        "workoutSteps": built_children,
    }


def _build_step(node: dict[str, Any], sport_obj: dict, order: _Counter) -> dict[str, Any]:
    intent = node.get("intent")
    if intent not in _STEP_TYPE:
        raise BuildError(f"unknown intent {intent!r}")
    step_id, step_key = _STEP_TYPE[intent]

    cond, cond_value = _end_condition(node.get("duration") or {})
    cond_id, cond_key = cond

    step: dict[str, Any] = {
        "type": "ExecutableStepDTO",
        "stepOrder": order.next(),
        "stepType": {"stepTypeId": step_id, "stepTypeKey": step_key},
        "endCondition": {"conditionTypeId": cond_id, "conditionTypeKey": cond_key},
        "endConditionValue": cond_value,
    }
    if node.get("note"):
        step["description"] = node["note"]
    step.update(_target(node.get("target") or {"kind": "none"}))
    # Bike steps may carry a second simultaneous target (e.g. power + cadence);
    # the backend validates it bike-only and family-distinct, so just emit it.
    secondary = node.get("secondary_target")
    if secondary:
        step.update(_secondary_target(secondary))
    return step


def _end_condition(duration: dict[str, Any]) -> tuple[tuple[int, str], float | None]:
    kind = duration.get("kind")
    if kind not in _END_CONDITION:
        raise BuildError(f"unknown duration kind {kind!r}")
    if kind == "time":
        seconds = duration.get("seconds")
        if not isinstance(seconds, (int, float)) or seconds <= 0:
            raise BuildError("time duration needs seconds > 0")
        return _END_CONDITION[kind], float(seconds)
    if kind == "distance":
        meters = duration.get("meters")
        if not isinstance(meters, (int, float)) or meters <= 0:
            raise BuildError("distance duration needs meters > 0")
        return _END_CONDITION[kind], float(meters)
    # lap_button / open carry no value.
    return _END_CONDITION[kind], None


def _target_type_and_values(target: dict[str, Any]) -> tuple[int, str, dict[str, Any]]:
    """Resolve a target kind to its Garmin (typeId, typeKey) and the unprefixed
    value fields (zoneNumber / targetValueOne / targetValueTwo). Shared by the
    primary (_target) and secondary (_secondary_target) emitters."""
    kind = target.get("kind", "none")
    if kind not in _TARGET_TYPE:
        raise BuildError(f"unknown target kind {kind!r}")
    type_id, type_key = _TARGET_TYPE[kind]
    values: dict[str, Any] = {}

    if kind in ("hr_zone", "power_zone"):
        # Zone-by-number when a single zone (low==high); otherwise a value range.
        low, high = target.get("low"), target.get("high")
        if low is not None and high is not None and low == high:
            values["zoneNumber"] = low
        else:
            values["targetValueOne"] = low
            values["targetValueTwo"] = high
    elif kind in ("hr_bpm", "power_w", "cadence"):
        values["targetValueOne"] = target.get("low")
        values["targetValueTwo"] = target.get("high")
    elif kind == "pace":
        # Garmin pace targets are metres/second; convert from sec/km.
        values["targetValueOne"] = _pace_mps(target.get("low_sec_per_km"))
        values["targetValueTwo"] = _pace_mps(target.get("high_sec_per_km"))
    elif kind == "swim_pace":
        # Same Garmin m/s pace gate, but swim pace is sec/100m: 100/sec_per_100m.
        values["targetValueOne"] = _swim_pace_mps(target.get("low_sec_per_100m"))
        values["targetValueTwo"] = _swim_pace_mps(target.get("high_sec_per_100m"))
    return type_id, type_key, values


def _target(target: dict[str, Any]) -> dict[str, Any]:
    type_id, type_key, values = _target_type_and_values(target)
    out: dict[str, Any] = {"targetType": {"workoutTargetTypeId": type_id, "workoutTargetTypeKey": type_key}}
    out.update(values)
    return out


def _secondary_target(target: dict[str, Any]) -> dict[str, Any]:
    """Garmin's second simultaneous target for a (bike) step: the same type and
    value logic as _target, under the `secondary*` field names the
    ExecutableStepDTO uses (secondaryTargetType / secondaryZoneNumber /
    secondaryTargetValueOne / secondaryTargetValueTwo)."""
    type_id, type_key, values = _target_type_and_values(target)
    out: dict[str, Any] = {
        "secondaryTargetType": {"workoutTargetTypeId": type_id, "workoutTargetTypeKey": type_key}
    }
    for k, v in values.items():
        out["secondary" + k[0].upper() + k[1:]] = v
    return out


def _pace_mps(sec_per_km: float | None) -> float | None:
    if not sec_per_km or sec_per_km <= 0:
        return None
    return 1000.0 / sec_per_km


def _swim_pace_mps(sec_per_100m: float | None) -> float | None:
    if not sec_per_100m or sec_per_100m <= 0:
        return None
    return 100.0 / sec_per_100m
