import { Header } from "./components/Header";
import { FormGauge } from "./components/FormGauge";
import { LoadTrend } from "./components/LoadTrend";
import { FitnessPanel } from "./components/FitnessPanel";
import { RacePredictions } from "./components/RacePredictions";
import { PowerThresholds } from "./components/PowerThresholds";
import { Zones } from "./components/Zones";
import { RecoverySnapshot } from "./components/RecoverySnapshot";
import { WorkoutList } from "./components/WorkoutList";
import {
  useFitnessTrend,
  useRecoveryContext,
  useTrainingContext,
} from "./api/hooks";

// The dashboard is training-only: header, ACWR/form gauge, acute/chronic load
// trend, fitness / race-prediction / power-threshold / zone panels, recovery
// snapshot, and recent + upcoming workouts. All data comes from the existing
// context payloads. No fueling / energy-availability / nutrition panels.
export function App() {
  const training = useTrainingContext();
  const recovery = useRecoveryContext();
  const trend = useFitnessTrend();

  const t = training.data;

  return (
    <div className="mx-auto flex min-h-full max-w-screen-2xl flex-col gap-4 p-4 lg:p-6">
      <Header training={t} isLoading={training.isLoading} />

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <FormGauge
          acwr={t?.acwr ?? null}
          isLoading={training.isLoading}
          isError={training.isError}
          error={training.error}
        />
        <div className="lg:col-span-2">
          <LoadTrend
            metrics={trend.data?.fitness_metrics}
            isLoading={trend.isLoading}
            isError={trend.isError}
            error={trend.error}
          />
        </div>
      </div>

      <FitnessPanel
        fitness={t?.fitness ?? null}
        isLoading={training.isLoading}
        isError={training.isError}
        error={training.error}
      />

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <RacePredictions
          fitness={t?.fitness ?? null}
          isLoading={training.isLoading}
          isError={training.isError}
          error={training.error}
        />
        <PowerThresholds
          config={t?.athlete_config ?? null}
          wattsPerKg={t?.watts_per_kg ?? null}
          isLoading={training.isLoading}
          isError={training.isError}
          error={training.error}
        />
      </div>

      <Zones
        config={t?.athlete_config ?? null}
        isLoading={training.isLoading}
        isError={training.isError}
        error={training.error}
      />

      <RecoverySnapshot
        latest={recovery.data?.latest ?? null}
        isLoading={recovery.isLoading}
        isError={recovery.isError}
        error={recovery.error}
      />

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <WorkoutList
          title="Recent workouts"
          workouts={t?.recent_workouts}
          bySport={t?.recent_load?.by_sport ?? null}
          isLoading={training.isLoading}
          isError={training.isError}
          error={training.error}
          emptyHint="No completed workouts in window"
        />
        <WorkoutList
          title="Upcoming workouts"
          workouts={t?.upcoming_workouts}
          isLoading={training.isLoading}
          isError={training.isError}
          error={training.error}
          emptyHint="Nothing scheduled"
        />
      </div>
    </div>
  );
}
