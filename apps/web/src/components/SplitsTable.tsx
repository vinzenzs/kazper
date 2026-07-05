import type { Split } from "../api/types";
import { PLACEHOLDER, clock, km, num, pace } from "../lib/format";

// A per-lap splits table. Rendered only when a workout has splits; the parent
// omits it gracefully otherwise. Pace is derived from avg_speed_mps.
export function SplitsTable({ splits }: { splits: Split[] }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-left text-[11px] uppercase tracking-wide text-slate-500">
            <th className="pb-2 font-medium">#</th>
            <th className="pb-2 text-right font-medium">Dist</th>
            <th className="pb-2 text-right font-medium">Time</th>
            <th className="pb-2 text-right font-medium">Pace</th>
            <th className="pb-2 text-right font-medium">HR</th>
            <th className="pb-2 text-right font-medium">Power</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-ink-600/50">
          {splits.map((s) => (
            <tr key={s.split_index} className="tabular-nums">
              <td className="py-1.5 pr-3 text-slate-400">{s.split_index + 1}</td>
              <td className="py-1.5 pl-3 text-right text-slate-200">
                {km(s.distance_m)}
              </td>
              <td className="py-1.5 pl-3 text-right text-slate-200">
                {clock(s.duration_s)}
              </td>
              <td className="py-1.5 pl-3 text-right text-slate-200">
                {paceFromSpeed(s.avg_speed_mps)}
              </td>
              <td className="py-1.5 pl-3 text-right text-slate-300">
                {num(s.avg_hr, 0)}
              </td>
              <td className="py-1.5 pl-3 text-right text-slate-300">
                {num(s.avg_power_w, 0)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// Convert m/s to a "m:ss /km" pace; placeholder when speed is absent or zero.
function paceFromSpeed(mps: number | null | undefined): string {
  if (mps === null || mps === undefined || Number.isNaN(mps) || mps <= 0) {
    return PLACEHOLDER;
  }
  return pace(1000 / mps);
}
