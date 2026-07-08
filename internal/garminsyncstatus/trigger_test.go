package garminsyncstatus_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/garminsyncstatus"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

// fakeNotifier records the relogin side-effect calls.
type fakeNotifier struct {
	notified int
	cleared  int
}

func (f *fakeNotifier) NotifyReloginNeeded(context.Context) error { f.notified++; return nil }
func (f *fakeNotifier) ClearReloginLatch(context.Context) error   { f.cleared++; return nil }

// fakePresence reports a fixed token-presence answer.
type fakePresence struct{ has bool }

func (f fakePresence) HasToken(context.Context) (bool, error) { return f.has, nil }

func newTriggerSvc(t *testing.T, has bool) (*garminsyncstatus.Service, *fakeNotifier) {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := garminsyncstatus.NewService(garminsyncstatus.NewRepo(pool))
	n := &fakeNotifier{}
	svc.SetReloginNotifier(n)
	svc.SetGarminTokenPresence(fakePresence{has: has})
	return svc, n
}

func TestClose_ErrorWithAbsentToken_Notifies(t *testing.T) {
	svc, n := newTriggerSvc(t, false) // token absent
	ctx := context.Background()
	run, err := svc.Open(ctx, nil, nil)
	require.NoError(t, err)

	_, err = svc.Close(ctx, run.ID, "error", strptr("garmin auth expired"), nil)
	require.NoError(t, err)

	assert.Equal(t, 1, n.notified, "absent token on error-close must notify")
	assert.Equal(t, 0, n.cleared)
}

func TestClose_ErrorWithTokenPresent_DoesNotNotify(t *testing.T) {
	svc, n := newTriggerSvc(t, true) // token still present ⇒ transient
	ctx := context.Background()
	run, err := svc.Open(ctx, nil, nil)
	require.NoError(t, err)

	_, err = svc.Close(ctx, run.ID, "error", strptr("garmin 429 rate limited"), nil)
	require.NoError(t, err)

	assert.Equal(t, 0, n.notified, "token present on error-close must not notify")
}

func TestClose_Success_ClearsLatch(t *testing.T) {
	svc, n := newTriggerSvc(t, false)
	ctx := context.Background()
	run, err := svc.Open(ctx, nil, nil)
	require.NoError(t, err)

	_, err = svc.Close(ctx, run.ID, "success", nil, nil)
	require.NoError(t, err)

	assert.Equal(t, 1, n.cleared, "success-close must clear the latch")
	assert.Equal(t, 0, n.notified)
}

func strptr(s string) *string { return &s }
