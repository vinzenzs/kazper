// Types mirror the Go response shapes returned by the Kazper REST API. Every
// metric is nullable (a pointer in Go) — the dashboard must degrade gracefully
// when a field is absent rather than assuming a value. See
// internal/coachcontext/types.go, internal/fitnessmetrics/types.go and
// internal/recoverymetrics/types.go.

export interface PhaseLite {
  id: string;
  name: string;
  type: string;
  start_date: string;
  end_date: string;
  methodology: string | null;
}

export interface MacrocycleLite {
  id: string;
  name: string;
  start_date: string;
  end_date: string;
  race_id: string | null;
  race_name: string | null;
  race_date: string | null;
  days_to_race: number | null;
  current_phase_ordinal: number | null;
  total_periods: number;
}

export interface FitnessSnapshot {
  date: string;
  vo2max_running?: number | null;
  vo2max_cycling?: number | null;
  race_predictor_5k_seconds?: number | null;
  race_predictor_10k_seconds?: number | null;
  race_predictor_half_seconds?: number | null;
  race_predictor_full_seconds?: number | null;
  acute_load?: number | null;
  chronic_load?: number | null;
  endurance_score?: number | null;
  hill_score?: number | null;
  fitness_age?: number | null;
  training_status?: string | null;
}

export interface RecoverySnapshot {
  date: string;
  sleep_seconds?: number | null;
  sleep_score?: number | null;
  hrv_ms?: number | null;
  resting_hr?: number | null;
  stress_avg?: number | null;
  body_battery_charged?: number | null;
  body_battery_drained?: number | null;
  training_readiness?: number | null;
}

export interface WorkoutLite {
  id: string;
  sport: string;
  status: string;
  name?: string | null;
  started_at: string;
  ended_at: string;
  duration_min: number;
  kcal_burned?: number | null;
  tss?: number | null;
}

export interface LoadSummary {
  count: number;
  total_duration_min: number;
  total_kcal: number;
  by_sport: Record<string, number>;
}

export interface AthleteConfig {
  ftp_watts?: number | null;
  [key: string]: unknown;
}

export interface TrainingContext {
  date: string;
  tz: string;
  lookback_days: number;
  lookahead_days: number;
  phase: PhaseLite | null;
  macrocycle: MacrocycleLite | null;
  fitness: FitnessSnapshot | null;
  acwr: number | null;
  athlete_config: AthleteConfig | null;
  watts_per_kg: number | null;
  recent_load: LoadSummary;
  recent_workouts: WorkoutLite[] | null;
  upcoming_workouts: WorkoutLite[] | null;
}

export interface RecoveryContext {
  date: string;
  days: number;
  latest: RecoverySnapshot | null;
  recent: RecoverySnapshot[] | null;
}

export interface FitnessMetricsList {
  fitness_metrics: FitnessSnapshot[];
}
