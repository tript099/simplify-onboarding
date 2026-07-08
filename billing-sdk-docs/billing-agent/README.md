# billing-agent — the Cerbos of quota

A product-side sidecar. Your BFF asks two simple questions; the agent hides all
SimplifyBilling logic (entitlement resolution, plan + add-ons + token pools, overage,
wallet/Simplify-Credit funding, caching, batching, reporting).

## Local API (localhost:8099, no auth — localhost only)
- `POST /v1/can-use`   `{org_id, feature}`           → `{allow, remaining, limit, reset_at, reason, plan, state}`
- `POST /v1/report`    `{org_id, feature, units}`    → `{ok, units_consumed, funding_source}`   (decrement after use)
- `POST /v1/authorize` `{org_id, feature, units}`    → `{allowed, remaining, funding_source, payment_options}` (atomic)
- `GET  /v1/features?org_id=`     full entitlement snapshot
- `GET  /v1/subscription?org_id=` plan, state, period
- `GET  /v1/credit?org_id=`       Simplify Credit balance
- `POST /v1/provision/org` · `/v1/provision/trial`
- `GET  /healthz` · `/readyz`

## BFF usage (mirrors Cerbos CheckResource)
```
allow := agent.POST("/v1/can-use", {org, "ai_interview"})   // before serving
if !allow.allow { return 402, allow.payment_options }
... run service ...
agent.POST("/v1/report", {org, "ai_interview", units})       // after
# or fuse both with /v1/authorize for strict/expensive features
```

## Config — see `.env.example`. API key from admin → Integration (bk_…).
