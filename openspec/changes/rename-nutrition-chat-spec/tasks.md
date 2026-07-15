# Tasks

## 1. Rename mechanics (executed at archive per design D1)

- [x] 1.1 `git mv openspec/specs/nutrition-chat/spec.md openspec/specs/coach-chat/spec.md` (history-preserving); retitle the `# ... Specification` header to coach-chat
- [x] 1.2 Replace `Purpose: TBD` with the real paragraph (grounded in the moved requirements; design D2 sketch)
- [x] 1.3 Fold away the seeded naming requirement (scaffolding, not behavior); confirm `openspec validate --strict` passes on the moved spec
- [x] 1.4 Sweep current-state references to `nutrition-chat` (README/docs capability mentions only — historical notes stay verbatim)

## 2. Verification

- [x] 2.1 `openspec doctor` / `validate` clean; no code, no swag, no test impact (`git grep -l nutrition-chat` shows only historical docs)
