import { Group } from "@visx/group";
import { scaleLinear } from "@visx/scale";
import { LinePath } from "@visx/shape";
import { AxisBottom, AxisLeft } from "@visx/axis";
import { curveStepAfter } from "@visx/curve";

import type { CPModelHistoryResult, ThresholdHistory } from "../api/types";
import { num } from "../lib/format";

const W = 560;
const H = 220;
const MARGIN = { top: 12, right: 12, bottom: 26, left: 40 };

// The critical-power trend: fitted CP (W) at weekly anchors over the range, with
// the configured-FTP step line overlaid (client-side compose — the backend stays
// uncoupled from athlete-config). Null anchors are gapped, not zeroed.
export function CPTrendChart({
  history,
  ftp,
}: {
  history: CPModelHistoryResult;
  ftp?: ThresholdHistory;
}) {
  const anchors = history.anchors;
  const innerW = W - MARGIN.left - MARGIN.right;
  const innerH = H - MARGIN.top - MARGIN.bottom;

  const fitted = anchors.filter((a) => a.model !== null);
  if (fitted.length < 2) {
    return (
      <div className="py-4 text-center text-sm text-slate-500">
        Not enough fitted anchors to chart a CP trend yet.
      </div>
    );
  }

  const t0 = new Date(anchors[0].date).getTime();
  const t1 = new Date(anchors[anchors.length - 1].date).getTime();
  const xScale = scaleLinear<number>({ domain: [t0, Math.max(t1, t0 + 1)], range: [0, innerW] });

  const ftpPoints = (ftp?.history ?? [])
    .filter((h) => h.ftp_watts != null)
    .map((h) => ({ t: new Date(h.effective_from).getTime(), w: h.ftp_watts as number }));

  const cpVals = fitted.map((a) => a.model!.cp_watts);
  const allVals = [...cpVals, ...ftpPoints.map((p) => p.w)];
  const yMin = Math.min(...allVals) * 0.92;
  const yMax = Math.max(...allVals) * 1.08;
  const yScale = scaleLinear<number>({ domain: [yMin, yMax], range: [innerH, 0], nice: true });

  // The CP line is split at gaps: consecutive fitted anchors form segments.
  const segments: { t: number; w: number }[][] = [];
  let cur: { t: number; w: number }[] = [];
  for (const a of anchors) {
    if (a.model) {
      cur.push({ t: new Date(a.date).getTime(), w: a.model.cp_watts });
    } else if (cur.length) {
      segments.push(cur);
      cur = [];
    }
  }
  if (cur.length) segments.push(cur);

  return (
    <div className="flex flex-col gap-2">
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="Critical power trend">
        <Group left={MARGIN.left} top={MARGIN.top}>
          {/* Configured-FTP step line (overlay), if present. */}
          {ftpPoints.length > 0 && (
            <LinePath
              data={ftpPoints}
              x={(p) => xScale(p.t)}
              y={(p) => yScale(p.w)}
              stroke="#f59e0b"
              strokeWidth={1.5}
              strokeDasharray="4,3"
              curve={curveStepAfter}
              data-testid="ftp-overlay"
            />
          )}
          {/* CP trend, gapped at null anchors. */}
          {segments.map((seg, i) => (
            <LinePath
              key={i}
              data={seg}
              x={(p) => xScale(p.t)}
              y={(p) => yScale(p.w)}
              stroke="#38bdf8"
              strokeWidth={2}
            />
          ))}
          {fitted.map((a) => (
            <circle
              key={a.date}
              cx={xScale(new Date(a.date).getTime())}
              cy={yScale(a.model!.cp_watts)}
              r={2}
              fill="#38bdf8"
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
            numTicks={4}
            tickFormat={(v) => new Date(Number(v)).toISOString().slice(5, 10)}
            stroke="#3a4658"
            tickStroke="#3a4658"
            tickLabelProps={() => ({ fill: "#94a3b8", fontSize: 9, textAnchor: "middle" })}
          />
        </Group>
      </svg>
      <div className="text-xs text-slate-500">
        <span className="text-sky-300">— CP</span>
        {ftpPoints.length > 0 && <span className="text-amber-400"> ┄ configured FTP</span>} · latest CP{" "}
        {num(cpVals[cpVals.length - 1], 0)} W over a {history.window_days}-day window.
      </div>
    </div>
  );
}
