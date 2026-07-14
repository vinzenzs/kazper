import { Link, useParams } from "react-router-dom";

import { Panel } from "../components/Panel";
import { Stat } from "../components/Stat";
import { SplitsTable } from "../components/SplitsTable";
import { ZoneTimeStrip } from "../components/ZoneTimeStrip";
import { WPrimeBalanceStrip } from "../components/WPrimeBalanceStrip";
import { useWorkout, useCPModel, useWPrimeBalance } from "../api/hooks";
import { ApiError } from "../api/client";
import type { Workout } from "../api/types";
import {
  PLACEHOLDER,
  duration,
  km,
  num,
  shortDate,
  titleCase,
  weekday,
} from "../lib/format";

// The /workouts/:id route: a single activity's detail — summary metrics, HR-zone
// time, and a per-lap splits table — from the single-get that carries the nested
// detail the list-shaped context payloads omit. An unknown id renders a
// not-found state; a workout with no splits omits that table gracefully.
export function WorkoutDetailView() {
  const { id } = useParams<{ id: string }>();
  const { data, isLoading, isError, error } = useWorkout(id);

  // W′-balance parameters come from the athlete's critical-power fit over a
  // trailing-90-day window (the same model the stats page shows). The W′bal
  // fetch stays disabled until that fit yields cp/W′; when the window can't
  // support a fit, or the workout has no power stream, the strip renders
  // nothing — it is supplementary detail here, not a primary metric.
  const cp = useCPModel(daysAgoISO(90), daysAgoISO(0));
  const wp = useWPrimeBalance(id, cp.data?.model?.cp_watts, cp.data?.model?.w_prime_kj);

  const notFound =
    isError && error instanceof ApiError && error.status === 404;

  if (notFound) {
    return (
      <Panel title="Workout" isEmpty emptyHint="Workout not found">
        <div />
      </Panel>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <Panel
        title={data ? summaryTitle(data) : "Workout"}
        isLoading={isLoading}
        isError={isError}
        error={error}
      >
        {data && <Summary workout={data} />}
      </Panel>

      {data && hasZoneTime(data) && (
        <Panel title="Time in HR zones">
          <ZoneTimeStrip
            label="HR"
            secs={[
              data.secs_in_zone_1,
              data.secs_in_zone_2,
              data.secs_in_zone_3,
              data.secs_in_zone_4,
              data.secs_in_zone_5,
            ]}
          />
        </Panel>
      )}

      {wp.data && (
        <Panel title="W′ balance">
          <WPrimeBalanceStrip result={wp.data} />
        </Panel>
      )}

      {data && (data.splits?.length ?? 0) > 0 && (
        <Panel title="Splits">
          <SplitsTable splits={data.splits ?? []} />
        </Panel>
      )}

      <Link
        to="/"
        className="text-xs uppercase tracking-wide text-accent hover:underline"
      >
        ← Back to dashboard
      </Link>
    </div>
  );
}

function summaryTitle(w: Workout): string {
  return w.name || titleCase(w.sport);
}

function Summary({ workout: w }: { workout: Workout }) {
  const min = durationMin(w.started_at, w.ended_at);
  return (
    <div className="flex flex-col gap-3">
      <div className="text-sm text-slate-400">
        {weekday(w.started_at)} {shortDate(w.started_at)} · {titleCase(w.sport)} ·{" "}
        {titleCase(w.status)}
      </div>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
        <Stat label="Duration" value={min === null ? PLACEHOLDER : duration(min)} />
        <Stat label="Distance" value={km(w.distance_m)} unit="km" />
        <Stat label="Elevation" value={num(w.elevation_gain_m, 0)} unit="m" />
        <Stat label="Kcal" value={num(w.kcal_burned, 0)} />
        <Stat label="Avg HR" value={num(w.avg_hr, 0)} unit="bpm" />
        <Stat label="Max HR" value={num(w.max_hr, 0)} unit="bpm" />
        <Stat label="Avg power" value={num(w.avg_power_w, 0)} unit="W" />
        <Stat label="NP" value={num(w.normalized_power_w, 0)} unit="W" />
        <Stat label="IF" value={num(w.intensity_factor, 2)} />
        <Stat label="Cadence" value={num(w.avg_cadence, 0)} />
        <Stat label="TSS" value={num(w.tss, 0)} />
      </div>
    </div>
  );
}

function hasZoneTime(w: Workout): boolean {
  return [
    w.secs_in_zone_1,
    w.secs_in_zone_2,
    w.secs_in_zone_3,
    w.secs_in_zone_4,
    w.secs_in_zone_5,
  ].some((v) => (v ?? 0) > 0);
}

// daysAgoISO returns a YYYY-MM-DD string n days before today, in local time —
// used to frame the trailing-90-day critical-power window.
function daysAgoISO(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return d.toISOString().slice(0, 10);
}

// Elapsed minutes between two ISO timestamps; null when either is unparseable.
function durationMin(start: string, end: string): number | null {
  const s = new Date(start).getTime();
  const e = new Date(end).getTime();
  if (Number.isNaN(s) || Number.isNaN(e)) return null;
  return (e - s) / 60000;
}
