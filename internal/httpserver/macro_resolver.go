package httpserver

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/macrocycle"
	"github.com/vinzenzs/kazper/internal/pmc"
)

// macroResolver adapts *macrocycle.Repo to pmc.MacroResolver so the
// target-trajectory endpoint can resolve the subject macrocycle without pmc
// importing the macrocycle package. Active resolution uses the latest-start_date
// covering-today rule (the public-race-feed convention), not the coach-context
// updated_at tie-break.
type macroResolver struct {
	repo *macrocycle.Repo
}

func (m macroResolver) Resolve(ctx context.Context, id *string, today time.Time) (*pmc.Macro, error) {
	if id != nil {
		uid, err := uuid.Parse(*id)
		if err != nil {
			// A malformed id is treated as "no such macrocycle" — the endpoint's
			// subject simply doesn't resolve.
			return nil, pmc.ErrMacrocycleNotFound
		}
		mc, err := m.repo.GetByID(ctx, uid)
		if err != nil {
			if errors.Is(err, macrocycle.ErrMacrocycleNotFound) {
				return nil, pmc.ErrMacrocycleNotFound
			}
			return nil, err
		}
		return toPMCMacro(mc), nil
	}

	// Active macrocycle: the first (latest start_date — List is start_date DESC)
	// whose [start,end] contains today.
	list, err := m.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	td := today.Format("2006-01-02")
	for _, c := range list {
		if c.StartDate.Format("2006-01-02") <= td && td <= c.EndDate.Format("2006-01-02") {
			// List omits member phases; fetch the full record for its targets.
			mc, err := m.repo.GetByID(ctx, c.ID)
			if err != nil {
				return nil, err
			}
			return toPMCMacro(mc), nil
		}
	}
	return nil, pmc.ErrMacrocycleNotFound
}

func toPMCMacro(mc *macrocycle.Macrocycle) *pmc.Macro {
	phases := make([]pmc.MacroPhase, 0, len(mc.Phases))
	for _, p := range mc.Phases {
		phases = append(phases, pmc.MacroPhase{
			StartDate:       p.StartDate,
			EndDate:         p.EndDate,
			TargetWeeklyTSS: p.TargetWeeklyTSS,
		})
	}
	return &pmc.Macro{
		ID:        mc.ID.String(),
		Name:      mc.Name,
		StartDate: mc.StartDate,
		EndDate:   mc.EndDate,
		Phases:    phases,
	}
}
