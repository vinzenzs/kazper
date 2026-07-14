package publicfeed_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/publicfeed"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

// engineFor builds a root engine with the feed registered (no auth), backed by
// pool (may be nil for the secret-gate tests, which never reach the repo).
func engineFor(pool *pgxpool.Pool, secret string) *gin.Engine {
	svc := publicfeed.NewService(publicfeed.NewRepo(pool), time.UTC)
	r := gin.New()
	publicfeed.NewHandlers(svc, secret).Register(r)
	return r
}

func getFeed(r http.Handler, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/public/race-feed", nil)
	if key != "" {
		req.Header.Set("X-Feed-Key", key)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func dayStr(offset int) string {
	return time.Now().UTC().AddDate(0, 0, offset).Format("2006-01-02")
}

func seedRace(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name, raceDate string) string {
	t.Helper()
	id := uuid.NewString()
	_, err := pool.Exec(ctx, `INSERT INTO races (id,name,race_date) VALUES ($1::uuid,$2,$3::date)`, id, name, raceDate)
	require.NoError(t, err)
	return id
}

func seedMacro(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name, start, end string, raceID *string) {
	t.Helper()
	var rid any
	if raceID != nil {
		rid = *raceID
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO macrocycles (name,start_date,end_date,race_id) VALUES ($1,$2::date,$3::date,$4::uuid)`,
		name, start, end, rid)
	require.NoError(t, err)
}

// 4.1 (secret gate): a missing or wrong key is 401 — before any data access.
func TestFeed_MissingOrWrongKeyIs401(t *testing.T) {
	r := engineFor(nil, "s3cret")

	rec := getFeed(r, "") // no header
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "feed_unauthorized")

	rec = getFeed(r, "wrong")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "feed_unauthorized")
}

// 4.1 (disabled): an unset secret is 503, with or without a key.
func TestFeed_UnsetSecretIs503(t *testing.T) {
	r := engineFor(nil, "")
	for _, key := range []string{"", "anything"} {
		rec := getFeed(r, key)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Contains(t, rec.Body.String(), "feed_disabled")
	}
}

// 4.1 + 4.2: a valid key resolves the active macrocycle's A-race with a correct
// countdown and leaks no PII.
func TestFeed_ValidKeyResolvesCountdownNoPII(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewPool(t)
	rid := seedRace(t, ctx, pool, "Ironman Hamburg", dayStr(14))
	seedMacro(t, ctx, pool, "2026 A-season", dayStr(-10), dayStr(30), &rid)

	rec := getFeed(engineFor(pool, "s3cret"), "s3cret")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.Contains(t, body, `"name":"Ironman Hamburg"`)
	assert.Contains(t, body, `"race_date":"`+dayStr(14)+`"`)
	assert.Contains(t, body, `"days_remaining":14`)
	// Non-PII: none of the races/macrocycles extra columns leak.
	for _, forbidden := range []string{`"location"`, `"race_type"`, `"id"`, `"notes"`, `"methodology"`, "weight", "hrv"} {
		assert.NotContainsf(t, body, forbidden, "public feed must not expose %s", forbidden)
	}
}

// 4.2: on/after race day the countdown floors at zero.
func TestFeed_RaceDayFloorsAtZero(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewPool(t)
	rid := seedRace(t, ctx, pool, "Race Today", dayStr(-2)) // past, still within window
	seedMacro(t, ctx, pool, "season", dayStr(-10), dayStr(10), &rid)

	rec := getFeed(engineFor(pool, "k"), "k")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"days_remaining":0`)
}

// 4.2: no macrocycle containing today → graceful nulls.
func TestFeed_NoActiveMacrocycleDegradesToNull(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewPool(t)
	rid := seedRace(t, ctx, pool, "Future Race", dayStr(60))
	seedMacro(t, ctx, pool, "past season", dayStr(-60), dayStr(-10), &rid) // window is in the past

	rec := getFeed(engineFor(pool, "k"), "k")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"race":null,"days_remaining":null}`, rec.Body.String())
}

// 4.2: an active macrocycle with a NULL race_id → graceful nulls.
func TestFeed_NullRaceIDDegradesToNull(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewPool(t)
	seedMacro(t, ctx, pool, "unanchored season", dayStr(-5), dayStr(20), nil)

	rec := getFeed(engineFor(pool, "k"), "k")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"race":null,"days_remaining":null}`, rec.Body.String())
}

// 4.2: overlapping active macrocycles pick the one with the latest start_date.
func TestFeed_OverlappingPicksLatestStart(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewPool(t)
	older := seedRace(t, ctx, pool, "Older A-race", dayStr(40))
	newer := seedRace(t, ctx, pool, "Newer A-race", dayStr(20))
	seedMacro(t, ctx, pool, "older season", dayStr(-30), dayStr(60), &older)
	seedMacro(t, ctx, pool, "newer season", dayStr(-5), dayStr(60), &newer) // later start_date

	rec := getFeed(engineFor(pool, "k"), "k")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"name":"Newer A-race"`)
	assert.NotContains(t, body, "Older A-race")
	assert.Contains(t, body, `"days_remaining":20`)
}
