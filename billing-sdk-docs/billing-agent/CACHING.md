# Billing Agent — caching & consistency plan

Goal: products read entitlement/quota without hammering core, **and never act on a
dirty read** (a `remaining` that doesn't reflect committed consumes → wrongly allow =
overspend, or wrongly deny = lost revenue).

## 1. The key idea: split data by mutation rate
"Entitlement" is two different things with opposite needs:

| Class | Examples | Changes | Cacheability |
|---|---|---|---|
| **Structure** (slow) | plan, subscription state, feature list, **quota limits**, reset windows, credit branding | rarely — on subscribe / change-plan / addon / cancel | cache hard; invalidate on events |
| **Counter** (fast) | quota **used/remaining** | every `consume` | authoritative + atomic; never a stale binding read |

Caching the *structure* is what eliminates most core calls. The *counter* is the only
consistency-critical value, and is handled separately so caching never causes a dirty read.

## 2. Two-layer cache

- **L1 — in-proc (agent process)**: hot structure snapshot per org. Answers `can-use`
  (limits), `features`, `subscription`, `credit` with zero core calls on a hit.
- **L2 — shared Redis (only when multiple agent replicas / HA)**: shared structure +
  the authoritative usage counters, so all replicas agree. L1 sits in front of L2 with
  pub/sub invalidation. Single-replica deploys can skip L2 (in-proc only).

## 3. Cache schema (keys / values / TTL)

```
ent:{app}:{org}                         → snapshot JSON:
    { version, plan_code, state, period{start,end},
      features:[{code, enabled, limit, reset_at}],
      credit{balance, currency, name}, fetched_at }
    soft_ttl = 60s   (freshness backstop)
    hard_ttl = 10m   (stale-on-error window only)

quota:{app}:{org}:{feature}:{period_id} → integer counter (remaining)   [authoritative]
    owned by core's metering (Redis softgate already exists); atomic DECR-if-≥units

idem:{app}:{idempotency_key}            → consume dedup (already in core)  TTL 24h
neg:{app}:{org}                         → negative cache for unknown/invalid org  TTL 10s
```

- `version`/`fetched_at` make snapshots monotonic: a newer invalidation supersedes; the
  agent ignores out-of-order updates.
- `period_id` keys the counter to the billing window so a period rollover is a new key
  (no manual reset, no stale carry-over).

## 4. Invalidation — event-driven, NOT polling (this is what cuts core calls)
Core already emits `subscription.created/updated/canceled`, `addon.purchased`, and has
`POST /sdk/entitlements/{org}/invalidate`. The agent runs a **webhook receiver**
(HMAC-verified — the SDK's `webhook` pkg) and on any of those events **busts `ent:{org}`**
(and optionally re-warms it). Result: a structure snapshot is fetched **once per org, then
again only when it actually changes** — not per request, not on a timer.

- TTL is only a *backstop* for missed events (soft 60s), not the primary refresh path.
- Add **singleflight** per `ent:{org}` so a cache miss under load triggers one core fetch,
  not a thundering herd; **jitter** TTLs to avoid synchronized expiry.

## 5. No-dirty-read guarantees (the counter)
Five rules make caching safe for quota:

1. **Single authoritative counter, atomic decrement.** Every `consume` is an atomic
   compare-and-decrement (`DECR if remaining ≥ units`, Redis Lua or DB `FOR UPDATE`).
   Concurrent consumes across agents/replicas can't overspend or lose updates.
2. **Binding vs advisory.** `authorize`/`report` (→ core atomic consume) is the **binding**
   decision and is always authoritative. `can-use` returns the cached snapshot's remaining
   as an **advisory hint** (clearly labelled). Strict/expensive features gate on `authorize`,
   so a slightly-stale `can-use` can never cause an overspend.
3. **Read-your-writes.** On its own consume the agent updates/invalidates its L1 view, so
   the next read reflects what it just spent.
4. **Versioned snapshots.** Invalidations carry the new version; the agent only replaces a
   snapshot with a strictly-newer one (ignores duplicate/out-of-order webhooks).
5. **Fail-closed on counter uncertainty.** If core is unreachable and there's no fresh
   counter, structure may be served stale (limits don't drift) but the **counter** is
   treated as unknown → strict features deny (`fail_closed`); only non-critical features
   honour `fail_open`. Stale-on-error never *extends* spendable quota.

## 6. Recommended rollout (simplest correct first)

- **Phase A — structure cache + event invalidation (now).** L1 in-proc snapshot,
  webhook-busted, soft TTL. `authorize` stays read-through to core's atomic consume
  (already Redis-fast). This already removes ~all read traffic for plan/limits/sub/credit
  with **zero dirty-read risk** (binding = core atomic). Covers the 80% case.
- **Phase B — shared Redis (when ≥2 agent replicas).** Move L2 to Redis so replicas share
  structure + invalidation (pub/sub) and the authoritative counter.
- **Phase C — counter read-through Redis (only if consume QPS is high).** Agent reads
  `remaining` straight from the shared Redis counter (no core call even for reads) and
  decrements via atomic Lua; core hydrates the counter from Postgres once per window and
  async-flushes `usage_events` for the ledger. Still no dirty read (atomic single counter).
- **Phase D — lease/reservation (extreme throughput only).** Agent leases a block of N
  units per (org,feature), serves local atomic decrements, returns the remainder on TTL/
  shutdown. Fewest core calls under load; most complex (lease reconciliation) — defer unless
  measured need.

## 7. What NOT to do
- Don't cache `remaining` and treat it as binding — that's the dirty read. Cache *limits*;
  bind on the *atomic counter*.
- Don't poll core on a timer for freshness — invalidate on events; TTL is only a backstop.
- Don't share one global TTL — separate structure (60s/10m) from counter (authoritative).

## Summary
Cache the **slow structure** aggressively (event-invalidated, in-proc → Redis at scale);
keep the **fast counter** authoritative and atomic; make the **binding** decision via
`authorize`. That gives near-zero core reads for the common path with a hard no-dirty-read
guarantee, and a clear graduation path (Redis → counter-read-through → leases) as load grows.

---

## Consumption write-back — IMPLEMENTED + e2e (2026-06-23)
`report` is now **batched**, `authorize` stays **synchronous** (binding gate).

- `internal/batch.Batcher`: buffers report events, flushes to `/sdk/metering/consume/batch`
  every `AGENT_FLUSH_SIZE` events or `AGENT_FLUSH_INTERVAL` (default 50 / 2s), **on shutdown**,
  and **re-queues on flush error** (at-least-once; idempotency keys dedup at core).
- `report` → 202 `{queued:true}` (buffered); `authorize` → synchronous atomic consume.
- On flush, the flushed orgs' read cache is invalidated so the next `can-use` is accurate.
- Graceful SIGTERM flush runs **synchronously in main after the server stops** (not in the
  signal goroutine) so the process can't exit before the final flush — no lost usage on deploy.

**e2e (live)**: 2 batched reports (5 units) → core unchanged (buffered) → after 4s flush core
`16→11` + 2 usage_events; `authorize 1` → core `11→10` immediately (sync); buffered report +
`docker stop` (SIGTERM) → final flush applied core `10→9`. All green.

**Config**: `AGENT_FLUSH_INTERVAL`, `AGENT_FLUSH_SIZE` (0/0 disables → report falls back to sync).
