import { useState } from "react";

import { Panel } from "../components/Panel";
import { StatsTotals } from "../components/StatsTotals";
import { ActivityHeatmap } from "../components/ActivityHeatmap";
import { useWorkoutStats } from "../api/hooks";

type Period = "week" | "month" | "ytd";

const PERIODS: { key: Period; label: string }[] = [
  { key: "week", label: "Week" },
  { key: "month", label: "Month" },
  { key: "ytd", label: "YTD" },
];

// The /stats route: a training-log view. A Week/Month/YTD toggle selects the
// range that drives the volume totals and the activity heatmap.
export function StatsView() {
  const [period, setPeriod] = useState<Period>("week");
  const { from, to } = rangeFor(period);
  const { data, isLoading, isError, error } = useWorkoutStats(from, to);

  const total = data?.total;
  const isEmpty = !!total && total.count === 0;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex gap-1">
        {PERIODS.map((p) => (
          <button
            key={p.key}
            type="button"
            onClick={() => setPeriod(p.key)}
            className={`rounded-md px-3 py-1 text-sm font-medium transition-colors ${
              period === p.key
                ? "bg-ink-700/70 text-slate-100"
                : "text-slate-400 hover:text-slate-200"
            }`}
          >
            {p.label}
          </button>
        ))}
      </div>

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
