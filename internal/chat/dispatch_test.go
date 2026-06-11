package chat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingHandler captures the loopback sub-requests the dispatcher makes.
type recordedReq struct {
	method, path, auth, idemKey, body string
}

type recordingHandler struct {
	reqs   []recordedReq
	status int
	resp   string
}

func (h *recordingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	h.reqs = append(h.reqs, recordedReq{
		method:  r.Method,
		path:    r.URL.Path,
		auth:    r.Header.Get("Authorization"),
		idemKey: r.Header.Get("Idempotency-Key"),
		body:    string(body),
	})
	st := h.status
	if st == 0 {
		st = http.StatusOK
	}
	w.WriteHeader(st)
	_, _ = io.WriteString(w, h.resp)
}

func TestDispatcher_WriteToolForwardsBearerAndIdempotencyKey(t *testing.T) {
	rec := &recordingHandler{resp: `{"ok":true}`, status: http.StatusCreated}
	d := newDispatcher(rec)

	res := d.execute(context.Background(), "add_shopping_items",
		json.RawMessage(`{"items":[{"name":"onion"}]}`), "test-token")

	require.True(t, res.ok)
	require.Len(t, rec.reqs, 1)
	got := rec.reqs[0]
	assert.Equal(t, "POST", got.method)
	assert.Equal(t, "/shopping/items", got.path)
	assert.Equal(t, "Bearer test-token", got.auth)
	assert.NotEmpty(t, got.idemKey, "write tool must carry an Idempotency-Key")
	assert.Contains(t, got.body, "onion")

	// Same call again → same derived key (the retry-replay guarantee).
	d.execute(context.Background(), "add_shopping_items",
		json.RawMessage(`{"items":[{"name":"onion"}]}`), "test-token")
	require.Len(t, rec.reqs, 2)
	assert.Equal(t, rec.reqs[0].idemKey, rec.reqs[1].idemKey)
}

func TestDispatcher_ReadToolHasNoIdempotencyKey(t *testing.T) {
	rec := &recordingHandler{resp: `{}`}
	d := newDispatcher(rec)

	d.execute(context.Background(), "get_daily_context",
		json.RawMessage(`{"date":"2026-06-12"}`), "tok")
	require.Len(t, rec.reqs, 1)
	assert.Empty(t, rec.reqs[0].idemKey)
	assert.Equal(t, "GET", rec.reqs[0].method)
	assert.Equal(t, "/context/daily", rec.reqs[0].path)
}

func TestDispatcher_Non2xxIsNotOK(t *testing.T) {
	rec := &recordingHandler{resp: `{"error":"product_not_found"}`, status: http.StatusNotFound}
	d := newDispatcher(rec)
	res := d.execute(context.Background(), "get_product", json.RawMessage(`{"product_id":"x"}`), "tok")
	assert.False(t, res.ok)
	assert.Equal(t, http.StatusNotFound, res.status)
	assert.Contains(t, string(res.body), "product_not_found")
}

func TestDispatcher_UnknownToolErrors(t *testing.T) {
	d := newDispatcher(&recordingHandler{})
	res := d.execute(context.Background(), "delete_everything", json.RawMessage(`{}`), "tok")
	assert.Error(t, res.err)
}

func TestDispatcher_BadInputNeverReachesREST(t *testing.T) {
	rec := &recordingHandler{}
	d := newDispatcher(rec)
	// mark_planned_meal_eaten requires plan_id; omitting it must fail in build.
	res := d.execute(context.Background(), "mark_planned_meal_eaten", json.RawMessage(`{}`), "tok")
	assert.Error(t, res.err)
	assert.Empty(t, rec.reqs, "no REST call should be made for un-buildable input")
}
