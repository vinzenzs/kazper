import { useState } from "react";

import { Panel } from "../components/Panel";
import { StatsTotals } from "../components/StatsTotals";
import { ActivityHeatmap } from "../components/ActivityHeatmap";
import { PowerCurveChart } from "../components/PowerCurveChart";
import { PMCChart, PMCSummary, TargetReadout } from "../components/PMCChart";
import { CPModelChart } from "../components/CPModelChart";
import { PowerProfilePanel } from "../components/PowerProfilePanel";
import { DurabilityPanel } from "../components/DurabilityPanel";
import { IntensityDistributionPanel } from "../components/IntensityDistribution";
import {
  useCPModel,
  useIntensityDistribution,
  usePMC,
  usePowerCurve,
  usePowerProfile,
  useDurability,
  useTargetTrajectory,
  useWorkoutStats,
} from "../api/hooks";

type PMCWindow = "90" | "180" | "365";

const PMC_WINDOWS: { key: PMCWindow; label: string }[] = [
  { key: "90", label: "90d" },
  { key: "180", label: "180d" },
  { key: "365", label: "365d" },
];

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
  const [pmcWindow, setPmcWindow] = useState<PMCWindow>("90");
  const [cpWindow, setCpWindow] = useState<PMCWindow>("90");
  const { from, to } = rangeFor(period);
  const { data, isLoading, isError, error } = useWorkoutStats(from, to);
  const curve = usePowerCurve(from, to, sport);
  const pmcRange = trailingDays(Number(pmcWindow) - 1);
  const pmc = usePMC(pmcRange.from, pmcRange.to);
  const trajectory = useTargetTrajectory();
  // Overlay only when the active macrocycle declares targets; a 404 / targets_missing
  // / fetch error leaves the measured PMC panel exactly as it was.
  const target = trajectory.data?.trajectory ?? undefined;
  const targetSummary = trajectory.data?.summary;
  const cpRange = trailingDays(Number(cpWindow) - 1);
  const cp = useCPModel(cpRange.from, cpRange.to);
  const [ppWindow, setPpWindow] = useState<PMCWindow>("90");
  const ppRange = trailingDays(Number(ppWindow) - 1);
  const pp = usePowerProfile(ppRange.from, ppRange.to);
  const [durWindow, setDurWindow] = useState<PMCWindow>("365");
  const durRange = trailingDays(Number(durWindow) - 1);
  const durability = useDurability(durRange.from, durRange.to);
  const intensity = useIntensityDistribution(from, to);

  const total = data?.total;
  const isEmpty = !!total && total.count === 0;
  const curveEmpty = !!curve.data && curve.data.points.length === 0;
  // All-zero form/fitness ⇒ no training history to chart.
  const pmcEmpty =
    !!pmc.data && pmc.data.days.every((d) => d.ctl === 0 && d.atl === 0);
  const intensityEmpty =
    !!intensity.data && intensity.data.total.total_zone_secs === 0;

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
        title="Performance (CTL / ATL / TSB)"
        isLoading={pmc.isLoading}
        isError={pmc.isError}
        error={pmc.error}
      >
        <div className="flex flex-col gap-3">
          <Toggle options={PMC_WINDOWS} value={pmcWindow} onChange={setPmcWindow} />
          {pmc.data && !pmcEmpty ? (
            <>
              <PMCSummary series={pmc.data} />
              {targetSummary && <TargetReadout summary={targetSummary} />}
              <PMCChart series={pmc.data} target={target} />
            </>
          ) : (
            <div className="py-6 text-center text-sm text-slate-500">
              No training history to chart yet
            </div>
          )}
        </div>
      </Panel>

      <Panel
        title="Intensity distribution"
        isLoading={intensity.isLoading}
        isError={intensity.isError}
        error={intensity.error}
      >
        {intensity.data && !intensityEmpty ? (
          <IntensityDistributionPanel dist={intensity.data} />
        ) : (
          <div className="py-6 text-center text-sm text-slate-500">
            No HR-zone data in this period
          </div>
        )}
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

      <Panel
        title="Critical power (CP / W′)"
        isLoading={cp.isLoading}
        isError={cp.isError}
        error={cp.error}
      >
        <div className="flex flex-col gap-3">
          <Toggle options={PMC_WINDOWS} value={cpWindow} onChange={setCpWindow} />
          {cp.data ? (
            <CPModelChart result={cp.data} />
          ) : (
            <div className="py-6 text-center text-sm text-slate-500">
              Loading critical-power model…
            </div>
          )}
        </div>
      </Panel>

      <Panel title="Power profile (Coggan)" isLoading={pp.isLoading}>
        {/* The read 400s with weight_data_missing when no weight is on file; the
            panel degrades to a neutral hint rather than an error banner. */}
        <div className="flex flex-col gap-3">
          <Toggle options={PMC_WINDOWS} value={ppWindow} onChange={setPpWindow} />
          {pp.data ? (
            <PowerProfilePanel result={pp.data} />
          ) : (
            <div className="py-6 text-center text-sm text-slate-500">
              {pp.isError
                ? "Add a body-weight entry to rank your power profile."
                : "Loading power profile…"}
            </div>
          )}
        </div>
      </Panel>

      <Panel
        title="Durability (fatigue resistance)"
        isLoading={durability.isLoading}
        isError={durability.isError}
        error={durability.error}
      >
        <div className="flex flex-col gap-3">
          <Toggle options={PMC_WINDOWS} value={durWindow} onChange={setDurWindow} />
          {durability.data && <DurabilityPanel result={durability.data} />}
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

// trailingDays returns [today − n days, today] as local YYYY-MM-DD bounds.
function trailingDays(n: number): { from: string; to: string } {
  const now = new Date();
  const start = new Date(now);
  start.setDate(start.getDate() - n);
  return { from: ymd(start), to: ymd(now) };
}

function ymd(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}
