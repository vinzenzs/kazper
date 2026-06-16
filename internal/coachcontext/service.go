package coachcontext

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/fitnessmetrics"
	"github.com/vinzenzs/kazper/internal/multisport"
	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/recoverymetrics"
	"github.com/vinzenzs/kazper/internal/trainingphases"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// multisportRepo is the narrow read view the training-context load summary needs
// to decompose a multisport workout into its segment sports. Satisfied by
// *multisport.Repo; cross-injected (nil-safe) so multisport stays optional.
type multisportRepo interface {
	GetByID(ctx context.Context, id string) (*multisport.Template, error)
}

// Window defaults and clamps for the aggregate reads.
const (
	defaultLookbackDays  = 14
	defaultLookaheadDays = 7
	defaultRecoveryDays  = 7
	maxLookbackDays      = 90
	maxLookaheadDays     = 60
	maxRecoveryDays      = 90
)

// Service composes the coach context bundles from existing read repos. Wide
// constructor by design (mirrors internal/dailycontext): each dep is a
// load-bearing reader.
type Service struct {
	workoutsRepo      *workouts.Repo
	fitnessRepo       *fitnessmetrics.Repo
	recoveryRepo      *recoverymetrics.Repo
	phasesRepo        *trainingphases.PhasesRepo
	athleteConfigRepo *athleteconfig.Repo
	bodyWeightRepo    *bodyweight.Repo
	multisportRepo    multisportRepo
}

// SetMultisportRepo cross-injects the multisport-template repo the recent-load
// summary uses to decompose a multisport workout into its segment sports.
// Optional: when unset, a multisport workout stays a single `multisport` entry
// in by_sport. Wired in httpserver.Run() to keep multisport optional and avoid
// an import cycle.
func (s *Service) SetMultisportRepo(r multisportRepo) { s.multisportRepo = r }

func NewService(
	workoutsRepo *workouts.Repo,
	fitnessRepo *fitnessmetrics.Repo,
	recoveryRepo *recoverymetrics.Repo,
	phasesRepo *trainingphases.PhasesRepo,
	athleteConfigRepo *athleteconfig.Repo,
	bodyWeightRepo *bodyweight.Repo,
) *Service {
	return &Service{
		workoutsRepo:      workoutsRepo,
		fitnessRepo:       fitnessRepo,
		recoveryRepo:      recoveryRepo,
		phasesRepo:        phasesRepo,
		athleteConfigRepo: athleteConfigRepo,
		bodyWeightRepo:    bodyWeightRepo,
	}
}

// clamp bounds n to [lo, hi], substituting def when n <= 0 (unset).
func clamp(n, def, lo, hi int) int {
	if n <= 0 {
		n = def
	}
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

// BuildTraining returns the training context bundle for date (a calendar day in
// loc). All slice reads run in parallel; any failure cancels and returns an
// error — no partial bundle.
func (s *Service) BuildTraining(ctx context.Context, date time.Time, loc *time.Location, lookbackDays, lookaheadDays int) (*TrainingContext, error) {
	lookbackDays = clamp(lookbackDays, defaultLookbackDays, 1, maxLookbackDays)
	lookaheadDays = clamp(lookaheadDays, defaultLookaheadDays, 0, maxLookaheadDays)

	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.Add(24 * time.Hour)
	dateStr := date.Format("2006-01-02")

	out := &TrainingContext{
		Date:             dateStr,
		TZ:               loc.String(),
		LookbackDays:     lookbackDays,
		LookaheadDays:    lookaheadDays,
		RecentWorkouts:   []*WorkoutLite{},
		UpcomingWorkouts: []*WorkoutLite{},
		RecentLoad:       LoadSummary{BySport: map[string]int{}},
	}

	g, gctx := errgroup.WithContext(ctx)

	// Phase covering the date.
	g.Go(func() error {
		p, err := s.phasesRepo.PhaseFor(gctx, date)
		if err != nil {
			if errors.Is(err, trainingphases.ErrPhaseNotFound) {
				return nil
			}
			return fmt.Errorf("phase for date: %w", err)
		}
		out.Phase = &PhaseLite{ID: p.ID, Name: p.Name, Type: p.Type, StartDate: p.StartDate, EndDate: p.EndDate, Methodology: p.Methodology}
		return nil
	})

	// Latest fitness snapshot on/before the date, within the lookback window.
	g.Go(func() error {
		fromStr := dayStart.AddDate(0, 0, -lookbackDays).Format("2006-01-02")
		snaps, err := s.fitnessRepo.List(gctx, fromStr, dateStr)
		if err != nil {
			return fmt.Errorf("fitness list: %w", err)
		}
		if len(snaps) > 0 {
			latest := snaps[len(snaps)-1] // List is ascending by date
			out.Fitness = latest
			out.ACWR = acwr(latest)
		}
		return nil
	})

	// Athlete physiology config + derived power-to-weight. Both reads are cheap
	// (singleton + indexed latest-before); a missing config row or no bodyweight
	// leaves the fields nil rather than erroring.
	g.Go(func() error {
		cfg, err := s.athleteConfigRepo.Get(gctx)
		if err != nil {
			return fmt.Errorf("athlete config: %w", err)
		}
		out.AthleteConfig = cfg
		bw, err := s.bodyWeightRepo.LatestBefore(gctx, dayEnd.UTC())
		if err != nil {
			if errors.Is(err, bodyweight.ErrNotFound) {
				return nil
			}
			return fmt.Errorf("latest bodyweight: %w", err)
		}
		out.WattsPerKg = wattsPerKg(cfg, bw.WeightKg)
		return nil
	})

	// Recent completed workouts in [date-lookback, date].
	g.Go(func() error {
		from := dayStart.AddDate(0, 0, -lookbackDays)
		completed := string(workouts.StatusCompleted)
		ws, err := s.workoutsRepo.List(gctx, from.UTC(), dayEnd.UTC(), nil, &completed)
		if err != nil {
			return fmt.Errorf("recent workouts: %w", err)
		}
		out.RecentWorkouts = toLite(ws)
		out.RecentLoad = summarize(ws, s.segmentSportResolver(gctx))
		return nil
	})

	// Upcoming planned workouts in [date, date+lookahead].
	g.Go(func() error {
		to := dayStart.AddDate(0, 0, lookaheadDays+1) // inclusive of the lookahead day
		planned := string(workouts.StatusPlanned)
		ws, err := s.workoutsRepo.List(gctx, dayStart.UTC(), to.UTC(), nil, &planned)
		if err != nil {
			return fmt.Errorf("upcoming workouts: %w", err)
		}
		out.UpcomingWorkouts = toLite(ws)
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}

// BuildRecovery returns the recovery context bundle for date: the latest
// snapshot on/before date within the window plus the window's trend.
func (s *Service) BuildRecovery(ctx context.Context, date time.Time, days int) (*RecoveryContext, error) {
	days = clamp(days, defaultRecoveryDays, 1, maxRecoveryDays)
	dateStr := date.Format("2006-01-02")
	fromStr := date.AddDate(0, 0, -days).Format("2006-01-02")

	snaps, err := s.recoveryRepo.List(ctx, fromStr, dateStr)
	if err != nil {
		return nil, fmt.Errorf("recovery list: %w", err)
	}
	out := &RecoveryContext{Date: dateStr, Days: days, Recent: snaps}
	if out.Recent == nil {
		out.Recent = []*recoverymetrics.Snapshot{}
	}
	if len(snaps) > 0 {
		out.Latest = snaps[len(snaps)-1] // ascending → last is most recent
	}
	return out, nil
}

// acwr derives the acute:chronic workload ratio, rounded, only when both loads
// are present and chronic is non-zero.
func acwr(f *fitnessmetrics.Snapshot) *float64 {
	if f == nil || f.AcuteLoad == nil || f.ChronicLoad == nil || *f.ChronicLoad == 0 {
		return nil
	}
	v := numfmt.Round1(*f.AcuteLoad / *f.ChronicLoad)
	return &v
}

// wattsPerKg derives power-to-weight, rounded, only when an FTP is configured
// and bodyweight is positive.
func wattsPerKg(cfg *athleteconfig.AthleteConfig, bwKg float64) *float64 {
	if cfg == nil || cfg.FtpWatts == nil || bwKg <= 0 {
		return nil
	}
	v := numfmt.Round1(float64(*cfg.FtpWatts) / bwKg)
	return &v
}

func toLite(ws []*workouts.Workout) []*WorkoutLite {
	out := make([]*WorkoutLite, 0, len(ws))
	for _, w := range ws {
		out = append(out, &WorkoutLite{
			ID:          w.ID,
			Sport:       string(w.Sport),
			Status:      string(w.Status),
			Name:        w.Name,
			StartedAt:   w.StartedAt,
			EndedAt:     w.EndedAt,
			DurationMin: numfmt.Round1(w.EndedAt.Sub(w.StartedAt).Minutes()),
			KcalBurned:  w.KcalBurned,
			TSS:         w.TSS,
		})
	}
	return out
}

// summarize aggregates a workout window. resolveSegments, when non-nil, maps a
// multisport workout's template id to its non-transition segment sports so a
// brick credits each leg in by_sport; count/duration/kcal still treat it as one
// session. A nil resolver (or an unresolved id) leaves the workout as a single
// `multisport` entry.
func summarize(ws []*workouts.Workout, resolveSegments func(templateID string) ([]string, bool)) LoadSummary {
	s := LoadSummary{BySport: map[string]int{}}
	var dur, kcal float64
	for _, w := range ws {
		s.Count++
		dur += w.EndedAt.Sub(w.StartedAt).Minutes()
		if w.KcalBurned != nil {
			kcal += *w.KcalBurned
		}
		if w.Sport == workouts.SportMultisport && w.MultisportTemplateID != nil && resolveSegments != nil {
			if sports, ok := resolveSegments(w.MultisportTemplateID.String()); ok {
				for _, sp := range sports {
					s.BySport[sp]++
				}
				continue
			}
		}
		s.BySport[string(w.Sport)]++
	}
	s.TotalDurationMin = numfmt.Round1(dur)
	s.TotalKcal = numfmt.Round1(kcal)
	return s
}

// segmentSportResolver returns a best-effort resolver from a multisport
// template id to its non-transition segment sports, memoized within the call.
// Returns ok=false (caller falls back to the `multisport` bucket) when the repo
// is unset, the template is missing, or the load fails — never errors.
func (s *Service) segmentSportResolver(ctx context.Context) func(string) ([]string, bool) {
	if s.multisportRepo == nil {
		return func(string) ([]string, bool) { return nil, false }
	}
	cache := map[string][]string{}
	return func(id string) ([]string, bool) {
		if sports, seen := cache[id]; seen {
			return sports, len(sports) > 0
		}
		var sports []string
		if t, err := s.multisportRepo.GetByID(ctx, id); err == nil && t != nil {
			for _, seg := range t.Segments {
				if !seg.IsTransition() {
					sports = append(sports, seg.Sport)
				}
			}
		}
		cache[id] = sports
		return sports, len(sports) > 0
	}
}
