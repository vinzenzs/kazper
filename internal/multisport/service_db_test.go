package multisport_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/multisport"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	wt "github.com/vinzenzs/kazper/internal/workouttemplates"
)

func i(v int) *int { return &v }

func timeSeg(sport string, sec int) multisport.Segment {
	return multisport.Segment{Sport: sport, Steps: []wt.Step{
		{Type: wt.NodeStep, Intent: wt.IntentActive, Duration: &wt.Duration{Kind: wt.DurationTime, Seconds: i(sec)}, Target: &wt.Target{Kind: wt.TargetNone}},
	}}
}

func newSvc(t *testing.T) *multisport.Service {
	t.Helper()
	return multisport.NewService(multisport.NewRepo(storetest.NewPool(t)))
}

func TestService_DerivesEstimatedDuration_FullyTimeBounded(t *testing.T) {
	svc := newSvc(t)
	ctx := context.Background()
	tmpl := &multisport.Template{Name: "brick", Segments: []multisport.Segment{
		timeSeg(wt.SportBike, 1200),
		{Sport: multisport.SportTransition, Duration: &wt.Duration{Kind: wt.DurationTime, Seconds: i(60)}},
		timeSeg(wt.SportRun, 900),
	}}
	created, err := svc.Create(ctx, tmpl)
	require.NoError(t, err)
	require.NotNil(t, created.EstimatedDurationSec)
	assert.Equal(t, 1200+60+900, *created.EstimatedDurationSec, "Create stamps the derived total")

	// Round-trips on read from the DB (JSONB segments → derived on Get).
	got, err := svc.Get(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, got.EstimatedDurationSec)
	assert.Equal(t, 1200+60+900, *got.EstimatedDurationSec)
}

func TestService_OmitsEstimatedDuration_WhenNotTimeBounded(t *testing.T) {
	svc := newSvc(t)
	ctx := context.Background()
	// Swim segment uses a distance step → total is not determinable.
	tmpl := &multisport.Template{Name: "open-ended", Segments: []multisport.Segment{
		{Sport: wt.SportSwim, Steps: []wt.Step{
			{Type: wt.NodeStep, Intent: wt.IntentActive, Duration: &wt.Duration{Kind: wt.DurationDistance, Meters: i(750)}, Target: &wt.Target{Kind: wt.TargetSwimPace, LowSecPer100m: i(100), HighSecPer100m: i(110)}},
		}},
		timeSeg(wt.SportBike, 1200),
	}}
	created, err := svc.Create(ctx, tmpl)
	require.NoError(t, err)
	assert.Nil(t, created.EstimatedDurationSec, "non-time-bounded segment → omitted")

	got, err := svc.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, got.EstimatedDurationSec)
}
