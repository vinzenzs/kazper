import { Group } from "@visx/group";
import { scaleLinear, scaleLog } from "@visx/scale";
import { LinePath } from "@visx/shape";
import { AxisBottom, AxisLeft } from "@visx/axis";
import { GridRows } from "@visx/grid";
import { curveMonotoneX } from "@visx/curve";

import type { CurvePoint, PowerCurve } from "../api/types";
import { durationLabel, num, pace } from "../lib/format";

const W = 560;
const H = 240;
const MARGIN = { top: 12, right: 16, bottom: 30, left: 44 };

// A mean-maximal curve: best value vs duration on a logarithmic duration axis.
// Power (W) plots directly; speed (m/s) also plots as m/s ascending (higher is
// better), with pace shown in the point tooltip. Matches the LoadTrend visx
// idiom (muted grid, cool accent line).
export function PowerCurveChart({ curve }: { curve: PowerCurve }) {
  const points = [...curve.points].sort((a, b) => a.duration_s - b.duration_s);
  const innerW = W - MARGIN.left - MARGIN.right;
  const innerH = H - MARGIN.top - MARGIN.bottom;

  const isPower = curve.metric === "power";
  const unit = isPower ? "W" : "m/s";

  const xScale = scaleLog<number>({
    domain: [points[0].duration_s, points[points.length - 1].duration_s],
    range: [0, innerW],
    base: 10,
  });
  const maxVal = Math.max(1, ...points.map((p) => p.value));
  const yScale = scaleLinear<number>({
    domain: [0, maxVal * 1.1],
    range: [innerH, 0],
    nice: true,
  });

  return (
    <div>
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="Power/pace curve">
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
            tickValues={points.map((p) => p.duration_s)}
            tickFormat={(v) => durationLabel(Number(v))}
            stroke="#3a4658"
            tickStroke="#3a4658"
            tickLabelProps={() => ({ fill: "#94a3b8", fontSize: 9, textAnchor: "middle" })}
          />
          <LinePath<CurvePoint>
            data={points}
            x={(p) => xScale(p.duration_s)}
            y={(p) => yScale(p.value)}
            stroke="#38bdf8"
            strokeWidth={2}
            curve={curveMonotoneX}
          />
          {points.map((p) => (
            <circle
              key={p.duration_s}
              cx={xScale(p.duration_s)}
              cy={yScale(p.value)}
              r={3}
              fill="#38bdf8"
            >
              <title>
                {durationLabel(p.duration_s)}: {num(p.value, isPower ? 0 : 2)} {unit}
                {isPower ? "" : ` (${pace(1000 / p.value)})`} · {p.date}
              </title>
            </circle>
          ))}
        </Group>
      </svg>
      <div className="mt-1 text-xs text-slate-500">
        Best {isPower ? "power" : "pace"} by duration ({unit})
      </div>
    </div>
  );
}
