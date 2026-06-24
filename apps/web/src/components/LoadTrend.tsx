import { Group } from "@visx/group";
import { scaleLinear, scaleTime } from "@visx/scale";
import { LinePath } from "@visx/shape";
import { AxisBottom, AxisLeft } from "@visx/axis";
import { GridRows } from "@visx/grid";
import { curveMonotoneX } from "@visx/curve";

import { Panel } from "./Panel";
import type { FitnessSnapshot } from "../api/types";

interface LoadTrendProps {
  metrics: FitnessSnapshot[] | undefined;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
}

interface Point {
  date: Date;
  acute: number | null;
  chronic: number | null;
}

const W = 560;
const H = 220;
const MARGIN = { top: 12, right: 16, bottom: 28, left: 36 };

export function LoadTrend({ metrics, isLoading, isError, error }: LoadTrendProps) {
  const points: Point[] = (metrics ?? [])
    .map((m) => ({
      date: new Date(m.date),
      acute: m.acute_load ?? null,
      chronic: m.chronic_load ?? null,
    }))
    .filter((p) => !Number.isNaN(p.date.getTime()))
    .sort((a, b) => a.date.getTime() - b.date.getTime());

  const hasData = points.some((p) => p.acute !== null || p.chronic !== null);

  return (
    <Panel
      title="Training load · acute vs chronic"
      isLoading={isLoading}
      isError={isError}
      error={error}
      isEmpty={!hasData}
      emptyHint="No load history in window"
    >
      {hasData && <Chart points={points} />}
    </Panel>
  );
}

function Chart({ points }: { points: Point[] }) {
  const innerW = W - MARGIN.left - MARGIN.right;
  const innerH = H - MARGIN.top - MARGIN.bottom;

  const xScale = scaleTime({
    domain: [points[0].date, points[points.length - 1].date],
    range: [0, innerW],
  });

  const maxLoad = Math.max(
    1,
    ...points.flatMap((p) => [p.acute ?? 0, p.chronic ?? 0]),
  );
  const yScale = scaleLinear({
    domain: [0, maxLoad * 1.1],
    range: [innerH, 0],
    nice: true,
  });

  const defined = (key: "acute" | "chronic") => (p: Point) => p[key] !== null;
  const yOf = (key: "acute" | "chronic") => (p: Point) => yScale(p[key] ?? 0);

  return (
    <div>
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="Training load trend">
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
            numTicks={5}
            stroke="#3a4658"
            tickStroke="#3a4658"
            tickLabelProps={() => ({ fill: "#94a3b8", fontSize: 10, textAnchor: "middle" })}
          />
          <LinePath<Point>
            data={points.filter(defined("chronic"))}
            x={(p) => xScale(p.date)}
            y={yOf("chronic")}
            stroke="#38bdf8"
            strokeWidth={2}
            curve={curveMonotoneX}
          />
          <LinePath<Point>
            data={points.filter(defined("acute"))}
            x={(p) => xScale(p.date)}
            y={yOf("acute")}
            stroke="#fbbf24"
            strokeWidth={2}
            curve={curveMonotoneX}
          />
        </Group>
      </svg>
      <div className="mt-2 flex gap-4 text-xs text-slate-400">
        <Legend color="#fbbf24" label="Acute (7d)" />
        <Legend color="#38bdf8" label="Chronic (28d)" />
      </div>
    </div>
  );
}

function Legend({ color, label }: { color: string; label: string }) {
  return (
    <span className="flex items-center gap-1.5">
      <span className="inline-block h-2 w-3 rounded-sm" style={{ backgroundColor: color }} />
      {label}
    </span>
  );
}
