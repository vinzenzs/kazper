import { PLACEHOLDER, clock } from "../lib/format";
import { ZONE_COLORS } from "./ZoneStrip";

interface ZoneTimeStripProps {
  label: string;
  // Seconds in each of the five HR zones (z1…z5); each nullable.
  secs: (number | null | undefined)[];
}

// A proportional stacked bar of time-in-zone, sharing the ZoneStrip palette but
// with time (not boundary) semantics: each segment's width is that zone's share
// of total measured seconds. Only zones with a positive value are drawn.
export function ZoneTimeStrip({ label, secs }: ZoneTimeStripProps) {
  const bands = secs
    .map((v, i) => ({ i, v: v ?? 0 }))
    .filter((b) => b.v > 0);
  const total = bands.reduce((s, b) => s + b.v, 0);

  if (total === 0) {
    return (
      <div className="flex items-center gap-3">
        <span className="w-12 shrink-0 text-xs uppercase tracking-wide text-slate-400">
          {label}
        </span>
        <span className="text-sm text-slate-500">{PLACEHOLDER}</span>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-3">
      <span className="w-12 shrink-0 text-xs uppercase tracking-wide text-slate-400">
        {label}
      </span>
      <div className="flex flex-1 overflow-hidden rounded-md">
        {bands.map((b) => (
          <div
            key={b.i}
            className="flex flex-col items-center justify-center py-1.5 text-center"
            style={{
              flexGrow: b.v,
              flexBasis: 0,
              backgroundColor: `${ZONE_COLORS[b.i] ?? "#64748b"}26`, // ~15% alpha
              borderTop: `2px solid ${ZONE_COLORS[b.i] ?? "#64748b"}`,
            }}
            title={`Z${b.i + 1}: ${clock(b.v)}`}
          >
            <span className="text-[10px] font-semibold uppercase text-slate-300">
              Z{b.i + 1}
            </span>
            <span className="text-xs font-semibold tabular-nums text-slate-100">
              {clock(b.v)}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
