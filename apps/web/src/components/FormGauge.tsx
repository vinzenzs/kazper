import { Panel } from "./Panel";
import { num } from "../lib/format";

interface FormGaugeProps {
  acwr: number | null | undefined;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
}

// ACWR (acute:chronic workload ratio) form gauge. The widely-cited "sweet spot"
// is ~0.8–1.3; below is detraining/fresh, above ~1.5 is a spike-injury-risk
// zone. We render a semicircular gauge from 0 to 2.0 with a needle at the ratio.
const MIN = 0;
const MAX = 2;

function zone(acwr: number): { label: string; color: string } {
  if (acwr < 0.8) return { label: "Undertrained / fresh", color: "#38bdf8" };
  if (acwr <= 1.3) return { label: "Optimal", color: "#34d399" };
  if (acwr <= 1.5) return { label: "Caution", color: "#fbbf24" };
  return { label: "High risk", color: "#f87171" };
}

// polar point on the 180° (π) top arc, angle measured from the left (π) to right (0).
function pointOnArc(cx: number, cy: number, r: number, fraction: number) {
  const angle = Math.PI - fraction * Math.PI;
  return { x: cx + r * Math.cos(angle), y: cy - r * Math.sin(angle) };
}

export function FormGauge({ acwr, isLoading, isError, error }: FormGaugeProps) {
  const hasValue = acwr !== null && acwr !== undefined && !Number.isNaN(acwr);

  return (
    <Panel
      title="Form · ACWR"
      isLoading={isLoading}
      isError={isError}
      error={error}
      isEmpty={!hasValue}
      emptyHint="No load ratio — needs acute + chronic load"
    >
      {hasValue && <Gauge acwr={acwr as number} />}
    </Panel>
  );
}

function Gauge({ acwr }: { acwr: number }) {
  const z = zone(acwr);
  const w = 240;
  const h = 140;
  const cx = w / 2;
  const cy = h - 12;
  const r = 96;

  const clamped = Math.max(MIN, Math.min(MAX, acwr));
  const fraction = (clamped - MIN) / (MAX - MIN);
  const needle = pointOnArc(cx, cy, r - 10, fraction);

  // Colored zone bands along the arc as stroked sub-arcs.
  const bands = [
    { from: 0, to: 0.8 / MAX, color: "#38bdf8" },
    { from: 0.8 / MAX, to: 1.3 / MAX, color: "#34d399" },
    { from: 1.3 / MAX, to: 1.5 / MAX, color: "#fbbf24" },
    { from: 1.5 / MAX, to: 1, color: "#f87171" },
  ];

  return (
    <div className="flex flex-col items-center">
      <svg viewBox={`0 0 ${w} ${h}`} className="w-full max-w-[240px]" role="img" aria-label="ACWR gauge">
        {bands.map((b, i) => {
          const a = pointOnArc(cx, cy, r, b.from);
          const c = pointOnArc(cx, cy, r, b.to);
          const large = b.to - b.from > 0.5 ? 1 : 0;
          return (
            <path
              key={i}
              d={`M ${a.x} ${a.y} A ${r} ${r} 0 ${large} 1 ${c.x} ${c.y}`}
              fill="none"
              stroke={b.color}
              strokeWidth={12}
              strokeLinecap="butt"
              opacity={0.85}
            />
          );
        })}
        <line
          x1={cx}
          y1={cy}
          x2={needle.x}
          y2={needle.y}
          stroke="#e2e8f0"
          strokeWidth={3}
          strokeLinecap="round"
        />
        <circle cx={cx} cy={cy} r={5} fill="#e2e8f0" />
      </svg>
      <div className="mt-1 text-center">
        <div className="text-3xl font-bold tabular-nums" style={{ color: z.color }}>
          {num(acwr, 2)}
        </div>
        <div className="text-xs uppercase tracking-widest text-slate-400">{z.label}</div>
      </div>
    </div>
  );
}
