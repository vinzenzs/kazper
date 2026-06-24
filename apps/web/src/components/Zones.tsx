import { Panel } from "./Panel";
import { ZoneStrip } from "./ZoneStrip";
import type { AthleteConfig } from "../api/types";

interface ZonesProps {
  config: AthleteConfig | null | undefined;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
}

// HR and power training zones as two banded Z1–Z5 strips. Boundaries come from
// athlete_config (already in the training context); each is nullable, so a strip
// shows only the bands it has and the panel is empty only when neither set has
// any boundary.
export function Zones({ config, isLoading, isError, error }: ZonesProps) {
  const hr = [
    config?.hr_zone_1_max,
    config?.hr_zone_2_max,
    config?.hr_zone_3_max,
    config?.hr_zone_4_max,
    config?.hr_zone_5_max,
  ];
  const power = [
    config?.power_zone_1_max,
    config?.power_zone_2_max,
    config?.power_zone_3_max,
    config?.power_zone_4_max,
    config?.power_zone_5_max,
  ];

  const present = (xs: (number | null | undefined)[]) =>
    xs.some((v) => v !== null && v !== undefined && !Number.isNaN(v));
  const hasAny = present(hr) || present(power);

  return (
    <Panel
      title="Training zones"
      isLoading={isLoading}
      isError={isError}
      error={error}
      isEmpty={!hasAny}
      emptyHint="No zones configured"
    >
      {hasAny && (
        <div className="flex flex-col gap-3">
          <ZoneStrip label="HR" unit="bpm" zones={hr} />
          <ZoneStrip label="Power" unit="W" zones={power} />
        </div>
      )}
    </Panel>
  );
}
