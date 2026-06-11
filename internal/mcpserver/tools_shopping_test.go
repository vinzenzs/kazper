package mcpserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddShoppingItems_PostsArrayWithKey(t *testing.T) {
	c, records := newRacePrepRecorder(t, 201, `{"items":[]}`)
	qty := "3"
	handleAddShoppingItems(context.Background(), c, AddShoppingItemsArgs{
		Items: []ShoppingItemArg{
			{Name: "Zwiebeln", QuantityText: &qty},
			{Name: "Hackfleisch"},
		},
	})
	rec := (*records)[0]
	assert.Equal(t, "POST", rec.method)
	assert.Equal(t, "/shopping/items", rec.path)
	assert.NotEmpty(t, rec.idemKey)
	assert.Contains(t, rec.body, `"name":"Zwiebeln"`)
	assert.Contains(t, rec.body, `"items":[`)
	assert.NotContains(t, rec.body, "idempotency_key")
}

func TestUpdateShoppingItem_PatchesChecked(t *testing.T) {
	c, records := newRacePrepRecorder(t, 200, `{}`)
	checked := true
	handleUpdateShoppingItem(context.Background(), c, UpdateShoppingItemArgs{ID: "i1", Checked: &checked})
	rec := (*records)[0]
	assert.Equal(t, "PATCH", rec.method)
	assert.Equal(t, "/shopping/items/i1", rec.path)
	assert.Contains(t, rec.body, `"checked":true`)
	assert.NotContains(t, rec.body, "name", "omitted fields not sent")
}
