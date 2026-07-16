package races

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinzenzs/kazper/internal/store"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrNameRequired         = errors.New("race_name_required")
	ErrRaceDateInvalid      = errors.New("race_date_invalid")
	ErrLegOrdinalDuplicate  = errors.New("leg_ordinal_duplicate")
	ErrLegDisciplineInvalid = errors.New("leg_discipline_invalid")
	ErrLegDurationInvalid   = errors.New("leg_expected_duration_min_invalid")
	ErrLegDistanceInvalid   = errors.New("leg_distance_m_invalid")
	ErrNotesTooLong         = errors.New("notes_too_long")
	ErrPriorityInvalid      = errors.New("race_priority_invalid")

	ErrBodyWeightRequired = errors.New("body_weight_kg_required")
	ErrBodyWeightRange    = errors.New("body_weight_kg_out_of_range")
	ErrSweatRateRange     = errors.New("sweat_rate_out_of_range")
)

const (
	bodyWeightKgMin = 30.0
	bodyWeightKgMax = 200.0
	sweatRateMlMax  = 5000.0
	maxNameLen      = 200
	maxNotesLen     = 2000
)

// Service orchestrates race CRUD and fuelling-plan computation. It holds the
// pool so race+legs writes are atomic; the repo runs against pool or tx.
type Service struct {
	pool *pgxpool.Pool
	repo *Repo
	// heat is optional: unwired leaves weather mode inert rather than erroring.
	heat HeatProvider
}

// SetHeatProvider enables `weather=true` on the fueling plan.
func (s *Service) SetHeatProvider(p HeatProvider) { s.heat = p }

func NewService(pool *pgxpool.Pool, repo *Repo) *Service {
	return &Service{pool: pool, repo: repo}
}

// LegInput is a leg supplied on create/update.
type LegInput struct {
	Ordinal             int
	Discipline          Discipline
	DistanceM           *float64
	ExpectedDurationMin *int
	Intensity           *string
}

// CreateInput is the payload for POST /races.
type CreateInput struct {
	Name     string
	RaceDate string
	RaceType *string
	Location *string
	Notes    *string
	Priority *string
	Legs     []LegInput
}

// Create validates and persists a race with its legs atomically.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Race, error) {
	if err := validateName(in.Name); err != nil {
		return nil, err
	}
	if err := validateNotes(in.Notes); err != nil {
		return nil, err
	}
	if err := validatePriority(in.Priority); err != nil {
		return nil, err
	}
	date, err := parseRaceDate(in.RaceDate)
	if err != nil {
		return nil, err
	}
	if err := validateLegs(in.Legs); err != nil {
		return nil, err
	}

	race := &Race{
		Name:     strings.TrimSpace(in.Name),
		RaceType: in.RaceType,
		Location: in.Location,
		Notes:    in.Notes,
		Priority: priorityEnumPtr(in.Priority),
	}
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := NewRepo(tx)
		if err := r.InsertRace(ctx, race, date); err != nil {
			return err
		}
		return insertLegs(ctx, r, race.ID, in.Legs)
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetRace(ctx, race.ID)
}

// Get returns a race with legs.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Race, error) {
	return s.repo.GetRace(ctx, id)
}

// List returns races with legs, optionally filtered to a single priority. A
// non-nil, invalid priority is rejected before touching the DB.
func (s *Service) List(ctx context.Context, priority *string) ([]*Race, error) {
	if err := validatePriority(priority); err != nil {
		return nil, err
	}
	return s.repo.ListRaces(ctx, priority)
}

// UpdateInput is the editable subset on PATCH /races/{id}. Nil scalar pointers
// leave the field unchanged; a non-nil Legs slice replaces all legs wholesale.
type UpdateInput struct {
	Name     *string
	RaceDate *string
	RaceType *string
	Location *string
	Notes    *string
	// Priority is tri-state: a non-nil value sets it; ClearPriority writes NULL;
	// neither leaves it unchanged. The handler converts the empty-string sentinel
	// into ClearPriority.
	Priority      *string
	ClearPriority bool
	Legs          *[]LegInput
}

// Update validates and applies a partial update, optionally replacing legs.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Race, error) {
	if in.Name != nil && strings.TrimSpace(*in.Name) == "" {
		return nil, ErrNameRequired
	}
	if in.Name != nil && len(*in.Name) > maxNameLen {
		return nil, ErrNameRequired
	}
	if err := validateNotes(in.Notes); err != nil {
		return nil, err
	}
	if err := validatePriority(in.Priority); err != nil {
		return nil, err
	}
	var datePtr *time.Time
	if in.RaceDate != nil {
		d, err := parseRaceDate(*in.RaceDate)
		if err != nil {
			return nil, err
		}
		datePtr = &d
	}
	if in.Legs != nil {
		if err := validateLegs(*in.Legs); err != nil {
			return nil, err
		}
	}

	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := NewRepo(tx)
		if err := r.UpdateRace(ctx, id, UpdateRaceParams{
			Name:          trimPtr(in.Name),
			RaceDate:      datePtr,
			RaceType:      in.RaceType,
			Location:      in.Location,
			Notes:         in.Notes,
			Priority:      in.Priority,
			ClearPriority: in.ClearPriority,
		}); err != nil {
			return err
		}
		if in.Legs != nil {
			if err := r.DeleteLegsForRace(ctx, id); err != nil {
				return err
			}
			return insertLegs(ctx, r, id, *in.Legs)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetRace(ctx, id)
}

// Delete removes a race (legs cascade).
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteRace(ctx, id)
}

// PlanFueling computes the per-leg fuelling plan for a stored race.
func (s *Service) PlanFueling(ctx context.Context, id uuid.UUID, p FuelingParams) (*FuelingPlan, error) {
	return s.PlanFuelingWithWeather(ctx, id, p, false)
}

// PlanFuelingWithWeather is the opt-in superset: with withWeather it resolves
// the race-day heat and scales fluid/sodium by a bounded multiplier. Without
// it, byte-identical to PlanFueling.
func (s *Service) PlanFuelingWithWeather(ctx context.Context, id uuid.UUID, p FuelingParams, withWeather bool) (*FuelingPlan, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	race, err := s.repo.GetRace(ctx, id)
	if err != nil {
		return nil, err
	}
	scaling, reason := s.resolveHeatScaling(ctx, race, withWeather)
	p.Heat = scaling
	plan := ComputeFueling(race, p)
	plan.HeatReason = reason
	return plan, nil
}

// ----- validators -----

func validateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrNameRequired
	}
	if len(name) > maxNameLen {
		return ErrNameRequired
	}
	return nil
}

func validateNotes(notes *string) error {
	if notes != nil && len(*notes) > maxNotesLen {
		return ErrNotesTooLong
	}
	return nil
}

// validatePriority rejects a non-nil priority outside the closed A|B|C set. A
// nil pointer (absent / clear-handled-elsewhere) is valid.
func validatePriority(p *string) error {
	if p != nil && !Priority(*p).valid() {
		return ErrPriorityInvalid
	}
	return nil
}

// priorityEnumPtr converts a validated *string into the typed *Priority stored
// on Race (nil passthrough).
func priorityEnumPtr(p *string) *Priority {
	if p == nil {
		return nil
	}
	v := Priority(*p)
	return &v
}

func parseRaceDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation(dateLayout, strings.TrimSpace(s), time.UTC)
	if err != nil {
		return time.Time{}, ErrRaceDateInvalid
	}
	return t, nil
}

func validateLegs(legs []LegInput) error {
	seen := map[int]bool{}
	for _, leg := range legs {
		if seen[leg.Ordinal] {
			return ErrLegOrdinalDuplicate
		}
		seen[leg.Ordinal] = true
		if !leg.Discipline.valid() {
			return ErrLegDisciplineInvalid
		}
		if leg.ExpectedDurationMin != nil && *leg.ExpectedDurationMin <= 0 {
			return ErrLegDurationInvalid
		}
		if leg.DistanceM != nil {
			d := *leg.DistanceM
			if math.IsNaN(d) || math.IsInf(d, 0) || d <= 0 {
				return ErrLegDistanceInvalid
			}
		}
	}
	return nil
}

func insertLegs(ctx context.Context, r *Repo, raceID uuid.UUID, legs []LegInput) error {
	for _, in := range legs {
		leg := &RaceLeg{
			Ordinal:             in.Ordinal,
			Discipline:          in.Discipline,
			DistanceM:           in.DistanceM,
			ExpectedDurationMin: in.ExpectedDurationMin,
			Intensity:           in.Intensity,
		}
		if err := r.InsertLeg(ctx, raceID, leg); err != nil {
			return err
		}
	}
	return nil
}

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	return &t
}
