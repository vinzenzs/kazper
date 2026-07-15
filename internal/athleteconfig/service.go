package athleteconfig

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinzenzs/kazper/internal/store"
)

// ValidationError carries the spec-defined error code plus the offending field
// name. All athlete-config validation failures use the single code
// `athlete_config_value_invalid` with a `field` hint.
type ValidationError struct {
	Field string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("athlete_config_value_invalid: %s", e.Field)
}

// Service orchestrates the athlete-config singleton over the repo. It holds the
// pool so the singleton upsert and its history snapshot run in one transaction.
type Service struct {
	repo *Repo
	pool *pgxpool.Pool
}

func NewService(repo *Repo, pool *pgxpool.Pool) *Service {
	return &Service{repo: repo, pool: pool}
}

// Get returns the singleton config, or (nil, nil) when none has been written.
func (s *Service) Get(ctx context.Context) (*AthleteConfig, error) {
	return s.repo.Get(ctx)
}

// Put validates and full-replaces the singleton config, maintaining the append-
// only history in the same transaction, then reads the singleton back. The GET/
// PUT request+response contract is unchanged; history is a pure side effect.
//
// History maintenance: snapshot today's full state only when it differs from the
// prior singleton (a no-change PUT — the daily Garmin re-PUT — touches history
// not at all). A same-day change replaces today's row (PK on effective_from); a
// same-day revert to the state in effect the day before deletes today's row, so
// no two consecutive history rows are ever physiology-identical.
func (s *Service) Put(ctx context.Context, cfg *AthleteConfig) (*AthleteConfig, error) {
	if err := validate(cfg); err != nil {
		return nil, err
	}
	today := truncateToDate(time.Now().UTC())
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := NewRepo(tx)
		prior, err := r.Get(ctx)
		if err != nil {
			return err
		}
		if err := r.Upsert(ctx, cfg); err != nil {
			return err
		}
		if prior != nil && physiologyEqual(prior, cfg) {
			return nil // no physiology change → history untouched
		}
		// State changed. If it reverts to the day-before snapshot, collapse the
		// same-day row instead of recording a duplicate.
		latest, err := r.LatestBefore(ctx, today)
		if err != nil {
			return err
		}
		if latest != nil && physiologyEqual(&latest.AthleteConfig, cfg) {
			return r.DeleteSnapshot(ctx, today)
		}
		return r.UpsertSnapshot(ctx, today, cfg)
	})
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx)
}

// SourceFieldError carries the `source_field_invalid` code plus the offending
// token, raised when PUT /athlete-config/sources includes a non-whitelisted
// field.
type SourceFieldError struct {
	Field string
}

func (e *SourceFieldError) Error() string {
	return fmt.Sprintf("source_field_invalid: %s", e.Field)
}

// sourceWhitelist is the set of tokens PUT /athlete-config/sources accepts.
var sourceWhitelist = map[string]struct{}{
	SourceFTPWatts:           {},
	SourceLactateThresholdHR: {},
	SourceMaxHR:              {},
	SourceThresholdPace:      {},
	SourceHRZones:            {},
	SourcePowerZones:         {},
}

// GetDetection returns the latest Garmin-detected thresholds, or (nil, nil) when
// none has been recorded.
func (s *Service) GetDetection(ctx context.Context) (*GarminDetectedThresholds, error) {
	return s.repo.GetDetection(ctx)
}

// PutDetection validates and full-replaces the detection singleton, then reads
// it back. It deliberately never reads or mutates athlete_config or
// threshold_history — a detection is advisory evidence, applied only through the
// deliberate PUT /athlete-config flow.
func (s *Service) PutDetection(ctx context.Context, d *GarminDetectedThresholds) (*GarminDetectedThresholds, error) {
	if err := validateDetection(d); err != nil {
		return nil, err
	}
	if err := s.repo.UpsertDetection(ctx, d); err != nil {
		return nil, err
	}
	return s.repo.GetDetection(ctx)
}

// GetSources returns the active source policy (empty slice, never nil).
func (s *Service) GetSources(ctx context.Context) ([]string, error) {
	return s.repo.GetSources(ctx)
}

// PutSources validates the tokens against the whitelist and full-replaces the
// policy, mutating only the policy column. Duplicate tokens are collapsed,
// preserving first-seen order.
func (s *Service) PutSources(ctx context.Context, sources []string) ([]string, error) {
	normalized := make([]string, 0, len(sources))
	seen := map[string]struct{}{}
	for _, f := range sources {
		if _, ok := sourceWhitelist[f]; !ok {
			return nil, &SourceFieldError{Field: f}
		}
		if _, dup := seen[f]; dup {
			continue
		}
		seen[f] = struct{}{}
		normalized = append(normalized, f)
	}
	if err := s.repo.PutSources(ctx, normalized); err != nil {
		return nil, err
	}
	return s.repo.GetSources(ctx)
}

// History returns the dated snapshots ascending, optionally bounded inclusively.
func (s *Service) History(ctx context.Context, from, to *time.Time) ([]*ThresholdSnapshot, error) {
	return s.repo.ListHistory(ctx, from, to)
}

// ConfigAsOf returns the physiology state in effect on date (latest snapshot with
// effective_from <= date), or (nil, nil) when history is empty. Provided as a
// resolution primitive; deliberately not wired into any existing consumer here.
func (s *Service) ConfigAsOf(ctx context.Context, date time.Time) (*ThresholdSnapshot, error) {
	return s.repo.AsOf(ctx, truncateToDate(date))
}

func truncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// physiologyEqual compares the 16 physiology fields of two configs pointer-aware
// (timestamps excluded). Two nil pointers are equal; a nil and a value differ.
func physiologyEqual(a, b *AthleteConfig) bool {
	return intEq(a.FtpWatts, b.FtpWatts) &&
		intEq(a.ThresholdHR, b.ThresholdHR) &&
		intEq(a.LactateThresholdHR, b.LactateThresholdHR) &&
		intEq(a.MaxHR, b.MaxHR) &&
		floatEq(a.ThresholdPaceSecPerKm, b.ThresholdPaceSecPerKm) &&
		floatEq(a.ThresholdSwimPaceSecPer100m, b.ThresholdSwimPaceSecPer100m) &&
		intEq(a.HRZone1Max, b.HRZone1Max) && intEq(a.HRZone2Max, b.HRZone2Max) &&
		intEq(a.HRZone3Max, b.HRZone3Max) && intEq(a.HRZone4Max, b.HRZone4Max) &&
		intEq(a.HRZone5Max, b.HRZone5Max) &&
		intEq(a.PowerZone1Max, b.PowerZone1Max) && intEq(a.PowerZone2Max, b.PowerZone2Max) &&
		intEq(a.PowerZone3Max, b.PowerZone3Max) && intEq(a.PowerZone4Max, b.PowerZone4Max) &&
		intEq(a.PowerZone5Max, b.PowerZone5Max)
}

func intEq(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func floatEq(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// validate rejects any present field that is not strictly positive (matching
// the column CHECKs) or, for floats, not finite. Each field is independent.
func validate(cfg *AthleteConfig) error {
	ints := []struct {
		name string
		v    *int
	}{
		{"ftp_watts", cfg.FtpWatts},
		{"threshold_hr", cfg.ThresholdHR},
		{"lactate_threshold_hr", cfg.LactateThresholdHR},
		{"max_hr", cfg.MaxHR},
		{"hr_zone_1_max", cfg.HRZone1Max},
		{"hr_zone_2_max", cfg.HRZone2Max},
		{"hr_zone_3_max", cfg.HRZone3Max},
		{"hr_zone_4_max", cfg.HRZone4Max},
		{"hr_zone_5_max", cfg.HRZone5Max},
		{"power_zone_1_max", cfg.PowerZone1Max},
		{"power_zone_2_max", cfg.PowerZone2Max},
		{"power_zone_3_max", cfg.PowerZone3Max},
		{"power_zone_4_max", cfg.PowerZone4Max},
		{"power_zone_5_max", cfg.PowerZone5Max},
	}
	for _, f := range ints {
		if f.v != nil && *f.v <= 0 {
			return &ValidationError{Field: f.name}
		}
	}
	floats := []struct {
		name string
		v    *float64
	}{
		{"threshold_pace_sec_per_km", cfg.ThresholdPaceSecPerKm},
		{"threshold_swim_pace_sec_per_100m", cfg.ThresholdSwimPaceSecPer100m},
	}
	for _, f := range floats {
		if f.v != nil && (math.IsNaN(*f.v) || math.IsInf(*f.v, 0) || *f.v <= 0) {
			return &ValidationError{Field: f.name}
		}
	}
	return nil
}

// validateDetection applies the same positivity/finiteness rules to a detection
// payload (the DB CHECKs enforce them too; this surfaces a clean field error).
func validateDetection(d *GarminDetectedThresholds) error {
	ints := []struct {
		name string
		v    *int
	}{
		{"ftp_watts", d.FtpWatts},
		{"lactate_threshold_hr", d.LactateThresholdHR},
		{"max_hr", d.MaxHR},
		{"hr_zone_1_max", d.HRZone1Max},
		{"hr_zone_2_max", d.HRZone2Max},
		{"hr_zone_3_max", d.HRZone3Max},
		{"hr_zone_4_max", d.HRZone4Max},
		{"hr_zone_5_max", d.HRZone5Max},
		{"power_zone_1_max", d.PowerZone1Max},
		{"power_zone_2_max", d.PowerZone2Max},
		{"power_zone_3_max", d.PowerZone3Max},
		{"power_zone_4_max", d.PowerZone4Max},
		{"power_zone_5_max", d.PowerZone5Max},
	}
	for _, f := range ints {
		if f.v != nil && *f.v <= 0 {
			return &ValidationError{Field: f.name}
		}
	}
	if d.ThresholdPaceSecPerKm != nil {
		v := *d.ThresholdPaceSecPerKm
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
			return &ValidationError{Field: "threshold_pace_sec_per_km"}
		}
	}
	return nil
}
