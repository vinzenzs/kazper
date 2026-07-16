## ADDED Requirements

### Requirement: The bridge derives each activity's environment from its Garmin type

The bridge SHALL map Garmin's activity type to the workout `environment` field in the bulk
items it posts: indoor cycling / virtual rides / treadmill running / indoor rowing / pool
swimming → `indoor`; road, gravel, mountain, open-water, and outdoor run types → `outdoor`;
an unrecognized type SHALL omit the field and SHALL NOT fail the mapping or the sync. Pool
swims map to `indoor` deliberately — the field expresses whether ambient weather applied.

#### Scenario: A virtual ride arrives marked indoor

- **WHEN** a synced activity carries a virtual/indoor cycling type
- **THEN** its bulk item includes `environment: "indoor"`

#### Scenario: An unknown activity type stays unmarked

- **WHEN** Garmin reports a type outside the mapping
- **THEN** the item omits `environment` and the sync proceeds normally
