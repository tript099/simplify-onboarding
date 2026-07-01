# Simplify Onboarding portal (SPA) — built with Vite, served by nginx which also
# reverse-proxies the backend API sub-paths so the browser stays SAME-ORIGIN (the
# HttpOnly session cookie is dropped cross-origin under SameSite=Lax). This mirrors
# the Vite dev proxy (see vite.config.ts): /auth/** and /onb/* → backend, while the
# bare /auth route is the SPA sign-in PAGE served by nginx.
#
# The backend (simplify-onboarding-service) is deployed separately (docker compose);
# point ONBOARDING_API_URL at it. VITE_* are baked at BUILD time — set them as build
# args in Coolify.

# ── build the SPA ──────────────────────────────────────────────────
FROM node:22-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .

# Vite inlines VITE_* at build time. Defaults are production-safe; override in Coolify.
ARG VITE_USE_MOCK=false
ARG VITE_AUTH_BASE=/auth
ARG VITE_DOCFLOW_URL
ARG VITE_OTP_DEV_HINT=false
ARG VITE_DEFAULT_COUNTRY=ID
ENV VITE_USE_MOCK=$VITE_USE_MOCK \
    VITE_AUTH_BASE=$VITE_AUTH_BASE \
    VITE_DOCFLOW_URL=$VITE_DOCFLOW_URL \
    VITE_OTP_DEV_HINT=$VITE_OTP_DEV_HINT \
    VITE_DEFAULT_COUNTRY=$VITE_DEFAULT_COUNTRY
RUN npm run build

# ── serve with nginx (SPA + reverse proxy) ─────────────────────────
FROM nginx:1.27-alpine
# Where the onboarding-service (backend) is reachable FROM this container. Set in
# Coolify to the backend's URL, e.g. https://onboarding-api.simplifyai.id (or its
# internal service address if on the same network). Substituted into the nginx
# config at container start by the official image's envsubst templating.
ENV ONBOARDING_API_URL=http://onboarding:8090
# Only substitute our own vars — leave nginx's $uri/$host/etc. untouched.
ENV NGINX_ENVSUBST_FILTER=^ONBOARDING_

COPY nginx.conf.template /etc/nginx/templates/default.conf.template
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
