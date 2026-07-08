# Account API — Integration Guide

Audience: **developers building an ecosystem account-management UI.** This HMAC-
authenticated, server-to-server endpoint returns a user's **organisations** and,
per org, the **products they've subscribed to** with subscription details.

---

## 1. Get a key
In the SimplifyBilling **admin portal → Integration → Account API**, click
**Generate key**. You receive:
- **Key ID** — public identifier, e.g. `acct_ab12cd34ef56`.
- **Secret** — shown **once**; store it securely. Used to sign requests.

Optionally set **Allowed origins** — required only if you call from a **browser**
(CORS); pure server-to-server calls send no `Origin` and skip that check.

## 2. Endpoint
```
POST  https://<your-billing-host>/api/v1/public/account/orgs
Content-Type: application/json
```

## 3. Authentication (HMAC-SHA256)
Send three headers:

| Header | Value |
|---|---|
| `X-Account-Key` | your Key ID (`acct_…`) |
| `X-Account-Timestamp` | current Unix time in **seconds** |
| `X-Account-Signature` | `sha256=` + `hex( HMAC_SHA256(secret, "<timestamp>.<raw body>") )` |

Rules:
- **Signing input** = the timestamp, a literal `.`, then the **raw request body bytes** — sign the exact bytes you send (don't re-serialize).
- **Key** = the secret string (`acsk_…`) as raw UTF-8 bytes.
- **Replay window**: the timestamp must be within **±5 minutes** of server time.
- The `sha256=` prefix is optional (a bare hex is also accepted).

## 4. Request body
```json
{ "email": "user@example.com" }
```

## 5. Response `200`
```json
{
  "email": "user@example.com",
  "orgs": [
    {
      "id": "…", "name": "Acme Inc", "slug": "acme-inc", "type": "team",
      "subscriptions": [
        {
          "product_slug": "simplify-docflow", "product_name": "SimplifyDocflow",
          "plan_code": "pro", "plan_name": "Pro", "billing_period": "monthly",
          "state": "active",
          "price_cents": 4900, "currency": "USD",
          "auto_renew": true, "cancel_at_period_end": false,
          "current_period_start": "2026-07-01T00:00:00Z",
          "current_period_end": "2026-08-01T00:00:00Z"
        }
      ]
    }
  ]
}
```
An org with no subscriptions returns `"subscriptions": []`. A user in N orgs returns N entries.

## 6. Errors
| Status | `code` | Meaning |
|---|---|---|
| 401 | `account.unauthorized` | missing headers / unknown key / stale timestamp / signature mismatch |
| 403 | `account.origin_forbidden` | `Origin` not in the key's allowed list (browser callers) |
| 400 | `account.email_required` | body missing `email` |

## 7. Example (bash)
```bash
HOST="https://<your-billing-host>"
KEY_ID="acct_ab12cd34ef56"
SECRET="acsk_…"
TS=$(date +%s)
BODY='{"email":"user@example.com"}'
SIG=$(printf '%s.%s' "$TS" "$BODY" | openssl dgst -sha256 -hmac "$SECRET" -hex | sed 's/^.*= //')

curl -s "$HOST/api/v1/public/account/orgs" \
  -H "Content-Type: application/json" \
  -H "X-Account-Key: $KEY_ID" \
  -H "X-Account-Timestamp: $TS" \
  -H "X-Account-Signature: sha256=$SIG" \
  -d "$BODY"
```

## 8. Example (Node)
```js
import crypto from 'node:crypto'
const ts = Math.floor(Date.now() / 1000).toString()
const body = JSON.stringify({ email: 'user@example.com' })
const sig = 'sha256=' + crypto.createHmac('sha256', SECRET).update(`${ts}.${body}`).digest('hex')
await fetch(`${HOST}/api/v1/public/account/orgs`, {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'X-Account-Key': KEY_ID,
    'X-Account-Timestamp': ts,
    'X-Account-Signature': sig,
  },
  body,
})
```

## Notes
- Rotate keys by generating a new one and revoking the old in the admin UI.
- The secret is stored **encrypted** at rest and never returned again after creation.
- Every request is tagged with an `X-Request-ID` for support correlation.
