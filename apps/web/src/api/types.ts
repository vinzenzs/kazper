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

// Mirrors internal/athleteconfig/types.go. Every field is nullable and already
// serialized into /context/training; the dashboard renders whatever is present.
export interface AthleteConfig {
  ftp_watts?: number | null;
  threshold_hr?: number | null;
  lactate_threshold_hr?: number | null;
  max_hr?: number | null;
  threshold_pace_sec_per_km?: number | null;
  threshold_swim_pace_sec_per_100m?: number | null;
  hr_zone_1_max?: number | null;
  hr_zone_2_max?: number | null;
  hr_zone_3_max?: number | null;
  hr_zone_4_max?: number | null;
  hr_zone_5_max?: number | null;
  power_zone_1_max?: number | null;
  power_zone_2_max?: number | null;
  power_zone_3_max?: number | null;
  power_zone_4_max?: number | null;
  power_zone_5_max?: number | null;
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

// Mirrors internal/personalrecords/types.go. `value` carries an accompanying
// `unit` (e.g. "s" for a time, "m" for a distance) so a row renders without a
// hard-coded PR-type lookup. `activity_id` is a Garmin external id (not a Kazper
// workout id), so it is display-only — not a link into /workouts/:id.
export interface PersonalRecord {
  id: string;
  external_id: string;
  pr_type: string;
  value: number | null;
  unit: string;
  activity_id?: string | null;
  achieved_at: string;
}

export interface PersonalRecordsList {
  personal_records: PersonalRecord[];
}

// Mirrors internal/achievements/types.go. `kind` is "badge" | "challenge".
export interface Achievement {
  id: string;
  external_id: string;
  kind: string;
  name: string;
  earned_at?: string | null;
  progress_pct?: number | null;
}

export interface AchievementsList {
  achievements: Achievement[];
}

// Mirrors internal/gear/types.go. `gear_type` is "shoes" | "bike" | "other";
// distance is metres on the wire (rendered as km). `retired` gear is dimmed.
export interface Gear {
  id: string;
  external_id: string;
  gear_type: string;
  display_name: string;
  total_distance_m?: number | null;
  total_activities?: number | null;
  retired: boolean;
  date_begin?: string | null;
  date_end?: string | null;
}

export interface GearList {
  gear: Gear[];
}

// Mirrors internal/workoutstats/types.go. One bucket per calendar day (date set)
// or the window total (date absent). by_sport is count-per-sport. Distance and
// elevation are metres on the wire; duration is elapsed minutes.
export interface WorkoutStatsBucket {
  date?: string;
  count: number;
  total_duration_min: number;
  total_distance_m: number;
  total_elevation_gain_m: number;
  total_kcal: number;
  by_sport: Record<string, number>;
}

// The GET /workouts/summary response: a per-day series (every calendar day in
// [from, to], zero-filled) plus the window total.
export interface WorkoutStats {
  from: string;
  to: string;
  tz: string;
  days: WorkoutStatsBucket[];
  total: WorkoutStatsBucket;
}

// Mirrors internal/effortanalytics/types.go. One point per ladder duration: the
// best mean value across the window and the workout/day it came from. `value` is
// watts when metric is "power", m/s when metric is "speed" (rendered as pace).
export interface CurvePoint {
  duration_s: number;
  value: number;
  workout_id: string;
  date: string;
}

export interface PowerCurve {
  from: string;
  to: string;
  tz: string;
  sport: string;
  metric: "power" | "speed";
  points: CurvePoint[];
}

// Mirrors internal/effortanalytics CP model. The 2-parameter critical-power fit
// over the window's power best-efforts (2–30 min). Advisory: `cp_watts` ≈ FTP,
// compared against the configured FTP by the reader. `model` is null when the
// window can't support a fit, with `reason` naming the gate.
export interface CPModel {
  cp_watts: number;
  w_prime_kj: number;
  r_squared: number;
  rmse_w: number;
}

export interface CPPoint {
  duration_s: number;
  watts: number;
  workout_id: string;
  date: string;
}

export interface CPModelResult {
  from: string;
  to: string;
  tz: string;
  model: CPModel | null;
  reason?: "insufficient_points" | "span_too_narrow";
  points: CPPoint[];
}

// Mirrors internal/effortanalytics CP-model history: the CP2 fit at weekly
// (Monday) anchors, each over a trailing window. `model` is null (with `reason`)
// where the window couldn't support a fit — the trend gaps, not zeroes.
export interface CPHistoryAnchor {
  date: string;
  model: CPModel | null;
  reason?: string;
}

export interface CPModelHistoryResult {
  from: string;
  to: string;
  tz: string;
  window_days: number;
  anchors: CPHistoryAnchor[];
}

// Mirrors internal/athleteconfig threshold history (subset): the configured FTP
// as of each effective date, for the CP-trend overlay.
export interface ThresholdSnapshot {
  effective_from: string;
  ftp_watts?: number | null;
}

export interface ThresholdHistory {
  history: ThresholdSnapshot[];
}

// Mirrors internal/effortanalytics durability. Per duration (1m/5m/20m): the
// fresh (tier-0) best power and each kJ-tier's best with `fade_pct`. `reason` is
// "no_tiered_data" when the window holds only fresh rows (recompute backfill).
export interface DurabilityPoint {
  watts: number;
  workout_id: string;
  date: string;
}

export interface DurabilityTierPoint {
  kj_tier: number;
  watts: number;
  fade_pct: number;
  workout_id: string;
  date: string;
}

export interface DurabilityDuration {
  duration_s: number;
  fresh: DurabilityPoint | null;
  tiers: DurabilityTierPoint[];
}

export interface DurabilityResult {
  from: string;
  to: string;
  tz: string;
  durations: DurabilityDuration[];
  reason?: "no_tiered_data";
}

// Mirrors internal/effortanalytics power-profile ranking. Each anchor ranks a
// benchmark duration's W/kg against the Coggan tables: `category` is the
// authoritative band, `percentile` an interpolated estimate. `phenotype` is null
// unless all four anchors are present. Advisory — no athlete-config coupling.
export interface PowerProfileAnchor {
  label: "neuromuscular" | "anaerobic" | "vo2max" | "threshold";
  duration_s: number;
  watts: number;
  w_per_kg: number;
  category: string;
  percentile: number;
  workout_id: string;
  date: string;
}

export interface PowerProfileResult {
  from: string;
  to: string;
  tz: string;
  sex: "male" | "female";
  weight_kg: number;
  weight_source: "param" | "stored";
  anchors: PowerProfileAnchor[];
  missing_anchors: string[];
  phenotype: "sprinter" | "time_trialist" | "climber" | "all_rounder" | null;
}

// Mirrors internal/activitystreams W′-balance. The anaerobic-battery story of a
// ride, computed from its stored power stream + explicit CP/W′ params.
// `min_w_prime_kj` can be negative / `max_depletion_pct` over 100 when the
// supplied W′ is too low. `series` is kJ per (downsampled) point; absent when
// requested summary-only.
export interface WPrimeSummary {
  min_w_prime_kj: number;
  min_at_s: number;
  end_w_prime_kj: number;
  max_depletion_pct: number;
  time_below_25_pct_s: number;
}

export interface WPrimeBalanceResult {
  workout_id: string;
  params: { cp_watts: number; w_prime_kj: number };
  duration_s: number;
  summary: WPrimeSummary;
  downsample?: number;
  series?: number[];
}

// Mirrors internal/activitystreams quadrant analysis. Force/velocity quadrant
// shares over a ride's paired power+cadence samples. `scatter` is the (down-
// sampled) paired points; absent under summary_only.
export interface QuadrantSummary {
  q1_pct: number;
  q2_pct: number;
  q3_pct: number;
  q4_pct: number;
  pedaling_s: number;
  excluded_s: number;
  aepf_ref_n: number;
  cpv_ref_mps: number;
}

export interface QuadrantPoint {
  aepf_n: number;
  cpv_mps: number;
}

export interface QuadrantResult {
  workout_id: string;
  params: { cp_watts: number; cadence_rpm: number; crank_mm: number };
  summary: QuadrantSummary;
  scatter?: QuadrantPoint[];
}

// Mirrors internal/activitystreams interval detection. `threshold_w` is the
// Otsu-derived work/rest split (null with reason "no_distinct_efforts" when the
// ride isn't meaningfully bimodal). Advisory — a compute-on-read read.
export interface DetectedInterval {
  n: number;
  start_s: number;
  end_s: number;
  duration_s: number;
  avg_w: number;
  max_w: number;
  kj: number;
}

export interface IntervalRest {
  after_n: number;
  duration_s: number;
  avg_w: number;
}

export interface IntervalsResult {
  workout_id: string;
  threshold_w: number | null;
  intervals: DetectedInterval[];
  rests: IntervalRest[];
  reason?: "no_distinct_efforts";
  summary: {
    count: number;
    work_total_s: number;
    mean_effort_s: number;
    mean_effort_w: number;
  };
}

// Mirrors internal/pmc/types.go. The Coggan Performance Management Chart: one
// entry per calendar day with fitness (ctl), fatigue (atl), form (tsb), and the
// 7-day ramp rate, plus weekly overreaching flags.
export interface PMCDay {
  date: string;
  tss_total: number;
  ctl: number;
  atl: number;
  tsb: number;
  ramp_rate: number;
  missing_tss_count?: number;
}

export interface PMCRampAlert {
  week_start: string;
  ctl_start: number;
  ctl_end: number;
  ctl_delta: number;
}

export interface PMCSeries {
  from: string;
  to: string;
  tz: string;
  sport?: string;
  seed_date?: string;
  days: PMCDay[];
  ramp_alerts: PMCRampAlert[];
  missing_tss_workouts: number;
}

// Mirrors internal/pmc target-trajectory. The target CTL curve implied by the
// macrocycle's declared phase targets, beside measured CTL. `trajectory` is null
// (with reason "targets_missing") when no phase declares a target.
export interface TargetDay {
  date: string;
  target_ctl: number;
  target_declared: boolean;
  actual_ctl?: number;
  delta?: number;
}

export interface TargetSummary {
  current_delta: number;
  delta_trend_14d: number;
  projected_end_ctl_planned: number;
  projected_end_ctl_current: number;
}

export interface TargetTrajectory {
  macrocycle: { id: string; name: string; start_date: string; end_date: string };
  tz: string;
  seed_ctl: number;
  trajectory: TargetDay[] | null;
  reason?: "targets_missing";
  summary?: TargetSummary;
  missing_tss_workouts: number;
}

// Mirrors internal/workoutstats intensity distribution. Zone shares of zoned
// time (not elapsed), with a band collapse + advisory classification label.
export interface ZoneShare {
  zone: number;
  secs: number;
  share_pct?: number;
}

export interface IntensityBands {
  low_pct: number;
  moderate_pct: number;
  high_pct: number;
}

export interface ZoneAggregate {
  workouts_counted: number;
  total_zone_secs: number;
  zones: ZoneShare[];
}

export interface IntensityTotal extends ZoneAggregate {
  bands: IntensityBands;
  classification: string | null;
}

export interface IntensityWeek extends ZoneAggregate {
  week_start: string;
  missing_zone_data_count?: number;
}

export interface IntensityDistribution {
  from: string;
  to: string;
  tz: string;
  total: IntensityTotal;
  by_sport: Record<string, ZoneAggregate>;
  weekly: IntensityWeek[];
  by_training_focus: Record<string, number>;
  unclassified_focus_count: number;
  missing_zone_data_count: number;
}

// Mirrors a workout_splits row (internal/workouts/types.go). One per lap; all
// metrics nullable. avg_speed_mps is stored sport-agnostically (pace derivable).
export interface Split {
  split_index: number;
  distance_m?: number | null;
  duration_s?: number | null;
  avg_hr?: number | null;
  avg_power_w?: number | null;
  avg_speed_mps?: number | null;
  elevation_gain_m?: number | null;
}

// Mirrors a workout_sets row — one per strength set; all fields nullable.
export interface WorkoutSet {
  set_index: number;
  exercise_name?: string | null;
  exercise_category?: string | null;
  reps?: number | null;
  weight_kg?: number | null;
  duration_s?: number | null;
}

// Mirrors the single-get GET /workouts/:id response (internal/workouts/types.go).
// The list-shaped WorkoutLite omits the nested splits/sets and the extended
// scalar/zone detail; this detail shape carries them. Every metric is nullable.
export interface Workout {
  id: string;
  sport: string;
  status: string;
  name?: string | null;
  started_at: string;
  ended_at: string;
  kcal_burned?: number | null;
  avg_hr?: number | null;
  max_hr?: number | null;
  tss?: number | null;
  distance_m?: number | null;
  elevation_gain_m?: number | null;
  avg_power_w?: number | null;
  normalized_power_w?: number | null;
  intensity_factor?: number | null;
  avg_cadence?: number | null;
  secs_in_zone_1?: number | null;
  secs_in_zone_2?: number | null;
  secs_in_zone_3?: number | null;
  secs_in_zone_4?: number | null;
  secs_in_zone_5?: number | null;
  splits?: Split[] | null;
  sets?: WorkoutSet[] | null;
}
