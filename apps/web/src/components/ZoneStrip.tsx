import { PLACEHOLDER } from "../lib/format";

// Zone colors run cool → hot (Z1 easy … Z5 max), reusing the accent palette.
const ZONE_COLORS = ["#38bdf8", "#34d399", "#fbbf24", "#fb923c", "#f87171"];

interface ZoneStripProps {
  label: string;
  unit: string;
  // The five cumulative zone-max boundaries (z1_max … z5_max); each nullable.
  zones: (number | null | undefined)[];
}

interface Band {
  index: number;
  from: number;
  to: number;
  color: string;
}

// A horizontal banded Z1–Z5 strip. Each band spans (previous boundary, this
// boundary]; width is proportional to the band's range so the strip reflects how
// the zones are actually sized. Only bands whose boundary is present are drawn —
// missing inner boundaries are never inferred. The previous boundary for a band
// is the nearest lower present boundary (0 for the first present zone).
export function ZoneStrip({ label, unit, zones }: ZoneStripProps) {
  const bands: Band[] = [];
  let prev = 0;
  for (let i = 0; i < zones.length; i++) {
    const v = zones[i];
    if (v === null || v === undefined || Number.isNaN(v)) continue;
    if (v > prev) {
      bands.push({ index: i, from: prev, to: v, color: ZONE_COLORS[i] ?? "#64748b" });
      prev = v;
    }
  }

  if (bands.length === 0) {
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
            key={b.index}
            className="flex min-w-10 flex-col items-center justify-center py-1.5 text-center"
            style={{
              flexGrow: Math.max(1, b.to - b.from),
              flexBasis: 0,
              backgroundColor: `${b.color}26`, // ~15% alpha fill
              borderTop: `2px solid ${b.color}`,
            }}
            title={`Z${b.index + 1}: ${b.from}–${b.to} ${unit}`}
          >
            <span className="text-[10px] font-semibold uppercase text-slate-300">
              Z{b.index + 1}
            </span>
            <span className="text-xs font-semibold tabular-nums text-slate-100">
              ≤{b.to}
            </span>
          </div>
        ))}
        <span className="self-center pl-2 text-[10px] uppercase tracking-wide text-slate-500">
          {unit}
        </span>
      </div>
    </div>
  );
}
