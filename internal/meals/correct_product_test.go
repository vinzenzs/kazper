package meals_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/meals"
)

// seedFreeform logs a freeform meal with a note and returns the decoded entry.
func seedFreeform(t *testing.T, f *fixture) meals.MealEntry {
	t.Helper()
	body := `{
        "name":"guessed pasta",
        "nutriments_per_100g":{"kcal":200,"protein_g":7,"carbs_g":40,"fat_g":2},
        "quantity_g":300,
        "logged_at":"2026-06-06T19:00:00Z",
        "note":"eyeballed macros"
    }`
	rec := doRequest(t, f.r, http.MethodPost, "/meals/freeform", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var m meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	return m
}

func TestCorrectProduct_FreeformToProductPreservesIdentity(t *testing.T) {
	f := setupMeals(t)
	before := seedFreeform(t, f)
	pid := makeProduct(t, f.productsRepo) // Nutella, 539 kcal/100g

	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":150}`, pid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals/"+before.ID.String()+"/correct-product", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var after meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &after))

	// Identity + preserved fields.
	assert.Equal(t, before.ID, after.ID)
	assert.Equal(t, before.LoggedAt, after.LoggedAt, "logged_at preserved")
	require.NotNil(t, after.Note)
	assert.Equal(t, "eyeballed macros", *after.Note, "note preserved")
	// Nutrients now derive from the product.
	require.NotNil(t, after.ProductID)
	assert.Equal(t, pid, *after.ProductID)
	assert.InDelta(t, 150.0, after.QuantityG, 0.001)
	assert.Equal(t, "Nutella", after.EffectiveName)
	require.NotNil(t, after.EffectiveNutrimentsPer100g.KcalPer100g)
	assert.InDelta(t, 539.0, *after.EffectiveNutrimentsPer100g.KcalPer100g, 0.001)
}

func TestCorrectProduct_ReCorrectionFixesWrongProduct(t *testing.T) {
	f := setupMeals(t)
	before := seedFreeform(t, f)
	pid1 := makeProduct(t, f.productsRepo)
	pid2 := makeProduct(t, f.productsRepo)

	first := fmt.Sprintf(`{"product_id":%q,"quantity_g":100}`, pid1)
	require.Equal(t, http.StatusOK, doRequest(t, f.r, http.MethodPost, "/meals/"+before.ID.String()+"/correct-product", first).Code)
	second := fmt.Sprintf(`{"product_id":%q,"quantity_g":250}`, pid2)
	rec := doRequest(t, f.r, http.MethodPost, "/meals/"+before.ID.String()+"/correct-product", second)
	require.Equal(t, http.StatusOK, rec.Code)
	var after meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &after))
	require.NotNil(t, after.ProductID)
	assert.Equal(t, pid2, *after.ProductID)
	assert.InDelta(t, 250.0, after.QuantityG, 0.001)
}

func TestCorrectProduct_SummaryReflectsCorrection(t *testing.T) {
	f := setupMeals(t)
	before := seedFreeform(t, f)
	pid := makeProduct(t, f.productsRepo)

	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":200}`, pid)
	require.Equal(t, http.StatusOK, doRequest(t, f.r, http.MethodPost, "/meals/"+before.ID.String()+"/correct-product", body).Code)

	// A re-GET reflects the corrected product (summaries read the same stored row).
	rec := doRequest(t, f.r, http.MethodGet, "/meals/"+before.ID.String(), "")
	require.Equal(t, http.StatusOK, rec.Code)
	var m meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	assert.Equal(t, "Nutella", m.EffectiveName)
}

func TestCorrectProduct_Errors(t *testing.T) {
	f := setupMeals(t)
	meal := seedFreeform(t, f)
	pid := makeProduct(t, f.productsRepo)

	cases := []struct {
		name, path, body, code string
		status                 int
	}{
		{"unknown meal", "/meals/" + uuid.NewString() + "/correct-product", fmt.Sprintf(`{"product_id":%q,"quantity_g":100}`, pid), "not_found", http.StatusNotFound},
		{"unknown product", "/meals/" + meal.ID.String() + "/correct-product", fmt.Sprintf(`{"product_id":%q,"quantity_g":100}`, uuid.NewString()), "product_not_found", http.StatusNotFound},
		{"zero quantity", "/meals/" + meal.ID.String() + "/correct-product", fmt.Sprintf(`{"product_id":%q,"quantity_g":0}`, pid), "quantity_invalid", http.StatusBadRequest},
		{"missing product_id", "/meals/" + meal.ID.String() + "/correct-product", `{"quantity_g":100}`, "product_id_required", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(t, f.r, http.MethodPost, tc.path, tc.body)
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.code, body["error"])
		})
	}
}
