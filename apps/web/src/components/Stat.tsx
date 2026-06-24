import { PLACEHOLDER } from "../lib/format";

// A single labeled metric tile. Shared across the stat-grid panels (recovery,
// fitness, race predictions, power/thresholds). `value` is already formatted;
// the unit is hidden when the value is the em-dash placeholder.
export function Stat({
  label,
  value,
  unit,
}: {
  label: string;
  value: string;
  unit?: string;
}) {
  return (
    <div className="rounded-lg bg-ink-700/60 px-3 py-2">
      <div className="text-xs uppercase tracking-wide text-slate-400">{label}</div>
      <div className="mt-0.5 text-xl font-semibold tabular-nums text-slate-100">
        {value}
        {unit && value !== PLACEHOLDER && (
          <span className="ml-1 text-xs text-slate-400">{unit}</span>
        )}
      </div>
    </div>
  );
}
