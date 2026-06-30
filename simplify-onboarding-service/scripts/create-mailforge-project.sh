#!/usr/bin/env bash
# Provision a DEDICATED MailForge project + send key for the onboarding service.
#
# Why: emails for the onboarding platform should live in their own MailForge project
# (clean analytics, quotas, isolation) — not piggyback on DocFlow's key.
#
# Requires an ADMIN-scoped MailForge key (the `admin` super-scope). The DocFlow project
# key is NOT admin — ask infra for the bootstrap-admin key, or mint one with admin scope.
#
# Usage:
#   MAILFORGE_URL=http://host.docker.internal:8099 \
#   MAILFORGE_ADMIN_KEY=mf_proj_<admin> \
#   ./create-mailforge-project.sh
#
# On success it prints the new `mf_proj_…` send key — put it in the onboarding service
# env as ONBOARDING_MAILFORGE_API_KEY.
set -euo pipefail

: "${MAILFORGE_URL:?set MAILFORGE_URL (e.g. http://host.docker.internal:8099)}"
: "${MAILFORGE_ADMIN_KEY:?set MAILFORGE_ADMIN_KEY (an admin-scoped mf_proj_ key)}"

FROM_NAME="${MAILFORGE_FROM_NAME:-Simplify}"
FROM_EMAIL="${MAILFORGE_FROM_EMAIL:-no-reply@simplifyaipro.com}"
REPLY_TO="${MAILFORGE_REPLY_TO:-sales@simplifyaipro.com}"

echo "→ Creating project 'Simplify Onboarding' on ${MAILFORGE_URL} …"
PROJECT_JSON=$(curl -sS -X POST "${MAILFORGE_URL}/v1/projects/" \
  -H "Authorization: Bearer ${MAILFORGE_ADMIN_KEY}" \
  -H "Content-Type: application/json" \
  -d "{
        \"slug\": \"simplify-onboarding\",
        \"name\": \"Simplify Onboarding\",
        \"from_name\": \"${FROM_NAME}\",
        \"from_email\": \"${FROM_EMAIL}\",
        \"reply_to\": \"${REPLY_TO}\"
      }")

PROJECT_ID=$(printf '%s' "$PROJECT_JSON" | sed -nE 's/.*"id"\s*:\s*"([^"]+)".*/\1/p' | head -1)
if [ -z "$PROJECT_ID" ]; then
  echo "✗ Could not create/parse project. Response:"; echo "$PROJECT_JSON"; exit 1
fi
echo "  project id: ${PROJECT_ID}"

echo "→ Issuing a send key (scope: email:send) …"
KEY_JSON=$(curl -sS -X POST "${MAILFORGE_URL}/v1/projects/${PROJECT_ID}/api-keys" \
  -H "Authorization: Bearer ${MAILFORGE_ADMIN_KEY}" \
  -H "Content-Type: application/json" \
  -d '{ "name": "onboarding-send", "scopes": ["email:send"] }')

API_KEY=$(printf '%s' "$KEY_JSON" | sed -nE 's/.*"(key|api_key|secret)"\s*:\s*"(mf_[^"]+)".*/\2/p' | head -1)
if [ -z "$API_KEY" ]; then
  echo "✗ Could not parse the issued key. Response:"; echo "$KEY_JSON"; exit 1
fi

echo
echo "✓ Done. Add this to the onboarding env (.env):"
echo
echo "    ONBOARDING_MAILFORGE_API_KEY=${API_KEY}"
echo
echo "(Then: docker compose -f docker-compose.sso.yml --env-file ../../.env up -d)"
