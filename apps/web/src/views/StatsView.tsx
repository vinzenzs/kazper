import { useState } from "react";

import { Panel } from "../components/Panel";
import { StatsTotals } from "../components/StatsTotals";
import { ActivityHeatmap } from "../components/ActivityHeatmap";
import { PowerCurveChart } from "../components/PowerCurveChart";
import { usePowerCurve, useWorkoutStats } from "../api/hooks";

type Period = "week" | "month" | "ytd";

const PERIODS: { key: Period; label: string }[] = [
  { key: "week", label: "Week" },
  { key: "month", label: "Month" },
  { key: "ytd", label: "YTD" },
];

const SPORTS: { key: string; label: string }[] = [
  { key: "bike", label: "Bike" },
  { key: "run", label: "Run" },
  { key: "swim", label: "Swim" },
];

// A small segmented toggle shared by the period and sport selectors.
function Toggle<T extends string>({
  options,
  value,
  onChange,
}: {
  options: { key: T; label: string }[];
  value: T;
  onChange: (v: T) => void;
}) {
  return (
    <div className="flex gap-1">
      {options.map((o) => (
        <button
          key={o.key}
          type="button"
          onClick={() => onChange(o.key)}
          className={`rounded-md px-3 py-1 text-sm font-medium transition-colors ${
            value === o.key
              ? "bg-ink-700/70 text-slate-100"
              : "text-slate-400 hover:text-slate-200"
          }`}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

// The /stats route: a training-log view. A Week/Month/YTD toggle selects the
// range that drives the volume totals and the activity heatmap.
export function StatsView() {
  const [period, setPeriod] = useState<Period>("week");
  const [sport, setSport] = useState<string>("bike");
  const { from, to } = rangeFor(period);
  const { data, isLoading, isError, error } = useWorkoutStats(from, to);
  const curve = usePowerCurve(from, to, sport);

  const total = data?.total;
  const isEmpty = !!total && total.count === 0;
  const curveEmpty = !!curve.data && curve.data.points.length === 0;

  return (
    <div className="flex flex-col gap-4">
      <Toggle options={PERIODS} value={period} onChange={setPeriod} />

      <Panel
        title="Totals"
        isLoading={isLoading}
        isError={isError}
        error={error}
        isEmpty={isEmpty}
        emptyHint="No completed workouts in this period"
      >
        {total && <StatsTotals total={total} />}
      </Panel>

      <Panel
        title="Activity"
        isLoading={isLoading}
        isError={isError}
        error={error}
        isEmpty={isEmpty}
        emptyHint="No activity in this period"
      >
        {data && <ActivityHeatmap days={data.days} />}
      </Panel>

      <Panel
        title="Power / pace curve"
        isLoading={curve.isLoading}
        isError={curve.isError}
        error={curve.error}
      >
        {/* The sport selector stays visible even when a sport has no data, so
            the user can switch to one that does. */}
        <div className="flex flex-col gap-3">
          <Toggle options={SPORTS} value={sport} onChange={setSport} />
          {curve.data && !curveEmpty ? (
            <PowerCurveChart curve={curve.data} />
          ) : (
            <div className="py-6 text-center text-sm text-slate-500">
              No effort data for {sport} in this period
            </div>
          )}
        </div>
      </Panel>
    </div>
  );
}

// rangeFor returns local YYYY-MM-DD bounds for the selected period: trailing 7
// or 30 days, or year-to-date (Jan 1 → today).
function rangeFor(period: Period): { from: string; to: string } {
  const now = new Date();
  const to = ymd(now);
  if (period === "ytd") {
    return { from: ymd(new Date(now.getFullYear(), 0, 1)), to };
  }
  const days = period === "week" ? 6 : 29;
  const start = new Date(now);
  start.setDate(start.getDate() - days);
  return { from: ymd(start), to };
}

function ymd(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}
