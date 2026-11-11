#!/usr/bin/env bash
# Deploy to Google Cloud Run. Cloud Run instances are ephemeral, so the SQLite
# cache is rebuilt per instance; the container self-refreshes its store list on
# boot. For a durable cache, mount a volume (see README).
set -euo pipefail

PROJECT="${PROJECT:-$(gcloud config get-value project 2>/dev/null)}"
REGION="${REGION:-europe-west1}"
SERVICE="${SERVICE:-il-supermarket-compare}"

if [[ -z "${PROJECT}" ]]; then
  echo "Set PROJECT or run: gcloud config set project <id>" >&2
  exit 1
fi

gcloud builds submit --tag "gcr.io/${PROJECT}/${SERVICE}" .

gcloud run deploy "${SERVICE}" \
  --image "gcr.io/${PROJECT}/${SERVICE}" \
  --region "${REGION}" \
  --allow-unauthenticated \
  --port 8080 \
  --cpu 1 --memory 512Mi \
  --min-instances 1 \
  --set-env-vars "DB_PATH=/data/prices.db,STATIC_DIR=/app/web/dist,SELF_REFRESH=1"

echo "Deployed. URL: $(gcloud run services describe "${SERVICE}" --region "${REGION}" --format='value(status.url)')"
