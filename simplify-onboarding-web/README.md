# simplify-onboarding-web

Branded onboarding portal for the Simplify suite — "One account, every product."
React + Vite + TypeScript + Tailwind, with a dark-first design system, Lenis smooth
scroll, framer-motion micro-interactions, and skeleton loading throughout.

See the plans in the parent folder:
[00_MASTER_PLAN.md](../00_MASTER_PLAN.md) ·
[Frontend plan](../SIMPLIFY_ONBOARDING_FRONTEND_PLAN.md) ·
[Backend plan](../SIMPLIFY_ONBOARDING_BACKEND_PLAN.md)

## Run

```bash
npm install
npm run dev          # http://localhost:3100
```

The app runs **standalone** against a mock API (`VITE_USE_MOCK=true`, the default), so
every screen — including skeleton states — works without the backend. In the OTP screen,
enter `123456` to verify.

```bash
npm run build        # type-check + production build
npm run typecheck    # types only
```

## Routes

| Path | Screen |
|---|---|
| `/` | Homepage — "What would you like to Simplify today?" problem-router |
| `/product/:key` | Product landing — value-first, motion split, CTA by motion |
| `/auth` | Create account / Sign in (split brand panel) |
| `/verify` | Email OTP verification |

Deep-link a product through auth with `?product=<key>` or `?client_id=<key>`
(e.g. `/auth?product=legal`) to theme the brand panel's product chip.

## Stack notes

- **Theme:** dark by default, persisted; toggle in the top-right. Tokens are HSL CSS
  variables in `src/index.css`; Tailwind maps them in `tailwind.config.ts`.
- **Smooth scroll:** Lenis (`src/hooks/useSmoothScroll.ts`), auto-disabled under
  `prefers-reduced-motion`.
- **Data:** `@tanstack/react-query` over `src/lib/api.ts`. Swap mock for the real
  `/auth` + `/onb` endpoints by setting `VITE_USE_MOCK=false`.
- **Forms:** `react-hook-form` + `zod` (`src/lib/validation.ts`).

When the backend lands, only `src/lib/api.ts` changes — the screens stay the same.

## Run against the live backend

The Vite dev server proxies `/auth` and `/onb` to `http://localhost:8090` (same-origin,
so the HttpOnly session cookie works). `.env.local` already sets `VITE_USE_MOCK=false`.

```bash
# terminal 1 — backend (see ../simplify-onboarding-service)
docker run -d --rm -p 6379:6379 redis:7-alpine
cd ../simplify-onboarding-service && go run ./cmd/onboarding   # :8090

# terminal 2 — frontend
npm run dev        # :3100, proxied to the backend
```

Create an account → on the OTP screen enter **123456** (dev bypass) → you're signed in.
Delete `.env.local` (or set `VITE_USE_MOCK=true`) to go back to the standalone mock.

> Verified end-to-end through the proxy: `register → otp verify → /auth/me` returns the
> signed-in user with `emailVerified: true`.
