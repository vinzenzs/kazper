import { Stat } from "./Stat";
import type { WorkoutStatsBucket } from "../api/types";
import { duration, km, num, titleCase } from "../lib/format";

// The volume totals for the selected window: distance, time (elapsed, not moving
// time — the backend stores no moving-time field), elevation, activity count,
// and a by-sport count breakdown.
export function StatsTotals({ total }: { total: WorkoutStatsBucket }) {
  const sports = Object.entries(total.by_sport ?? {}).filter(([, n]) => n > 0);
  return (
    <div className="flex flex-col gap-3">
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <Stat label="Distance" value={km(total.total_distance_m)} unit="km" />
        <Stat label="Time" value={duration(total.total_duration_min)} />
        <Stat label="Elevation" value={num(total.total_elevation_gain_m, 0)} unit="m" />
        <Stat label="Activities" value={num(total.count, 0)} />
      </div>
      {sports.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {sports.map(([sport, n]) => (
            <span
              key={sport}
              className="rounded-full bg-ink-700/70 px-2 py-0.5 text-[11px] text-slate-300"
            >
              {titleCase(sport)} ×{n}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
