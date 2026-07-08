# SMS OTP login via a non-Twilio provider (Zitadel HTTP SMS webhook)

Zitadel sends SMS OTPs. Instead of Twilio, we point Zitadel at a **webhook we host**; the
webhook forwards the OTP to **any SMS gateway** (configured with env vars, no code changes).

```
User enters phone → StartLoginOTP → Zitadel needs to send an SMS
   → Zitadel POSTs the OTP to  https://onboarding.simplifyai.id/sms/zitadel   (HTTP SMS provider)
   → our webhook verifies ZITADEL-Signature, then forwards {phone,text} to YOUR gateway
   → user gets the code → VerifyLoginOTP
```

## Pieces (already built)
- **Webhook:** `POST /sms/zitadel` (onboarding service) — verifies the `ZITADEL-Signature`
  (HMAC-SHA256), parses `contextInfo.recipientPhoneNumber` + `templateData.text`, forwards.
- **Exposure:** the frontend nginx proxies `/sms/` to the backend, so the public HTTPS URL
  is `https://onboarding.simplifyai.id/sms/zitadel`.
- **Forwarder:** provider-agnostic — you describe your gateway with `SMS_PROVIDER_*` env.
- **Setup script:** `scripts/setup-zitadel-sms.sh` registers + activates the provider in Zitadel.

## Step 1 — Choose your SMS transport (env on the onboarding service)

### Option A — AWS SNS (recommended here) `SMS_PROVIDER=sns`
SNS is used **only as the transport** — Zitadel generates & verifies the OTP; we call
`sns:Publish` to deliver the one SMS. (So it's normal per-message SNS SMS cost, **not**
AWS-managed OTP/Cognito/Pinpoint.)
```env
SMS_PROVIDER=sns
SMS_SNS_REGION=ap-southeast-1       # a region that supports SMS
SMS_SNS_TYPE=Transactional          # OTP → transactional
SMS_SNS_SENDER_ID=Simplify          # optional; only where the country allows alphanumeric senders
AWS_ACCESS_KEY_ID=<key>             # IAM user/role with sns:Publish
AWS_SECRET_ACCESS_KEY=<secret>
```
**AWS side (DevOps):** IAM policy with `sns:Publish`; request **production SMS access** (out
of the SNS sandbox) so you can send to any number; set an SMS **spend limit**; for
Indonesia/India destinations register a **Sender ID / origination** if required.
Minimal IAM policy:
```json
{ "Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"sns:Publish","Resource":"*"}] }
```
> Leave `SMS_SNS_REGION` empty to run **log-only** first (webhook logs the OTP, no send).

### Option B — any HTTP SMS gateway `SMS_PROVIDER=http`
Fill these for whatever provider you use (examples below), then (re)deploy:
```env
SMS_PROVIDER_URL=https://<your-gateway>/send
SMS_PROVIDER_METHOD=POST
SMS_PROVIDER_CONTENT_TYPE=application/json
SMS_PROVIDER_AUTH_HEADER=Authorization: Bearer <your-gateway-token>
SMS_PROVIDER_BODY_TEMPLATE={"to":"{{phone}}","message":"{{text}}"}
```
`{{phone}}` and `{{text}}` are substituted (and escaped for JSON/form). Examples:

| Gateway style | CONTENT_TYPE | AUTH_HEADER | BODY_TEMPLATE |
|---|---|---|---|
| JSON API | `application/json` | `Authorization: Bearer TOKEN` | `{"to":"{{phone}}","message":"{{text}}"}` |
| MSG91-style | `application/json` | `authkey: KEY` | `{"mobiles":"{{phone}}","message":"{{text}}","sender":"SMPLFY"}` |
| Form POST | `application/x-www-form-urlencoded` | `X-API-Key: KEY` | `to={{phone}}&body={{text}}` |

> Leave `SMS_PROVIDER_URL` **empty** to run in **log-only** mode first — the webhook logs the
> OTP instead of sending, so you can validate the Zitadel round-trip before wiring the gateway.

## Step 2 — Register the webhook in Zitadel
Needs an **IAM_OWNER** token (instance admin) — a normal org PAT usually isn't enough.
```bash
ZITADEL_DOMAIN=https://auth.simplifyai.id \
ZITADEL_ADMIN_TOKEN=<iam_owner_token> \
WEBHOOK_URL=https://onboarding.simplifyai.id/sms/zitadel \
./scripts/setup-zitadel-sms.sh
```
It prints a **`signingKey`**. Set it on the onboarding service and redeploy:
```env
SMS_WEBHOOK_SIGNING_KEY=<signingKey>
SMS_WEBHOOK_VERIFY=true
```
(The script also **activates** the provider, so Zitadel starts using it immediately.)

Manual equivalent, if you prefer:
```bash
# add
curl -X POST "$ZITADEL_DOMAIN/admin/v1/sms/http" -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"endpoint":"https://onboarding.simplifyai.id/sms/zitadel","description":"Onboarding SMS webhook"}'
# → note the returned "id" and "signingKey", then activate:
curl -X POST "$ZITADEL_DOMAIN/admin/v1/sms/<id>/_activate" -H "Authorization: Bearer $TOKEN" -d '{}'
```

## Step 3 — Test
1. First-run (no signing key yet, no gateway): `SMS_WEBHOOK_VERIFY=false`, `SMS_PROVIDER_URL=` empty.
   Trigger an SMS OTP login with a **verified phone** → check the onboarding logs for
   `zitadel sms webhook: SMS_PROVIDER_URL not set — logging OTP` with the code. That proves
   Zitadel → webhook works.
2. Add `SMS_WEBHOOK_SIGNING_KEY` + `SMS_WEBHOOK_VERIFY=true` → confirm it still forwards (signature OK).
3. Add `SMS_PROVIDER_*` → real SMS delivered.

## Notes / gotchas
- **Phone must be verified** in Zitadel for OTP-SMS login (see the phone-verification flow).
- The webhook returns **200** on success; a non-200 tells Zitadel the send failed.
- **Signature scheme:** `ZITADEL-Signature: t=<unix>,v1=<hex>`, HMAC-SHA256 over `"<t>.<rawBody>"`
  with the `signingKey`. If verification unexpectedly fails on your Zitadel version, set
  `SMS_WEBHOOK_VERIFY=false` temporarily and share a sample header so we can adjust.
- Provider add/activate is **instance-level** — must be an IAM_OWNER token (or done from the
  Zitadel console API playground).
