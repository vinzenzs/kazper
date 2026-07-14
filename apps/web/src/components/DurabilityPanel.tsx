import type { DurabilityResult, DurabilityDuration } from "../api/types";
import { num } from "../lib/format";

const TIERS = [500, 1000, 1500, 2000];

const DURATION_LABEL: Record<number, string> = {
  60: "1 min",
  300: "5 min",
  1200: "20 min",
};

// The durability fade grid: rows are durations (1m/5m/20m), columns are the
// fresh best then each kJ tier — each tier cell shows the faded watts and the
// fade % vs fresh. Fade coloring deepens with the drop. The empty state points
// at the recompute backfill (historical rides gain tiers only after re-derive).
export function DurabilityPanel({ result }: { result: DurabilityResult }) {
  const hasTiers = result.durations.some((d) => d.tiers.length > 0);
  if (result.durations.length === 0 || !hasTiers) {
    return (
      <div className="py-6 text-center text-sm text-slate-500">
        No fatigue-resistance data yet — durability appears once long rides have
        been re-derived (recompute their streams to backfill kJ tiers).
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="overflow-x-auto">
        <table className="w-full text-sm" data-testid="durability-grid">
          <thead>
            <tr className="text-left text-xs uppercase tracking-wide text-slate-500">
              <th className="py-1 pr-3">Duration</th>
              <th className="py-1 pr-3">Fresh</th>
              {TIERS.map((t) => (
                <th key={t} className="py-1 pr-3">
                  {t / 1000}k kJ
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {result.durations.map((d) => (
              <DurabilityRow key={d.duration_s} col={d} />
            ))}
          </tbody>
        </table>
      </div>
      <div className="text-xs text-slate-500">
        Best power that survives after N kJ of work vs fresh — fatigue resistance.
        Fresh and tiered bests may come from different rides.
      </div>
    </div>
  );
}

function DurabilityRow({ col }: { col: DurabilityDuration }) {
  const byTier = new Map(col.tiers.map((t) => [t.kj_tier, t]));
  return (
    <tr className="border-t border-slate-800">
      <td className="py-1 pr-3 text-slate-300">{DURATION_LABEL[col.duration_s] ?? `${col.duration_s}s`}</td>
      <td className="py-1 pr-3 font-semibold text-slate-100">
        {col.fresh ? `${num(col.fresh.watts, 0)} W` : "—"}
      </td>
      {TIERS.map((t) => {
        const tier = byTier.get(t);
        if (!tier) {
          return (
            <td key={t} className="py-1 pr-3 text-slate-600">
              —
            </td>
          );
        }
        return (
          <td key={t} className="py-1 pr-3">
            <span className="text-slate-200">{num(tier.watts, 0)}</span>{" "}
            <span className={fadeClass(tier.fade_pct)}>
              (−{num(tier.fade_pct, 1)}%)
            </span>
          </td>
        );
      })}
    </tr>
  );
}

// Deeper fade → warmer color.
function fadeClass(fadePct: number): string {
  if (fadePct >= 15) return "text-xs text-red-400";
  if (fadePct >= 7) return "text-xs text-amber-400";
  return "text-xs text-slate-400";
}
