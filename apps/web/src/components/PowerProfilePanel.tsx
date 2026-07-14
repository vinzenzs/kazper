import type { PowerProfileResult, PowerProfileAnchor } from "../api/types";
import { num } from "../lib/format";

// Human labels for the four Coggan anchors and the rider phenotypes.
const ANCHOR_LABEL: Record<PowerProfileAnchor["label"], string> = {
  neuromuscular: "Neuromuscular (5 s)",
  anaerobic: "Anaerobic (1 min)",
  vo2max: "VO₂max (5 min)",
  threshold: "Threshold (20 min)",
};

const PHENOTYPE_LABEL: Record<string, string> = {
  sprinter: "Sprinter",
  time_trialist: "Time-trialist",
  climber: "Climber",
  all_rounder: "All-rounder",
};

// Order anchors shortest → longest regardless of API order.
const ANCHOR_ORDER: PowerProfileAnchor["label"][] = [
  "neuromuscular",
  "anaerobic",
  "vo2max",
  "threshold",
];

// The power-profile panel: one row per ranked anchor showing W/kg, the Coggan
// category badge and an interpolated percentile bar, plus the rider phenotype and
// the W/kg denominator provenance. Category is authoritative; the percentile is a
// gradient estimate. Missing anchors are listed, not fabricated.
export function PowerProfilePanel({ result }: { result: PowerProfileResult }) {
  const byLabel = new Map(result.anchors.map((a) => [a.label, a]));
  const ordered = ANCHOR_ORDER.map((l) => byLabel.get(l)).filter(
    (a): a is PowerProfileAnchor => !!a,
  );

  if (ordered.length === 0) {
    return (
      <div className="py-6 text-center text-sm text-slate-500">
        No rankable power efforts in this window.
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-baseline justify-between gap-2 text-xs text-slate-400">
        <span>
          {result.phenotype ? (
            <span className="font-semibold text-accent">
              {PHENOTYPE_LABEL[result.phenotype] ?? result.phenotype}
            </span>
          ) : (
            <span>Phenotype needs all four anchors</span>
          )}
        </span>
        <span>
          {num(result.weight_kg, 1)} kg ({result.sex},{" "}
          {result.weight_source === "param" ? "given" : "stored"})
        </span>
      </div>

      <div className="flex flex-col gap-2" data-testid="power-profile-anchors">
        {ordered.map((a) => (
          <AnchorRow key={a.label} anchor={a} />
        ))}
      </div>

      {result.missing_anchors.length > 0 && (
        <div className="text-xs text-slate-500">
          No effort for: {result.missing_anchors.map((m) => ANCHOR_LABEL[m as PowerProfileAnchor["label"]] ?? m).join(", ")}
        </div>
      )}
      <div className="text-xs text-slate-500">
        Coggan power profile — category is the ranking; percentile is an interpolated estimate. Advisory.
      </div>
    </div>
  );
}

function AnchorRow({ anchor }: { anchor: PowerProfileAnchor }) {
  return (
    <div className="rounded-md bg-slate-800/50 px-3 py-2">
      <div className="flex items-baseline justify-between gap-2">
        <span className="text-sm text-slate-300">{ANCHOR_LABEL[anchor.label]}</span>
        <span className="text-sm font-semibold text-slate-100">
          {num(anchor.w_per_kg, 1)} W/kg
          <span className="ml-1 text-xs font-normal text-slate-400">
            ({num(anchor.watts, 0)} W)
          </span>
        </span>
      </div>
      <div className="mt-1 flex items-center gap-2">
        <span className="rounded bg-slate-700 px-1.5 py-0.5 text-xs text-slate-200">
          {anchor.category}
        </span>
        <div className="h-1.5 flex-1 overflow-hidden rounded bg-slate-700">
          <div
            className="h-full bg-accent"
            style={{ width: `${Math.max(0, Math.min(100, anchor.percentile))}%` }}
          />
        </div>
        <span className="w-10 text-right text-xs text-slate-400">
          {num(anchor.percentile, 0)}
        </span>
      </div>
    </div>
  );
}
