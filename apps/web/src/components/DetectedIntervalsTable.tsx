import type { IntervalsResult, IntervalRest } from "../api/types";
import { num } from "../lib/format";

// The detected-intervals table: the work efforts found in an unstructured ride's
// power stream, with the derived Otsu threshold and a rest column. Shown only
// when detection returns ≥ 1 interval (the caller gates on that).
export function DetectedIntervalsTable({ result }: { result: IntervalsResult }) {
  const restByAfter = new Map<number, IntervalRest>(
    result.rests.map((r) => [r.after_n, r]),
  );
  const s = result.summary;

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-4 text-sm text-slate-400">
        <span>
          <span className="font-semibold text-slate-100">{s.count}</span> efforts
        </span>
        <span>
          mean{" "}
          <span className="font-semibold text-slate-100">{mmss(s.mean_effort_s)}</span> @{" "}
          <span className="font-semibold text-slate-100">{num(s.mean_effort_w, 0)} W</span>
        </span>
        {result.threshold_w !== null && (
          <span>threshold {num(result.threshold_w, 0)} W</span>
        )}
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-sm" data-testid="intervals-table">
          <thead>
            <tr className="text-left text-xs uppercase tracking-wide text-slate-500">
              <th className="py-1 pr-3">#</th>
              <th className="py-1 pr-3">Duration</th>
              <th className="py-1 pr-3">Avg W</th>
              <th className="py-1 pr-3">Max W</th>
              <th className="py-1 pr-3">kJ</th>
              <th className="py-1 pr-3">Rest after</th>
            </tr>
          </thead>
          <tbody>
            {result.intervals.map((iv) => {
              const rest = restByAfter.get(iv.n);
              return (
                <tr key={iv.n} className="border-t border-slate-800">
                  <td className="py-1 pr-3 text-slate-400">{iv.n}</td>
                  <td className="py-1 pr-3 font-semibold text-slate-100">{mmss(iv.duration_s)}</td>
                  <td className="py-1 pr-3">{num(iv.avg_w, 0)}</td>
                  <td className="py-1 pr-3 text-slate-400">{num(iv.max_w, 0)}</td>
                  <td className="py-1 pr-3 text-slate-400">{num(iv.kj, 1)}</td>
                  <td className="py-1 pr-3 text-slate-400">
                    {rest ? `${mmss(rest.duration_s)} @ ${num(rest.avg_w, 0)} W` : "—"}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <div className="text-xs text-slate-500">
        Detected from the power stream (Otsu work/rest split) — advisory, not planned structure.
      </div>
    </div>
  );
}

function mmss(secs: number): string {
  const m = Math.floor(secs / 60);
  const s = Math.round(secs % 60);
  return `${m}:${String(s).padStart(2, "0")}`;
}
