import { Group } from "@visx/group";
import { scaleLinear, scaleLog } from "@visx/scale";
import { LinePath } from "@visx/shape";
import { AxisBottom, AxisLeft } from "@visx/axis";
import { GridRows } from "@visx/grid";
import { curveMonotoneX } from "@visx/curve";

import type { CPModelResult, CPPoint } from "../api/types";
import { durationLabel, num } from "../lib/format";

const W = 560;
const H = 240;
const MARGIN = { top: 12, right: 16, bottom: 30, left: 44 };

const REASON_TEXT: Record<string, string> = {
  insufficient_points:
    "Not enough long efforts in this window to estimate critical power.",
  span_too_narrow:
    "The efforts don't span a wide enough duration range to fit a model.",
};

// The critical-power panel: a CP / W′ / R² readout plus the in-band effort points
// with the fitted hyperbola P(t) = CP + W′/t on a log duration axis (the
// power-curve idiom). A null model renders its gate reason instead of a chart.
export function CPModelChart({ result }: { result: CPModelResult }) {
  const { model, points } = result;

  if (!model) {
    return (
      <div className="py-6 text-center text-sm text-slate-500">
        {(result.reason && REASON_TEXT[result.reason]) ??
          "No critical-power estimate for this window."}
      </div>
    );
  }

  const sorted = [...points].sort((a, b) => a.duration_s - b.duration_s);
  const innerW = W - MARGIN.left - MARGIN.right;
  const innerH = H - MARGIN.top - MARGIN.bottom;
  const wPrimeJ = model.w_prime_kj * 1000;

  const minD = sorted[0]?.duration_s ?? 120;
  const maxD = sorted[sorted.length - 1]?.duration_s ?? 1800;
  const xScale = scaleLog<number>({ domain: [minD, maxD], range: [0, innerW], base: 10 });
  const maxVal = Math.max(1, ...sorted.map((p) => p.watts));
  const yScale = scaleLinear<number>({ domain: [0, maxVal * 1.1], range: [innerH, 0], nice: true });

  // Sample the fitted model P(t) = CP + W′/t across the duration span.
  const steps = 40;
  const fitted = Array.from({ length: steps + 1 }, (_, i) => {
    const t = minD * Math.pow(maxD / minD, i / steps);
    return { t, p: model.cp_watts + wPrimeJ / t };
  });

  return (
    <div className="flex flex-col gap-3">
      <div className="grid grid-cols-3 gap-3" data-testid="cp-readout">
        <Readout label="Critical power" value={`${num(model.cp_watts, 0)} W`} />
        <Readout label="W′" value={`${num(model.w_prime_kj, 1)} kJ`} />
        <Readout label="Fit R²" value={num(model.r_squared, 2)} />
      </div>
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="Critical-power model">
        <Group left={MARGIN.left} top={MARGIN.top}>
          <GridRows scale={yScale} width={innerW} stroke="#26303f" strokeDasharray="2,3" />
          <AxisLeft
            scale={yScale}
            numTicks={4}
            stroke="#3a4658"
            tickStroke="#3a4658"
            tickLabelProps={() => ({ fill: "#94a3b8", fontSize: 10, dx: -2, dy: 3 })}
          />
          <AxisBottom
            top={innerH}
            scale={xScale}
            tickValues={sorted.map((p) => p.duration_s)}
            tickFormat={(v) => durationLabel(Number(v))}
            stroke="#3a4658"
            tickStroke="#3a4658"
            tickLabelProps={() => ({ fill: "#94a3b8", fontSize: 9, textAnchor: "middle" })}
          />
          {/* Fitted CP curve (muted), then the measured best-effort points. */}
          <LinePath<{ t: number; p: number }>
            data={fitted}
            x={(d) => xScale(d.t)}
            y={(d) => yScale(d.p)}
            stroke="#64748b"
            strokeWidth={1.5}
            strokeDasharray="4,3"
            curve={curveMonotoneX}
          />
          {sorted.map((p: CPPoint) => (
            <circle key={p.duration_s} cx={xScale(p.duration_s)} cy={yScale(p.watts)} r={3} fill="#38bdf8">
              <title>
                {durationLabel(p.duration_s)}: {num(p.watts, 0)} W · {p.date}
              </title>
            </circle>
          ))}
        </Group>
      </svg>
      <div className="text-xs text-slate-500">
        Fitted critical power vs. best-effort points (RMSE {num(model.rmse_w, 1)} W). Advisory — compare
        against your configured FTP.
      </div>
    </div>
  );
}

function Readout({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md bg-slate-800/50 px-3 py-2">
      <div className="text-xs text-slate-400">{label}</div>
      <div className="text-lg font-semibold text-slate-100">{value}</div>
    </div>
  );
}
