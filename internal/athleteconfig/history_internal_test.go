package athleteconfig

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func ip(i int) *int         { return &i }
func fp(f float64) *float64 { return &f }

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// --- physiologyEqual (task 3.1) ------------------------------------------

func TestPhysiologyEqual(t *testing.T) {
	base := &AthleteConfig{FtpWatts: ip(250), ThresholdPaceSecPerKm: fp(270)}
	same := &AthleteConfig{FtpWatts: ip(250), ThresholdPaceSecPerKm: fp(270)}
	assert.True(t, physiologyEqual(base, same))

	// Different int field.
	assert.False(t, physiologyEqual(base, &AthleteConfig{FtpWatts: ip(255), ThresholdPaceSecPerKm: fp(270)}))
	// Different float field.
	assert.False(t, physiologyEqual(base, &AthleteConfig{FtpWatts: ip(250), ThresholdPaceSecPerKm: fp(271)}))
	// nil vs value differs; two nils equal.
	assert.False(t, physiologyEqual(base, &AthleteConfig{ThresholdPaceSecPerKm: fp(270)}))
	assert.True(t, physiologyEqual(&AthleteConfig{}, &AthleteConfig{}))
	// Timestamps excluded from equality.
	withTS := &AthleteConfig{FtpWatts: ip(250), ThresholdPaceSecPerKm: fp(270)}
	withTS.CreatedAt = time.Now()
	assert.True(t, physiologyEqual(base, withTS))
}

// --- repo: list ascending + range + as-of + latest-before ----------------

func TestRepo_HistoryQueries(t *testing.T) {
	ctx := context.Background()
	repo := NewRepo(storetest.NewPool(t))

	require.NoError(t, repo.UpsertSnapshot(ctx, date(1970, 1, 1), &AthleteConfig{FtpWatts: ip(240)}))
	require.NoError(t, repo.UpsertSnapshot(ctx, date(2026, 5, 1), &AthleteConfig{FtpWatts: ip(255)}))
	require.NoError(t, repo.UpsertSnapshot(ctx, date(2026, 6, 1), &AthleteConfig{FtpWatts: ip(270)}))

	// Ascending, all.
	all, err := repo.ListHistory(ctx, nil, nil)
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, "1970-01-01", all[0].EffectiveFrom)
	assert.Equal(t, 240, *all[0].FtpWatts)
	assert.Equal(t, "2026-06-01", all[2].EffectiveFrom)

	// Inclusive range filter.
	from, to := date(2026, 5, 1), date(2026, 6, 1)
	ranged, err := repo.ListHistory(ctx, &from, &to)
	require.NoError(t, err)
	require.Len(t, ranged, 2)
	assert.Equal(t, "2026-05-01", ranged[0].EffectiveFrom)

	// As-of: before first → nil.
	pre, err := repo.AsOf(ctx, date(1969, 1, 1))
	require.NoError(t, err)
	assert.Nil(t, pre)
	// As-of between: resolves the earlier snapshot.
	mid, err := repo.AsOf(ctx, date(2026, 5, 15))
	require.NoError(t, err)
	require.NotNil(t, mid)
	assert.Equal(t, 255, *mid.FtpWatts)
	// As-of on a snapshot date resolves it.
	onDate, err := repo.AsOf(ctx, date(2026, 6, 1))
	require.NoError(t, err)
	assert.Equal(t, 270, *onDate.FtpWatts)

	// LatestBefore is strictly-before.
	lb, err := repo.LatestBefore(ctx, date(2026, 6, 1))
	require.NoError(t, err)
	require.NotNil(t, lb)
	assert.Equal(t, 255, *lb.FtpWatts)
}

func TestRepo_AsOfEmptyHistory(t *testing.T) {
	got, err := NewRepo(storetest.NewPool(t)).AsOf(context.Background(), date(2026, 6, 1))
	require.NoError(t, err)
	assert.Nil(t, got)
}

// --- service: snapshot-on-change, same-day, revert-collapse --------------

func TestService_PutHistoryMaintenance(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewPool(t)
	svc := NewService(NewRepo(pool), pool)
	repo := NewRepo(pool)

	// First PUT (fresh DB, no seed) → one snapshot at today.
	_, err := svc.Put(ctx, &AthleteConfig{FtpWatts: ip(240)})
	require.NoError(t, err)
	h, _ := repo.ListHistory(ctx, nil, nil)
	require.Len(t, h, 1)
	assert.Equal(t, 240, *h[0].FtpWatts)

	// No-op re-PUT (identical) appends nothing.
	_, err = svc.Put(ctx, &AthleteConfig{FtpWatts: ip(240)})
	require.NoError(t, err)
	h, _ = repo.ListHistory(ctx, nil, nil)
	require.Len(t, h, 1)

	// Same-day change replaces today's row (still one row, new value).
	_, err = svc.Put(ctx, &AthleteConfig{FtpWatts: ip(255)})
	require.NoError(t, err)
	h, _ = repo.ListHistory(ctx, nil, nil)
	require.Len(t, h, 1)
	assert.Equal(t, 255, *h[0].FtpWatts)
}

func TestService_SameDayRevertCollapses(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewPool(t)
	svc := NewService(NewRepo(pool), pool)
	repo := NewRepo(pool)

	// Seed a "yesterday" snapshot at ftp 240 and align the singleton to it.
	yesterday := truncateToDate(time.Now().UTC()).AddDate(0, 0, -1)
	require.NoError(t, repo.UpsertSnapshot(ctx, yesterday, &AthleteConfig{FtpWatts: ip(240)}))
	require.NoError(t, repo.Upsert(ctx, &AthleteConfig{FtpWatts: ip(240)}))

	// Change to 255 today → today's snapshot appended (2 rows).
	_, err := svc.Put(ctx, &AthleteConfig{FtpWatts: ip(255)})
	require.NoError(t, err)
	h, _ := repo.ListHistory(ctx, nil, nil)
	require.Len(t, h, 2)

	// Revert to 240 today → equals yesterday's snapshot → today's row deleted.
	_, err = svc.Put(ctx, &AthleteConfig{FtpWatts: ip(240)})
	require.NoError(t, err)
	h, _ = repo.ListHistory(ctx, nil, nil)
	require.Len(t, h, 1)
	assert.Equal(t, "yesterday only", oneRowLabel(h, yesterday))
}

func oneRowLabel(h []*ThresholdSnapshot, yesterday time.Time) string {
	if len(h) == 1 && h[0].EffectiveFrom == yesterday.Format(dateLayout) {
		return "yesterday only"
	}
	return "unexpected"
}

// ConfigAsOf is exposed but unwired; smoke it end-to-end through the service.
func TestService_ConfigAsOf(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewPool(t)
	svc := NewService(NewRepo(pool), pool)
	require.NoError(t, NewRepo(pool).UpsertSnapshot(ctx, date(2026, 5, 1), &AthleteConfig{FtpWatts: ip(255)}))

	got, err := svc.ConfigAsOf(ctx, date(2026, 5, 10))
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 255, *got.FtpWatts)

	empty, err := svc.ConfigAsOf(ctx, date(2020, 1, 1))
	require.NoError(t, err)
	assert.Nil(t, empty)
}
