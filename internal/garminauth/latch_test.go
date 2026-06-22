package garminauth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/garminauth"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

// fakeClearer records latch-clear calls and can fail.
type fakeClearer struct {
	calls int
	fail  bool
}

func (f *fakeClearer) ClearReloginLatch(context.Context) error {
	f.calls++
	if f.fail {
		return errors.New("boom")
	}
	return nil
}

func TestStore_ClearsReloginLatch(t *testing.T) {
	pool := storetest.NewPool(t)
	svc, err := garminauth.NewService(garminauth.NewRepo(pool), encKey())
	require.NoError(t, err)
	clearer := &fakeClearer{}
	svc.SetReloginLatchClearer(clearer)

	require.NoError(t, svc.Store(context.Background(), []byte("fresh-token-blob")))
	assert.Equal(t, 1, clearer.calls, "storing a token must clear the relogin latch")
}

func TestStore_LatchClearFailureDoesNotFailStore(t *testing.T) {
	pool := storetest.NewPool(t)
	svc, err := garminauth.NewService(garminauth.NewRepo(pool), encKey())
	require.NoError(t, err)
	svc.SetReloginLatchClearer(&fakeClearer{fail: true})

	// The store must still succeed and the blob must be retrievable.
	require.NoError(t, svc.Store(context.Background(), []byte("blob")))
	got, err := svc.Get(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "blob", string(got))
}
