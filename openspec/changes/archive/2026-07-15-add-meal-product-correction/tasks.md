# Tasks

## 1. Backend

- [x] 1.1 `internal/meals` service `CorrectProduct` reusing the product-mode derivation; handler `POST /meals/{id}/correct-product` with Idempotency-Key + error matrix; verify against the actual meal column set at apply (preserved-vs-replaced per design D3)
- [x] 1.2 Integration tests: freeform→product derivation exactness, re-correction, preservation set (`logged_at`/note/workout link/`created_at`), summary reflection, all four errors, idempotent replay
- [x] 1.3 `task swag`

## 2. MCP

- [x] 2.1 `correct_meal_product` write tool; golden regen (additive) + registry/integration green

## 3. Verification

- [x] 3.1 `task vet` + meals suite green; live: correct a real old freeform entry via chat flow (write-confirm exercises the pause path)
