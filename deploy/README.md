# deploy/

Templates and notes for shipping omegle.lol to Google Cloud.

## Files

- `cloudrun-service.yaml` — Cloud Run service definition. Pins
  `min == max == 1` because the matcher is in-memory; multi-instance would
  silently break matching.

## What's *not* here (yet)

These get added when the deployment plan lands:

- Terraform (Cloud Run service, Artifact Registry repo, GCS bucket for the
  static SPA, Cloud Load Balancer + managed cert, optional Cloud Armor for
  per-IP rate limiting at the edge instead of in-process)
- `cloudbuild.yaml` (CI builds, push image, deploy service, sync GCS bucket)
- DNS records (Cloud DNS or external registrar)

## One-shot manual deploy (until Terraform lands)

```sh
PROJECT=your-project
REGION=us-central1
REPO=omegle
TAG=$(git rev-parse --short HEAD)

# 1. Build & push backend image.
gcloud builds submit backend \
  --tag "$REGION-docker.pkg.dev/$PROJECT/$REPO/omegle-backend:$TAG"

# 2. Substitute the image into the service template and apply.
sed "s|REGION-docker.pkg.dev/PROJECT/REGISTRY/omegle-backend:TAG|$REGION-docker.pkg.dev/$PROJECT/$REPO/omegle-backend:$TAG|" \
    deploy/cloudrun-service.yaml > /tmp/service.yaml
gcloud run services replace /tmp/service.yaml --region "$REGION"

# 3. Build & upload the SPA.
cd frontend && npm ci && npm run build
gsutil -m rsync -d -r build/ gs://omegle-lol-static/
```

## Notes for ops

- The first request after a cold start should hit `/readyz` and succeed; the
  startup probe gives it 10 s.
- WebSocket connections can live up to `timeoutSeconds` (1 h). Past that
  Cloud Run will drop them; clients auto-reconnect with a new sessionId and
  the store appends a "Connection lost." system message.
- `LOG_LEVEL=info` is the production default — drop to `warn` if log volume
  becomes a cost issue, or `debug` temporarily for an incident.
- `TRUSTED_PROXY_DEPTH=1` assumes Cloud Run direct. If you front the service
  with a Cloud Load Balancer, change to `2`.
- The in-process per-IP rate limiter (`connectRate` in handler.go) grows
  unbounded. Mitigation deferred to Phase 2 — for now, abuse should be
  filtered at the edge (Cloud Armor rate-limit rules).
