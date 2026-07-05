import { PersonalRecords } from "../components/PersonalRecords";
import { Achievements } from "../components/Achievements";
import { useAchievements, usePersonalRecords } from "../api/hooks";

// The /records route: personal records (best efforts) as a dense table plus a
// compact achievements strip. Reads the /personal-records and /achievements
// capability endpoints directly.
export function RecordsView() {
  const records = usePersonalRecords();
  const achievements = useAchievements();

  return (
    <div className="flex flex-col gap-4">
      <PersonalRecords
        records={records.data?.personal_records}
        isLoading={records.isLoading}
        isError={records.isError}
        error={records.error}
      />
      <Achievements
        achievements={achievements.data?.achievements}
        isLoading={achievements.isLoading}
        isError={achievements.isError}
        error={achievements.error}
      />
    </div>
  );
}
