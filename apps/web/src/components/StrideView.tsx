import { Group } from "@visx/group";
import { scaleLinear } from "@visx/scale";
import { AxisBottom, AxisLeft } from "@visx/axis";
import { LinePath } from "@visx/shape";

import type { StrideResult } from "../api/types";
import { num } from "../lib/format";

const W = 480;
const H = 320;
const MARGIN = { top: 12, right: 44, bottom: 34, left: 44 };

// Cadence vs step length across a run's speed range — the limiter view.
//
// The two series are charted on their own axes against speed, because the
// question is WHERE each one plateaus: a step length that flat-lines above
// threshold pace while cadence keeps climbing is the whole finding, and it is
// visible here rather than asserted by the split. The split is a summary of
// this picture, never a replacement for it.
export function StrideView({ result }: { result: StrideResult }) {
  const bins = result.bins ?? [];
  const innerW = W - MARGIN.left - MARGIN.right;
  const innerH = H - MARGIN.top - MARGIN.bottom;

  const mid = (b: { speed_low_mps: number; speed_high_mps: number }) =>
    (b.speed_low_mps + b.speed_high_mps) / 2;

  const speeds = bins.map(mid);
  const xScale = scaleLinear<number>({
    domain: [Math.min(...speeds, 0), Math.max(...speeds, 1)],
    range: [0, innerW],
    nice: true,
  });
  const cadScale = scaleLinear<number>({
    domain: [Math.min(...bins.map((b) => b.cadence_spm), 140) - 5, Math.max(...bins.map((b) => b.cadence_spm), 180) + 5],
    range: [innerH, 0],
    nice: true,
  });
  const stepScale = scaleLinear<number>({
    domain: [0, Math.max(...bins.map((b) => b.step_length_m), 1.5) * 1.15],
    range: [innerH, 0],
    nice: true,
  });

  return (
    <div className="flex flex-col gap-3" data-testid="stride-view">
      {result.contribution ? (
        <div className="grid grid-cols-2 gap-2 text-xs" data-testid="stride-split">
          <Share label="from turnover" pct={result.contribution.cadence_contribution_pct} />
          <Share label="from step length" pct={result.contribution.step_contribution_pct} />
        </div>
      ) : (
        // A steady run genuinely can't answer the question — say so instead of
        // rendering an empty or invented split.
        <p className="text-xs text-slate-400" data-testid="stride-reason">
          {result.reason === "insufficient_speed_range"
            ? "Not enough pace variety in this run to split speed into turnover vs step length — the bins below are still the raw picture."
            : "No contribution split available for this run."}
        </p>
      )}

      <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="Cadence vs step length">
        <Group left={MARGIN.left} top={MARGIN.top}>
          <LinePath
            data={bins}
            x={(b) => xScale(mid(b))}
            y={(b) => cadScale(b.cadence_spm)}
            stroke="#38bdf8"
            strokeWidth={2}
          />
          <LinePath
            data={bins}
            x={(b) => xScale(mid(b))}
            y={(b) => stepScale(b.step_length_m)}
            stroke="#fbbf24"
            strokeWidth={2}
          />
          <AxisLeft scale={cadScale} numTicks={4} stroke="#38bdf8" tickStroke="#38bdf8"
            tickLabelProps={() => ({ fill: "#38bdf8", fontSize: 9, textAnchor: "end", dx: -2, dy: 3 })} />
          <AxisBottom
            top={innerH}
            scale={xScale}
            numTicks={5}
            stroke="#475569"
            tickStroke="#475569"
            tickLabelProps={() => ({ fill: "#94a3b8", fontSize: 9, textAnchor: "middle" })}
          />
        </Group>
      </svg>

      <div className="flex flex-wrap gap-3 text-[11px] text-slate-400">
        <span><span className="text-sky-400">—</span> cadence (spm)</span>
        <span><span className="text-amber-400">—</span> step length (m)</span>
        <span>speed (m/s) →</span>
        <span>{num(result.analyzed_s)} s analyzed, {num(result.excluded_s)} s excluded</span>
      </div>
    </div>
  );
}

function Share({ label, pct }: { label: string; pct: number }) {
  return (
    <div className="rounded bg-slate-800/60 px-2 py-1">
      <div className="text-slate-400">{label}</div>
      {/* 1dp: the backend rounds the split to a decimal and a 31 vs 31.4 is a
          real difference when comparing two runs. */}
      <div className="font-mono text-sm text-slate-100">{num(pct, 1)}%</div>
    </div>
  );
}
