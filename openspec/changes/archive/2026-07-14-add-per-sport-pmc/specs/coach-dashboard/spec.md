## ADDED Requirements

### Requirement: The PMC panel offers a sport selector

The dashboard's `/stats` PMC panel SHALL offer an All/Bike/Run/Swim selector (All default,
preserving the existing view) that re-fetches the PMC with the corresponding `sport` filter. Any
plan-target overlay SHALL render only on All; a filtered fetch failure SHALL fall back to the
combined view rather than an empty panel.

#### Scenario: Selecting a sport re-renders the filtered series

- **WHEN** the user selects Run
- **THEN** the panel renders the run-filtered CTL/ATL/TSB series

#### Scenario: All remains the default

- **WHEN** `/stats` loads
- **THEN** the PMC panel shows the combined series exactly as before this change
