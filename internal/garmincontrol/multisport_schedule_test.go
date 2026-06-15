package garmincontrol_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/garmincontrol"
	"github.com/vinzenzs/kazper/internal/multisport"
	wt "github.com/vinzenzs/kazper/internal/workouttemplates"
)

type fakeMultisport struct {
	tmpl *multisport.Template
}

func (f *fakeMultisport) GetByID(_ context.Context, _ string) (*multisport.Template, error) {
	if f.tmpl == nil {
		return nil, multisport.ErrNotFound
	}
	return f.tmpl, nil
}

func ptr(i int) *int { return &i }

func triathlonTemplate(id string) *multisport.Template {
	return &multisport.Template{
		ID:   id,
		Name: "Tri Race Sim",
		Segments: []multisport.Segment{
			{Sport: wt.SportSwim, Steps: []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentActive,
				Duration: &wt.Duration{Kind: wt.DurationDistance, Meters: ptr(750)},
				Target:   &wt.Target{Kind: wt.TargetSwimPace, LowSecPer100m: ptr(100), HighSecPer100m: ptr(100)}}}},
			{Sport: multisport.SportTransition, Duration: &wt.Duration{Kind: wt.DurationLapButton}},
			{Sport: wt.SportBike, Steps: []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentActive,
				Duration: &wt.Duration{Kind: wt.DurationTime, Seconds: ptr(1200)},
				Target:   &wt.Target{Kind: wt.TargetPowerZone, Low: ptr(3), High: ptr(3)}}}},
			{Sport: wt.SportRun, Steps: []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentActive,
				Duration: &wt.Duration{Kind: wt.DurationTime, Seconds: ptr(900)},
				Target:   &wt.Target{Kind: wt.TargetNone}}}},
		},
	}
}

func newMultisportEngine(t *testing.T, bridgeURL string, fm *fakeMultisport) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := garmincontrol.NewHandlers(bridgeURL)
	h.SetSchedulingDeps(newFakeWorkouts(), &fakeTemplates{}, &fakePlan{})
	h.SetMultisportRepo(fm)
	r := gin.New()
	r.Use(auth.Middleware(auth.Config{MobileToken: "mobile-token-aaaaaaaaaaaaaa", AgentToken: agentTok}))
	h.Register(r.Group("/"))
	return r
}

func TestScheduleMultisport_CompilesSegmentsAndSchedules(t *testing.T) {
	bridge := newBridgeStub(t)
	id := uuid.New().String()
	fm := &fakeMultisport{tmpl: triathlonTemplate(id)}
	r := newMultisportEngine(t, bridge.server.URL, fm)

	w := req(t, r, http.MethodPost, "/garmin/schedule/multisport",
		`{"multisport_template_id":"`+id+`","date":"2026-06-20"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// One multisport create + one schedule call.
	assert.Equal(t, 1, bridge.createCalls)
	assert.Equal(t, 1, bridge.schedCalls)
	// The create body is the MULTISPORT form (segments), not single-sport steps.
	assert.Contains(t, bridge.createBody, `"segments"`)
	assert.Contains(t, bridge.createBody, `"swim"`)
	assert.Contains(t, bridge.createBody, `"transition"`)
	assert.NotContains(t, bridge.createBody, `"steps":[{"type":"step","intent":"active"}],"sport"`)

	// Response carries the Garmin ids from the bridge.
	assert.Contains(t, w.Body.String(), `"garmin_workout_id":"gw-1"`)
	assert.Contains(t, w.Body.String(), `"garmin_schedule_id":"gs-1"`)
}

func TestScheduleMultisport_UnknownTemplateIs404(t *testing.T) {
	bridge := newBridgeStub(t)
	fm := &fakeMultisport{tmpl: nil}
	r := newMultisportEngine(t, bridge.server.URL, fm)

	w := req(t, r, http.MethodPost, "/garmin/schedule/multisport",
		`{"multisport_template_id":"`+uuid.New().String()+`","date":"2026-06-20"}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "multisport_template_not_found")
	assert.Equal(t, 0, bridge.createCalls)
}
