# workouts — delta for add-training-plan

## MODIFIED Requirements

### Requirement: Workouts are stored in a dedicated table

The system SHALL persist workouts in a `workouts` table independent of meals, hydration, and products. Each row holds a sport, a time window (`started_at`, `ended_at`), provenance metadata, a `status` (`planned` or `completed`), optional intensity/burn signals, optional ingestion metrics (distance, average power, ambient temperature, estimated sweat loss), an optional session-group key linking the legs of a brick/multisport session, optional links to a `workout-template` and a training-plan `plan_slot` (for planned workouts originating from a plan), and audit timestamps. The table is the data shape that external writers — initially `garmin.py`, in future potentially Apple Health, Strava, or a manual UI — target via the REST endpoints, and that the training-plan materializer targets for planned sessions.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `workouts` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `external_id` (TEXT NULL)
  - `source` (TEXT NOT NULL, CHECK IN `('garmin', 'manual', 'other')`)
  - `sport` (TEXT NOT NULL, CHECK IN `('run', 'bike', 'swim', 'strength', 'other')`)
  - `status` (TEXT NOT NULL DEFAULT `'completed'`, CHECK IN `('planned', 'completed')`)
  - `name` (TEXT NULL)
  - `started_at` (TIMESTAMPTZ NOT NULL)
  - `ended_at` (TIMESTAMPTZ NOT NULL)
  - `kcal_burned` (NUMERIC(10, 1) NULL, CHECK `kcal_burned IS NULL OR kcal_burned > 0`)
  - `avg_hr` (INTEGER NULL, CHECK `avg_hr IS NULL OR avg_hr > 0`)
  - `tss` (NUMERIC(10, 2) NULL, CHECK `tss IS NULL OR tss >= 0`)
  - `rpe` (INTEGER NULL, CHECK `rpe IS NULL OR (rpe BETWEEN 1 AND 10)`)
  - `gi_distress_score` (INTEGER NULL, CHECK `gi_distress_score IS NULL OR (gi_distress_score BETWEEN 1 AND 5)`)
  - `distance_m` (NUMERIC(10, 1) NULL, CHECK `distance_m IS NULL OR distance_m > 0`)
  - `avg_power_w` (INTEGER NULL, CHECK `avg_power_w IS NULL OR avg_power_w > 0`)
  - `temperature_c` (NUMERIC(4, 1) NULL, CHECK `temperature_c IS NULL OR (temperature_c BETWEEN -40 AND 60)`)
  - `sweat_loss_ml` (NUMERIC(10, 1) NULL, CHECK `sweat_loss_ml IS NULL OR sweat_loss_ml > 0`)
  - `session_group` (TEXT NULL)
  - `template_id` (UUID NULL, REFERENCES `workout_templates(id)` ON DELETE SET NULL)
  - `plan_slot_id` (UUID NULL, REFERENCES `plan_slots(id)` ON DELETE SET NULL)
  - `notes` (TEXT NULL)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** a CHECK constraint enforces `ended_at > started_at`
- **AND** an index `workouts_started_at_idx` exists on `(started_at)`
- **AND** a partial UNIQUE index exists on `(external_id) WHERE external_id IS NOT NULL`
- **AND** a partial (non-unique) index `workouts_session_group_idx` exists on `(session_group) WHERE session_group IS NOT NULL`
- **AND** a partial UNIQUE index `workouts_plan_slot_id_key` exists on `(plan_slot_id) WHERE plan_slot_id IS NOT NULL`
- **AND** there is NO `intensity` column (TSS is the intensity signal; downstream tools derive bands at call time)

## ADDED Requirements

### Requirement: Planned workouts can originate from a training-plan slot via a slot-keyed upsert

The system SHALL support upserting a planned workout from a training-plan slot,
keyed on `plan_slot_id`. Such a row SHALL carry `status='planned'`, the slot's
template's `sport` and `name`, a `template_id`, and a `plan_slot_id`. Because the
key is `plan_slot_id` and imported activities never carry one, this path is
disjoint from the existing `external_id` UPSERT path: the two never collide.
Repeated upserts on the same slot SHALL update the same row rather than create a
new one. The upsert's update SHALL apply only where the existing row's `status`
is `planned`, so a workout already marked `completed` is never reverted or
overwritten by re-materialization.

#### Scenario: The slot upsert does not overwrite a completed workout

- **WHEN** a planned workout for a slot has been marked `completed` and the slot
  is upserted again
- **THEN** the existing completed row is left unchanged (the update is guarded by
  `status='planned'`)

#### Scenario: A planned workout upserts by slot, not external_id

- **WHEN** the training-plan materializer upserts a planned workout for a given
  `plan_slot_id` twice
- **THEN** exactly one planned `workouts` row exists for that slot, updated in place

#### Scenario: The slot-keyed and external_id paths do not collide

- **WHEN** a completed activity (with `external_id`, no `plan_slot_id`) and a
  planned workout (with `plan_slot_id`, no `external_id`) exist for the same date
- **THEN** both rows persist independently, each addressable by its own key
