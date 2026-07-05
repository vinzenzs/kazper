import { GearPanel } from "../components/GearPanel";
import { useGear } from "../api/hooks";

// The /gear route: gear inventory with accumulated mileage. Reads /gear.
export function GearView() {
  const gear = useGear();
  return (
    <GearPanel
      gear={gear.data?.gear}
      isLoading={gear.isLoading}
      isError={gear.isError}
      error={gear.error}
    />
  );
}
