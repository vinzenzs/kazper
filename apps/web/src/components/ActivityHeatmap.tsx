import { scaleLinear } from "@visx/scale";

import type { WorkoutStatsBucket } from "../api/types";
import { duration } from "../lib/format";

interface ActivityHeatmapProps {
  days: WorkoutStatsBucket[];
  // The metric coloring each cell — total elapsed minutes that day.
  // (Distance/kcal would be sport-biased; duration is the most sport-neutral.)
}

const CELL = 12;
const GAP = 3;
const DOW = ["Mon", "", "Wed", "", "Fri", "", "Sun"];

// A GitHub-style calendar heatmap: one cell per day, columns are ISO weeks
// (Monday-first), rows are weekdays. Cell fill scales with the day's total
// duration; zero-activity days stay a faint base. Scrolls horizontally for a
// year-long window. Built from plain SVG + a visx color scale, matching the
// muted analyst idiom (no external heatmap dep).
export function ActivityHeatmap({ days }: ActivityHeatmapProps) {
  const cells = days
    .filter((d): d is WorkoutStatsBucket & { date: string } => !!d.date)
    .map((d) => ({ bucket: d, date: parseDate(d.date) }))
    .filter((c) => c.date !== null) as {
    bucket: WorkoutStatsBucket & { date: string };
    date: Date;
  }[];

  if (cells.length === 0) {
    return <div className="text-sm text-slate-500">No activity in window</div>;
  }

  const max = Math.max(...cells.map((c) => c.bucket.total_duration_min), 1);
  const color = scaleLinear<number>({ domain: [0, max], range: [0.06, 0.85] });

  const first = cells[0].date;
  const weekOffset = (day: Date) =>
    Math.floor((day.getTime() - startOfWeek(first).getTime()) / (7 * 86400000));
  const weeks = weekOffset(cells[cells.length - 1].date) + 1;

  const width = 24 + weeks * (CELL + GAP);
  const height = 7 * (CELL + GAP);

  return (
    <div className="overflow-x-auto">
      <svg width={width} height={height} role="img" aria-label="Activity heatmap">
        {DOW.map((label, i) =>
          label ? (
            <text
              key={i}
              x={0}
              y={i * (CELL + GAP) + CELL}
              className="fill-slate-500"
              fontSize={9}
            >
              {label}
            </text>
          ) : null,
        )}
        {cells.map((c) => {
          const x = 24 + weekOffset(c.date) * (CELL + GAP);
          const y = mondayIndex(c.date) * (CELL + GAP);
          const min = c.bucket.total_duration_min;
          const active = min > 0;
          return (
            <rect
              key={c.bucket.date}
              x={x}
              y={y}
              width={CELL}
              height={CELL}
              rx={2}
              fill={active ? "#38bdf8" : "#1e293b"}
              fillOpacity={active ? color(min) : 0.35}
            >
              <title>
                {c.bucket.date}: {active ? duration(min) : "rest"}
                {c.bucket.count > 0 ? ` · ${c.bucket.count} act` : ""}
              </title>
            </rect>
          );
        })}
      </svg>
    </div>
  );
}

// Monday-first weekday index: Mon=0 … Sun=6.
function mondayIndex(d: Date): number {
  return (d.getDay() + 6) % 7;
}

function startOfWeek(d: Date): Date {
  const s = new Date(d);
  s.setDate(s.getDate() - mondayIndex(d));
  s.setHours(0, 0, 0, 0);
  return s;
}

// Parse a YYYY-MM-DD as a local date (avoids the UTC shift of new Date(iso)).
function parseDate(iso: string): Date | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!m) return null;
  return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
}
