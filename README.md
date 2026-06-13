# Omegle.lol

Anonymous one-to-one text chat with tag-based matching. 

## Quick start

```sh
make setup   
make dev     
```

Open two browser windows to `http://localhost:5173`, type a shared tag (or none), click START.

## Layout

- `backend/` — Go service (WS handler, in-memory matcher, chat rooms, metrics).
- `frontend/` — SvelteKit SPA.
- `docs/superpowers/specs/` — design spec.
- `docs/superpowers/plans/` — implementation plan.

I made use of websockets. Few endpoints are available;

| Path           | What               |
| -------------- | ------------------ |
| `GET /healthz` | Liveness           |
| `GET /readyz`  | Readiness          |
| `GET /metrics` | Prometheus metrics |
| `GET /ws`      | WebSocket upgrade  |



## Linting

This project runs **zero-tolerance linting** in both Go and TypeScript/Svelte/CSS:

- **Go:** golangci-lint v1.62.2 with ~30 linters incl. `gosec`, `errcheck`, `staticcheck`, `gocritic`, `revive`.
- **TypeScript:** `tsconfig` strict mode + extras (`noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, etc.) + ESLint `strict-type-checked` + `stylistic-type-checked` + `eslint-plugin-svelte` + `eslint-plugin-import-x` + curated `eslint-plugin-unicorn` rules.
- **CSS / Svelte styles:** stylelint with `stylelint-config-standard` + `stylelint-config-html` + property-order rules.
- **Security:** `npm audit --omit=dev --audit-level=high` and `gosec` (via golangci-lint).
- **Formatting:** Prettier owns formatting; ESLint defers to it via `eslint-config-prettier`.

Lint failures block code at every stage:

| Stage                 | What runs                                                       | What blocks it    |
| --------------------- | --------------------------------------------------------------- | ----------------- |
| Pre-commit (lefthook) | golangci-lint, ESLint, stylelint, prettier on staged files      | The commit itself |
| Pre-push (lefthook)   | Full `make lint` + `go test -race` + `npm test --run`           | The push          |
| CI (GitHub Actions)   | All linters with `--max-warnings 0` + audit + lefthook-validate | The PR / merge    |


If you find a class of bug that the current linters don't catch, add the corresponding rule to:

- `backend/.golangci.yml` for Go.
- `frontend/eslint.config.js` for TypeScript / Svelte.
- `frontend/.stylelintrc.json` for CSS.

Then fix all surfaced findings before merging. No `//nolint` / `// eslint-disable-next-line` without a comment explaining why.

Ensure tests: 

- Backend unit/integration: `make test`
- Frontend unit: `make test`
- Playwright E2E (boots its own servers): `make e2e`

----

Production GCP deployment is scoped to a separate, follow-up plan. The codebase is local-first today; the only GCP-aware code lives under `deploy/` (added later).
