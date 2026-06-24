import { Header } from "./components/Header";
import { FormGauge } from "./components/FormGauge";
import { LoadTrend } from "./components/LoadTrend";
import { RecoverySnapshot } from "./components/RecoverySnapshot";
import { WorkoutList } from "./components/WorkoutList";
import {
  useFitnessTrend,
  useRecoveryContext,
  useTrainingContext,
} from "./api/hooks";

// The v1 dashboard is training-only (per the proposal): header, ACWR/form gauge,
// acute/chronic load trend, recovery snapshot, and recent + upcoming workouts.
// No fueling / energy-availability / nutrition panels.
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
