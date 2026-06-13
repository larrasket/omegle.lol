# Build context: the repo root, so we can see both backend/ and frontend/.
# Multi-stage: build the SvelteKit SPA, embed it into the Go binary, ship
# a distroless image with just the static binary inside.

# ── frontend builder ────────────────────────────────────────────────────
FROM node:22-bookworm-slim AS frontend
WORKDIR /app
# Copy only package.json (not the lockfile). The lockfile encodes
# platform-specific optional deps from whoever generated it, and npm has
# a long-standing bug (https://github.com/npm/cli/issues/4828) where
# replaying that lockfile on a different arch silently skips the local
# platform's native binding. Regenerating in-container always picks the
# right set for linux/amd64.
COPY frontend/package.json ./
RUN npm install --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ── backend builder ─────────────────────────────────────────────────────
FROM golang:1.26-bookworm AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Replace the placeholder webroot with the real SPA before `go build` runs
# so embed.FS picks up the production assets.
RUN rm -rf internal/webroot/files
COPY --from=frontend /app/build internal/webroot/files
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/server ./cmd/server

# ── runtime ─────────────────────────────────────────────────────────────
# distroless/static-debian12:nonroot is ~2 MB, has no shell, runs as
# uid 65532. Static binary inside → nothing else needed.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

COPY --from=backend /out/server /server

LABEL org.opencontainers.image.title="omegle.lol"
LABEL org.opencontainers.image.description="Anonymous one-to-one text chat. Go + SvelteKit, single static binary."
LABEL org.opencontainers.image.source="https://github.com/omegle-lol/omegle"

# Cloud Run passes the listen port via PORT; default to 8080 for local runs.
EXPOSE 8080

# Probe targets:
#   GET /healthz  → liveness
#   GET /readyz   → readiness
#   GET /metrics  → Prometheus
#   GET /ws       → WebSocket upgrade
#   GET /*        → embedded SvelteKit SPA (SPA fallback to index.html)

ENTRYPOINT ["/server"]
