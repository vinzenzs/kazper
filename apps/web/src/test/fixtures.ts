import type {
  FitnessSnapshot,
  RecoveryContext,
  TrainingContext,
} from "../api/types";

// A populated training context covering the happy path.
export const populatedTraining: TrainingContext = {
  date: "2026-06-24",
  tz: "Europe/Vienna",
  lookback_days: 14,
  lookahead_days: 7,
  phase: {
    id: "p1",
    name: "Build 2",
    type: "build",
    start_date: "2026-06-01T00:00:00Z",
    end_date: "2026-06-28T00:00:00Z",
    methodology: null,
  },
  macrocycle: {
    id: "m1",
    name: "Alpe d'Huez 2026",
    start_date: "2026-01-01T00:00:00Z",
    end_date: "2026-08-01T00:00:00Z",
    race_id: "r1",
    race_name: "Alpe d'Huez Triathlon",
    race_date: "2026-07-30T00:00:00Z",
    days_to_race: 36,
    current_phase_ordinal: 4,
    total_periods: 7,
  },
  fitness: {
    date: "2026-06-24",
    vo2max_running: 56.2,
    vo2max_cycling: 61.4,
    race_predictor_5k_seconds: 1112,
    race_predictor_10k_seconds: 2310,
    race_predictor_half_seconds: 5045,
    race_predictor_full_seconds: 10625,
    endurance_score: 7100,
    hill_score: 62,
    fitness_age: 31,
    acute_load: 420,
    chronic_load: 390,
    training_status: "productive",
  },
  acwr: 1.08,
  athlete_config: {
    ftp_watts: 285,
    threshold_hr: 168,
    lactate_threshold_hr: 165,
    max_hr: 190,
    threshold_pace_sec_per_km: 245,
    threshold_swim_pace_sec_per_100m: 98,
    hr_zone_1_max: 120,
    hr_zone_2_max: 140,
    hr_zone_3_max: 160,
    hr_zone_4_max: 175,
    hr_zone_5_max: 190,
    power_zone_1_max: 155,
    power_zone_2_max: 210,
    power_zone_3_max: 255,
    power_zone_4_max: 300,
    power_zone_5_max: 360,
  },
  watts_per_kg: 4.1,
  recent_load: {
    count: 5,
    total_duration_min: 540,
    total_kcal: 6200,
    by_sport: { cycling: 3, running: 2 },
  },
  recent_workouts: [
    {
      id: "w1",
      sport: "cycling",
      status: "completed",
      name: "Threshold 4x8",
      started_at: "2026-06-23T06:00:00Z",
      ended_at: "2026-06-23T07:30:00Z",
      duration_min: 90,
      kcal_burned: 1100,
      tss: 120,
    },
  ],
  upcoming_workouts: [
    {
      id: "w2",
      sport: "running",
      status: "planned",
      name: "Long run",
      started_at: "2026-06-25T06:00:00Z",
      ended_at: "2026-06-25T08:00:00Z",
      duration_min: 120,
      tss: 95,
    },
  ],
};

// A null-heavy training context: no phase, no season, no fitness, no ACWR, no
// workouts. Every panel must degrade gracefully rather than throw.
export const emptyTraining: TrainingContext = {
  date: "2026-06-24",
  tz: "UTC",
  lookback_days: 14,
  lookahead_days: 7,
  phase: null,
  macrocycle: null,
  fitness: null,
  acwr: null,
  athlete_config: null,
  watts_per_kg: null,
  recent_load: { count: 0, total_duration_min: 0, total_kcal: 0, by_sport: {} },
  recent_workouts: null,
  upcoming_workouts: null,
};

export const populatedRecovery: RecoveryContext = {
  date: "2026-06-24",
  days: 7,
  latest: {
    date: "2026-06-24",
    hrv_ms: 78,
    sleep_seconds: 27000,
    sleep_score: 84,
    resting_hr: 44,
    stress_avg: 28,
    body_battery_charged: 71,
    body_battery_drained: 54,
    training_readiness: 82,
  },
  recent: [],
};

export const emptyRecovery: RecoveryContext = {
  date: "2026-06-24",
  days: 7,
  latest: null,
  recent: null,
};

export const populatedTrend: FitnessSnapshot[] = [
  { date: "2026-06-18", acute_load: 380, chronic_load: 370 },
  { date: "2026-06-20", acute_load: 410, chronic_load: 380 },
  { date: "2026-06-22", acute_load: 430, chronic_load: 388 },
  { date: "2026-06-24", acute_load: 420, chronic_load: 390 },
];

export const emptyTrend: FitnessSnapshot[] = [];
