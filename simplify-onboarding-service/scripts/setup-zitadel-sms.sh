#!/usr/bin/env bash
# Configure Zitadel to send SMS OTPs via our webhook (instead of Twilio), then forward
# them to any SMS gateway. Registers an HTTP SMS provider pointing at our webhook and
# activates it. Zitadel returns a signingKey — set it as SMS_WEBHOOK_SIGNING_KEY on the
# onboarding service so we can verify the ZITADEL-Signature.
#
# Requires an INSTANCE-admin token (IAM_OWNER). A normal org service PAT is usually NOT
# enough for instance SMS settings — generate a token for an IAM_OWNER user, or run these
# calls from the Zitadel console's API playground.
#
# Usage:
#   ZITADEL_DOMAIN=https://auth.simplifyai.id \
#   ZITADEL_ADMIN_TOKEN=<iam_owner_token> \
#   WEBHOOK_URL=https://onboarding.simplifyai.id/sms/zitadel \
#   ./scripts/setup-zitadel-sms.sh
set -euo pipefail

: "${ZITADEL_DOMAIN:?set ZITADEL_DOMAIN, e.g. https://auth.simplifyai.id}"
: "${ZITADEL_ADMIN_TOKEN:?set ZITADEL_ADMIN_TOKEN (IAM_OWNER)}"
: "${WEBHOOK_URL:?set WEBHOOK_URL, e.g. https://onboarding.simplifyai.id/sms/zitadel}"

api() { curl -sS -H "Authorization: Bearer ${ZITADEL_ADMIN_TOKEN}" -H "Content-Type: application/json" "$@"; }

echo "== 1. Add HTTP SMS provider → ${WEBHOOK_URL}"
ADD=$(api -X POST "${ZITADEL_DOMAIN}/admin/v1/sms/http" \
  -d "{\"endpoint\":\"${WEBHOOK_URL}\",\"description\":\"Onboarding SMS webhook (forwards to our gateway)\"}")
echo "$ADD" | python3 -m json.tool 2>/dev/null || echo "$ADD"

ID=$(echo "$ADD" | python3 -c "import sys,json;print(json.load(sys.stdin).get('id',''))" 2>/dev/null || true)
KEY=$(echo "$ADD" | python3 -c "import sys,json;print(json.load(sys.stdin).get('signingKey',''))" 2>/dev/null || true)

if [ -z "$ID" ]; then echo "!! could not read provider id — check the response/token scope above"; exit 1; fi

echo "== 2. Activate provider ${ID}"
api -X POST "${ZITADEL_DOMAIN}/admin/v1/sms/${ID}/_activate" -d '{}' >/dev/null && echo "activated."

echo
echo "==================== DONE ===================="
echo "SMS provider id : ${ID}"
echo "signingKey      : ${KEY}"
echo
echo "Set on the onboarding service and redeploy:"
echo "  SMS_WEBHOOK_SIGNING_KEY=${KEY}"
echo "=============================================="
