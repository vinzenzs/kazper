import { Group } from "@visx/group";
import { scaleLinear } from "@visx/scale";
import { AxisBottom, AxisLeft } from "@visx/axis";

import type { QuadrantResult } from "../api/types";
import { num } from "../lib/format";

const W = 480;
const H = 320;
const MARGIN = { top: 12, right: 12, bottom: 34, left: 44 };

// The force/velocity quadrant scatter: each pedaling second plotted as CPV (m/s,
// x) vs AEPF (N, y), with the reference point's cross-hairs splitting the four
// Coggan quadrants, plus a shares legend. Rendered only when the scatter is
// present (the caller gates on power+cadence + a CP fit).
export function QuadrantScatter({ result }: { result: QuadrantResult }) {
  const pts = result.scatter ?? [];
  const s = result.summary;
  const innerW = W - MARGIN.left - MARGIN.right;
  const innerH = H - MARGIN.top - MARGIN.bottom;

  const maxCPV = Math.max(s.cpv_ref_mps * 1.6, ...pts.map((p) => p.cpv_mps), 0.1);
  const maxAEPF = Math.max(s.aepf_ref_n * 1.6, ...pts.map((p) => p.aepf_n), 1);
  const xScale = scaleLinear<number>({ domain: [0, maxCPV], range: [0, innerW], nice: true });
  const yScale = scaleLinear<number>({ domain: [0, maxAEPF], range: [innerH, 0], nice: true });
  const refX = xScale(s.cpv_ref_mps);
  const refY = yScale(s.aepf_ref_n);

  return (
    <div className="flex flex-col gap-3">
      <div className="grid grid-cols-2 gap-2 text-xs sm:grid-cols-4" data-testid="quadrant-shares">
        <Share label="Q1 force+speed" pct={s.q1_pct} />
        <Share label="Q2 grinding" pct={s.q2_pct} />
        <Share label="Q3 easy" pct={s.q3_pct} />
        <Share label="Q4 spinning" pct={s.q4_pct} />
      </div>

      <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="Quadrant analysis">
        <Group left={MARGIN.left} top={MARGIN.top}>
          {/* Reference cross-hairs. */}
          <line x1={refX} x2={refX} y1={0} y2={innerH} stroke="#64748b" strokeDasharray="3,3" />
          <line x1={0} x2={innerW} y1={refY} y2={refY} stroke="#64748b" strokeDasharray="3,3" />
          {pts.map((p, i) => (
            <circle
              key={i}
              cx={xScale(p.cpv_mps)}
              cy={yScale(p.aepf_n)}
              r={1.4}
              fill="#38bdf8"
              fillOpacity={0.35}
            />
          ))}
          <AxisLeft
            scale={yScale}
            numTicks={4}
            stroke="#3a4658"
            tickStroke="#3a4658"
            tickLabelProps={() => ({ fill: "#94a3b8", fontSize: 9, dx: -2, dy: 3 })}
          />
          <AxisBottom
            top={innerH}
            scale={xScale}
            numTicks={5}
            stroke="#3a4658"
            tickStroke="#3a4658"
            tickLabelProps={() => ({ fill: "#94a3b8", fontSize: 9, textAnchor: "middle" })}
          />
        </Group>
      </svg>
      <div className="text-xs text-slate-500">
        AEPF (N, force) vs CPV (m/s, pedal speed) — reference {num(s.aepf_ref_n, 0)} N @{" "}
        {num(s.cpv_ref_mps, 2)} m/s (CP {num(result.params.cp_watts, 0)} W, {num(result.params.cadence_rpm, 0)} rpm).
        {s.excluded_s > 0 && ` ${s.excluded_s}s coasting excluded.`}
      </div>
    </div>
  );
}

function Share({ label, pct }: { label: string; pct: number }) {
  return (
    <div className="rounded-md bg-slate-800/50 px-2 py-1">
      <div className="text-slate-400">{label}</div>
      <div className="text-sm font-semibold text-slate-100">{num(pct, 1)}%</div>
    </div>
  );
}
