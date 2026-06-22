package push_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/push"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

// fakeSender records the tokens it was asked to send to and can fail a specific
// token with ErrTokenUnregistered to exercise pruning.
type fakeSender struct {
	mu          sync.Mutex
	sent        []string
	unregister  map[string]bool // tokens FCM "rejects" as unregistered
	failGeneric map[string]bool // tokens that error without pruning
}

func (s *fakeSender) Send(_ context.Context, token string, _ push.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, token)
	if s.unregister[token] {
		return push.ErrTokenUnregistered
	}
	if s.failGeneric[token] {
		return assert.AnError
	}
	return nil
}

func register(t *testing.T, svc *push.Service, tokens ...string) {
	t.Helper()
	for _, tok := range tokens {
		_, err := svc.RegisterToken(context.Background(), tok, "android")
		require.NoError(t, err)
	}
}

func TestNotifyReloginNeeded_LatchesAfterFirstSend(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := push.NewRepo(pool)
	sender := &fakeSender{}
	svc := push.NewService(repo, sender, nil)
	register(t, svc, "tokA", "tokB")
	ctx := context.Background()

	// First notification fans out to every token and sets the latch.
	require.NoError(t, svc.NotifyReloginNeeded(ctx))
	assert.ElementsMatch(t, []string{"tokA", "tokB"}, sender.sent)
	latch, err := repo.Latch(ctx)
	require.NoError(t, err)
	assert.True(t, latch.Notified)
	assert.NotNil(t, latch.NotifiedAt)

	// A second call while latched is a no-op — no further sends.
	require.NoError(t, svc.NotifyReloginNeeded(ctx))
	assert.Len(t, sender.sent, 2, "must not re-send while latched")
}

func TestNotifyReloginNeeded_ClearResetsLatch(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := push.NewRepo(pool)
	sender := &fakeSender{}
	svc := push.NewService(repo, sender, nil)
	register(t, svc, "tokA")
	ctx := context.Background()

	require.NoError(t, svc.NotifyReloginNeeded(ctx))
	require.NoError(t, svc.ClearReloginLatch(ctx))

	latch, err := repo.Latch(ctx)
	require.NoError(t, err)
	assert.False(t, latch.Notified)
	assert.Nil(t, latch.NotifiedAt)

	// After a clear, the next outage notifies again.
	require.NoError(t, svc.NotifyReloginNeeded(ctx))
	assert.Equal(t, []string{"tokA", "tokA"}, sender.sent)
}

func TestNotifyReloginNeeded_DisabledIsNoop(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := push.NewRepo(pool)
	svc := push.NewService(repo, nil, nil) // nil sender = push disabled
	register(t, svc, "tokA")
	ctx := context.Background()

	require.NoError(t, svc.NotifyReloginNeeded(ctx))
	latch, err := repo.Latch(ctx)
	require.NoError(t, err)
	assert.False(t, latch.Notified, "disabled push must not latch")
}

func TestNotifyReloginNeeded_NoTokensDoesNotLatch(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := push.NewRepo(pool)
	svc := push.NewService(repo, &fakeSender{}, nil)
	ctx := context.Background()

	require.NoError(t, svc.NotifyReloginNeeded(ctx))
	latch, err := repo.Latch(ctx)
	require.NoError(t, err)
	assert.False(t, latch.Notified, "with no registered device the latch stays open")
}

func TestNotifyReloginNeeded_PrunesUnregisteredToken(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := push.NewRepo(pool)
	sender := &fakeSender{unregister: map[string]bool{"dead": true}}
	svc := push.NewService(repo, sender, nil)
	register(t, svc, "live", "dead")
	ctx := context.Background()

	require.NoError(t, svc.NotifyReloginNeeded(ctx))
	// Both attempted...
	assert.ElementsMatch(t, []string{"live", "dead"}, sender.sent)
	// ...but the unregistered one is pruned, the live one kept.
	remaining, err := repo.ListTokens(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"live"}, remaining)
}

func TestNotifyReloginNeeded_GenericFailureDoesNotPrune(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := push.NewRepo(pool)
	sender := &fakeSender{failGeneric: map[string]bool{"flaky": true}}
	svc := push.NewService(repo, sender, nil)
	register(t, svc, "flaky", "ok")
	ctx := context.Background()

	require.NoError(t, svc.NotifyReloginNeeded(ctx))
	remaining, err := repo.ListTokens(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"flaky", "ok"}, remaining, "a transient send error must not prune the token")
}
