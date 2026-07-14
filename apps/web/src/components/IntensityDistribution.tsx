import type { IntensityDistribution, ZoneShare } from "../api/types";
import { ZONE_COLORS } from "./ZoneStrip";
import { num } from "../lib/format";

const CLASS_LABEL: Record<string, string> = {
  polarized: "Polarized",
  pyramidal: "Pyramidal",
  threshold: "Threshold",
  mixed: "Mixed",
};

// A horizontal stacked zone-share bar (Z1→Z5), width proportional to share_pct.
function ShareBar({ zones }: { zones: ZoneShare[] }) {
  return (
    <div
      className="flex h-3.5 w-full overflow-hidden rounded bg-ink-700/40"
      data-testid="zone-share-bar"
    >
      {zones.map((z) => (
        <div
          key={z.zone}
          style={{
            width: `${z.share_pct ?? 0}%`,
            backgroundColor: ZONE_COLORS[z.zone - 1] ?? "#64748b",
          }}
          title={`Z${z.zone} · ${num(z.share_pct ?? 0, 1)}%`}
        />
      ))}
    </div>
  );
}

// The window intensity distribution: classification badge + band shares, the
// total zone-share bar, and a per-week stacked-bar trend, with a muted note for
// sessions excluded for lacking HR zones.
export function IntensityDistributionPanel({ dist }: { dist: IntensityDistribution }) {
  const t = dist.total;
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        {t.classification && (
          <span
            className="rounded-full bg-ink-700/70 px-2 py-0.5 text-xs font-medium text-slate-200"
            data-testid="intensity-class"
          >
            {CLASS_LABEL[t.classification] ?? t.classification}
          </span>
        )}
        <span className="text-sm text-slate-400">
          low <b className="text-sky-300">{num(t.bands.low_pct, 1)}%</b> · mod{" "}
          <b className="text-amber-300">{num(t.bands.moderate_pct, 1)}%</b> · high{" "}
          <b className="text-orange-300">{num(t.bands.high_pct, 1)}%</b>
        </span>
      </div>

      <ShareBar zones={t.zones} />

      {dist.weekly.length > 0 && (
        <div className="flex flex-col gap-1.5">
          <div className="text-xs font-medium text-slate-500">Weekly</div>
          {dist.weekly.map((w) => (
            <div key={w.week_start} className="flex items-center gap-2 text-xs">
              <span className="w-12 shrink-0 text-slate-500">{w.week_start.slice(5)}</span>
              <div className="flex-1">
                <ShareBar zones={w.zones} />
              </div>
            </div>
          ))}
        </div>
      )}

      {dist.missing_zone_data_count > 0 && (
        <div className="text-xs text-amber-400/80" data-testid="intensity-missing">
          {dist.missing_zone_data_count} session
          {dist.missing_zone_data_count === 1 ? "" : "s"} without HR zones (excluded)
        </div>
      )}
    </div>
  );
}
