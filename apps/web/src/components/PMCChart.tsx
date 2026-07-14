import { Group } from "@visx/group";
import { scaleLinear } from "@visx/scale";
import { LinePath } from "@visx/shape";
import { AxisBottom, AxisLeft } from "@visx/axis";
import { GridRows } from "@visx/grid";
import { curveMonotoneX } from "@visx/curve";

import type { PMCDay, PMCSeries } from "../api/types";
import { num } from "../lib/format";

const W = 560;
const H = 260;
const MARGIN = { top: 12, right: 44, bottom: 30, left: 44 };

// The Performance Management Chart: CTL (fitness) and ATL (fatigue) as lines on
// the left load axis, TSB (form) as bars around a zero baseline on the right
// axis (positive = fresh, negative = fatigued), and unsafe-ramp weeks shaded on
// the CTL trace. Matches the LoadTrend / PowerCurve visx idiom.
export function PMCChart({ series }: { series: PMCSeries }) {
  const days = series.days;
  const innerW = W - MARGIN.left - MARGIN.right;
  const innerH = H - MARGIN.top - MARGIN.bottom;

  const n = days.length;
  const xScale = scaleLinear<number>({
    domain: [0, Math.max(1, n - 1)],
    range: [0, innerW],
  });

  const loadMax = Math.max(1, ...days.map((d) => Math.max(d.ctl, d.atl)));
  const loadScale = scaleLinear<number>({
    domain: [0, loadMax * 1.1],
    range: [innerH, 0],
    nice: true,
  });

  const tsbAbs = Math.max(1, ...days.map((d) => Math.abs(d.tsb)));
  const tsbScale = scaleLinear<number>({
    domain: [-tsbAbs * 1.1, tsbAbs * 1.1],
    range: [innerH, 0],
    nice: true,
  });
  const tsbZero = tsbScale(0);

  const idxByDate = new Map(days.map((d, i) => [d.date, i]));
  const dateIdx = (date: string): number | undefined => idxByDate.get(date);

  const line = (pick: (d: PMCDay) => number) =>
    days.map((d, i) => ({ x: xScale(i), y: loadScale(pick(d)) }));

  // Evenly-spaced date ticks (first, mid, last) to keep the axis readable.
  const tickIdx = n <= 1 ? [0] : [0, Math.floor((n - 1) / 2), n - 1];

  return (
    <svg
      viewBox={`0 0 ${W} ${H}`}
      className="w-full"
      role="img"
      aria-label="Performance management chart"
    >
      <Group left={MARGIN.left} top={MARGIN.top}>
        <GridRows scale={loadScale} width={innerW} stroke="#26303f" strokeDasharray="2,3" />

        {/* Ramp-alert weeks: shaded vertical bands behind the traces. */}
        {series.ramp_alerts.map((a) => {
          const start = dateIdx(a.week_start);
          if (start === undefined) return null;
          const endIdx = Math.min(n - 1, start + 6);
          const x0 = xScale(start);
          const x1 = xScale(endIdx);
          return (
            <rect
              key={a.week_start}
              x={x0}
              y={0}
              width={Math.max(2, x1 - x0)}
              height={innerH}
              fill="#f87171"
              opacity={0.12}
              data-testid="ramp-band"
            />
          );
        })}

        {/* TSB bars around the zero baseline. */}
        {days.map((d, i) => {
          const y = tsbScale(d.tsb);
          const top = Math.min(y, tsbZero);
          const h = Math.abs(y - tsbZero);
          return (
            <rect
              key={d.date}
              x={xScale(i) - 1}
              y={top}
              width={2}
              height={h}
              fill={d.tsb >= 0 ? "#4ade80" : "#fb923c"}
              opacity={0.5}
            />
          );
        })}
        <line x1={0} x2={innerW} y1={tsbZero} y2={tsbZero} stroke="#3a4658" strokeDasharray="3,3" />

        {/* ATL (fatigue) muted, CTL (fitness) accent on top. */}
        <LinePath data={line((d) => d.atl)} x={(p) => p.x} y={(p) => p.y} stroke="#f472b6" strokeWidth={1.5} curve={curveMonotoneX} />
        <LinePath data={line((d) => d.ctl)} x={(p) => p.x} y={(p) => p.y} stroke="#38bdf8" strokeWidth={2} curve={curveMonotoneX} />

        <AxisLeft
          scale={loadScale}
          numTicks={4}
          stroke="#3a4658"
          tickStroke="#3a4658"
          tickLabelProps={() => ({ fill: "#94a3b8", fontSize: 10, dx: -2, dy: 3 })}
        />
        <AxisBottom
          top={innerH}
          scale={xScale}
          tickValues={tickIdx}
          tickFormat={(v) => days[Number(v)]?.date.slice(5) ?? ""}
          stroke="#3a4658"
          tickStroke="#3a4658"
          tickLabelProps={() => ({ fill: "#94a3b8", fontSize: 9, textAnchor: "middle" })}
        />
      </Group>

      {/* Legend row (top-left). */}
      <g transform={`translate(${MARGIN.left},${MARGIN.top - 2})`} fontSize={9} fill="#94a3b8">
        <text x={0} y={0}>
          <tspan fill="#38bdf8">— CTL</tspan> <tspan fill="#f472b6">— ATL</tspan>{" "}
          <tspan fill="#4ade80">▮ TSB</tspan>
        </text>
      </g>
    </svg>
  );
}

// A compact fitness/fatigue/form readout for the latest day in the series.
export function PMCSummary({ series }: { series: PMCSeries }) {
  const last = series.days[series.days.length - 1];
  if (!last) return null;
  return (
    <div className="flex flex-wrap gap-4 text-sm">
      <span className="text-slate-400">
        CTL <span className="font-semibold text-sky-300">{num(last.ctl, 1)}</span>
      </span>
      <span className="text-slate-400">
        ATL <span className="font-semibold text-pink-300">{num(last.atl, 1)}</span>
      </span>
      <span className="text-slate-400">
        TSB{" "}
        <span className={`font-semibold ${last.tsb >= 0 ? "text-emerald-300" : "text-orange-300"}`}>
          {num(last.tsb, 1)}
        </span>
      </span>
      {series.missing_tss_workouts > 0 && (
        <span className="text-amber-400" data-testid="pmc-missing">
          {series.missing_tss_workouts} session
          {series.missing_tss_workouts === 1 ? "" : "s"} without TSS
        </span>
      )}
    </div>
  );
}
