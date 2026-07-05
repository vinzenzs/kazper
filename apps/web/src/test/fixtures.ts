import type {
  Achievement,
  FitnessSnapshot,
  Gear,
  PersonalRecord,
  RecoveryContext,
  TrainingContext,
  Workout,
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

export const populatedRecords: PersonalRecord[] = [
  {
    id: "pr1",
    external_id: "garmin:pr:1",
    pr_type: "fastest_5k",
    value: 1112, // 18:32
    unit: "s",
    activity_id: "garmin:act:9",
    achieved_at: "2026-05-10T07:00:00Z",
  },
  {
    id: "pr2",
    external_id: "garmin:pr:2",
    pr_type: "longest_ride",
    value: 182000, // 182 km
    unit: "m",
    achieved_at: "2026-04-02T06:00:00Z",
  },
];

export const populatedAchievements: Achievement[] = [
  {
    id: "a1",
    external_id: "garmin:badge:1",
    kind: "badge",
    name: "Century Ride",
    earned_at: "2026-04-02T09:00:00Z",
  },
  {
    id: "a2",
    external_id: "garmin:challenge:1",
    kind: "challenge",
    name: "March 200km",
    progress_pct: 64,
  },
];

export const populatedGear: Gear[] = [
  {
    id: "g1",
    external_id: "garmin:gear:1",
    gear_type: "shoes",
    display_name: "Vaporfly 3",
    total_distance_m: 412000,
    total_activities: 58,
    retired: false,
  },
  {
    id: "g2",
    external_id: "garmin:gear:2",
    gear_type: "bike",
    display_name: "Canyon Aeroad",
    total_distance_m: 8140000,
    total_activities: 210,
    retired: true,
  },
];

export const populatedWorkoutDetail: Workout = {
  id: "w1",
  sport: "cycling",
  status: "completed",
  name: "Threshold 4x8",
  started_at: "2026-06-23T06:00:00Z",
  ended_at: "2026-06-23T07:30:00Z",
  kcal_burned: 1100,
  avg_hr: 152,
  max_hr: 178,
  tss: 120,
  distance_m: 48200,
  elevation_gain_m: 640,
  avg_power_w: 232,
  normalized_power_w: 258,
  intensity_factor: 0.9,
  avg_cadence: 88,
  secs_in_zone_1: 600,
  secs_in_zone_2: 1800,
  secs_in_zone_3: 900,
  secs_in_zone_4: 1500,
  secs_in_zone_5: 600,
  splits: [
    {
      split_index: 0,
      distance_m: 1000,
      duration_s: 150,
      avg_hr: 148,
      avg_power_w: 230,
      avg_speed_mps: 6.7,
      elevation_gain_m: 12,
    },
    {
      split_index: 1,
      distance_m: 1000,
      duration_s: 145,
      avg_hr: 156,
      avg_power_w: 245,
      avg_speed_mps: 6.9,
      elevation_gain_m: 8,
    },
  ],
  sets: null,
};

// A minimal completed workout with no splits and no zone time — the graceful
// degrade case for the detail view.
export const bareWorkoutDetail: Workout = {
  id: "w2",
  sport: "running",
  status: "completed",
  started_at: "2026-06-24T06:00:00Z",
  ended_at: "2026-06-24T06:40:00Z",
  splits: null,
  sets: null,
};

export const populatedTrend: FitnessSnapshot[] = [
  { date: "2026-06-18", acute_load: 380, chronic_load: 370 },
  { date: "2026-06-20", acute_load: 410, chronic_load: 380 },
  { date: "2026-06-22", acute_load: 430, chronic_load: 388 },
  { date: "2026-06-24", acute_load: 420, chronic_load: 390 },
];

export const emptyTrend: FitnessSnapshot[] = [];
