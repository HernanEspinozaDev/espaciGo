#!/usr/bin/env bash
set -euo pipefail

# Deploy script for EspaciGo Frontend to Cloud Run
REGION="southamerica-west1"
PROJECT_ID="espacigo-448200"
SERVICE_NAME="espacigo-frontend"

echo "Deploying ${SERVICE_NAME} to Google Cloud Run (${REGION}) in project ${PROJECT_ID}..."
gcloud run deploy "${SERVICE_NAME}" \
  --source frontend \
  --project "${PROJECT_ID}" \
  --region "${REGION}" \
  --set-env-vars NEXT_PUBLIC_API_URL=https://espacigo-api-559139855589.southamerica-west1.run.app,NODE_ENV=production \
  --allow-unauthenticated \
  --quiet

echo "Deployment to Cloud Run completed successfully!"
