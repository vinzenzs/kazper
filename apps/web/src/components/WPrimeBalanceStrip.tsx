import { Group } from "@visx/group";
import { scaleLinear } from "@visx/scale";
import { LinePath, AreaClosed } from "@visx/shape";
import { curveMonotoneX } from "@visx/curve";

import type { WPrimeBalanceResult } from "../api/types";
import { num } from "../lib/format";

const W = 560;
const H = 160;
const MARGIN = { top: 10, right: 12, bottom: 18, left: 36 };

// The W′-balance strip: the anaerobic battery draining and recharging across the
// ride, with the minimum marked, plus a summary readout. Downsampled series;
// the exact minimum comes from the summary. A negative floor (depletion > 100%)
// is drawn below the zero line — the "you went past your modeled W′" signal.
export function WPrimeBalanceStrip({ result }: { result: WPrimeBalanceResult }) {
  const series = result.series ?? [];
  const { summary } = result;
  const innerW = W - MARGIN.left - MARGIN.right;
  const innerH = H - MARGIN.top - MARGIN.bottom;

  const wPrime = result.params.w_prime_kj;
  const lo = Math.min(0, summary.min_w_prime_kj);
  const xScale = scaleLinear<number>({
    domain: [0, Math.max(1, series.length - 1)],
    range: [0, innerW],
  });
  const yScale = scaleLinear<number>({
    domain: [lo, Math.max(wPrime, ...series, 0)],
    range: [innerH, 0],
    nice: true,
  });

  // Index of the plotted minimum (from the summary's full-resolution second,
  // mapped onto the downsampled x-axis).
  const minIdx =
    result.duration_s > 0
      ? Math.round((summary.min_at_s / result.duration_s) * (series.length - 1))
      : 0;

  return (
    <div className="flex flex-col gap-3">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4" data-testid="wprime-readout">
        <Readout label="Min W′" value={`${num(summary.min_w_prime_kj, 1)} kJ`} />
        <Readout label="Max depletion" value={`${num(summary.max_depletion_pct, 0)}%`} />
        <Readout label="End W′" value={`${num(summary.end_w_prime_kj, 1)} kJ`} />
        <Readout label="Below 25%" value={fmtMinSec(summary.time_below_25_pct_s)} />
      </div>
      {series.length > 1 && (
        <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="W-prime balance">
          <Group left={MARGIN.left} top={MARGIN.top}>
            {/* Zero line — anything below it is over-depletion. */}
            <line x1={0} x2={innerW} y1={yScale(0)} y2={yScale(0)} stroke="#3a4658" strokeDasharray="2,3" />
            <AreaClosed<number>
              data={series}
              x={(_, i) => xScale(i)}
              y={(d) => yScale(d)}
              yScale={yScale}
              curve={curveMonotoneX}
              fill="#38bdf8"
              fillOpacity={0.12}
            />
            <LinePath<number>
              data={series}
              x={(_, i) => xScale(i)}
              y={(d) => yScale(d)}
              stroke="#38bdf8"
              strokeWidth={1.75}
              curve={curveMonotoneX}
            />
            {series.length > 0 && (
              <circle
                data-testid="wprime-min-marker"
                cx={xScale(minIdx)}
                cy={yScale(summary.min_w_prime_kj)}
                r={3.5}
                fill="#f87171"
              >
                <title>Min {num(summary.min_w_prime_kj, 1)} kJ at {fmtMinSec(summary.min_at_s)}</title>
              </circle>
            )}
          </Group>
        </svg>
      )}
      <div className="text-xs text-slate-500">
        W′ balance over the ride (CP {num(result.params.cp_watts, 0)} W, W′ {num(wPrime, 1)} kJ).
        {summary.max_depletion_pct > 100
          ? " Depletion exceeded 100% — the modeled W′ may be stale; re-check the CP fit."
          : " Advisory, from the critical-power fit."}
      </div>
    </div>
  );
}

function Readout({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md bg-slate-800/50 px-3 py-2">
      <div className="text-xs text-slate-400">{label}</div>
      <div className="text-base font-semibold text-slate-100">{value}</div>
    </div>
  );
}

function fmtMinSec(s: number): string {
  const m = Math.floor(s / 60);
  const sec = s % 60;
  return m > 0 ? `${m}m ${sec}s` : `${sec}s`;
}
