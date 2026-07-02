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
ENV ONBOARDING_API_URL=http://jy4j98o2xwb9wed6roi1u6gv.16.78.196.32.sslip.io
# Only substitute our own vars — leave nginx's $uri/$host/etc. untouched.
ENV NGINX_ENVSUBST_FILTER=^ONBOARDING_

# nginx config is inlined here (no separate file). It serves the SPA and proxies the
# API paths to the backend on the same origin so the HttpOnly session cookie works.
# ${ONBOARDING_API_URL} is substituted at container start by nginx's envsubst templating.
COPY <<'NGINXCONF' /etc/nginx/templates/default.conf.template
server {
    listen 80;
    root /usr/share/nginx/html;

    # Always behind Coolify's HTTPS edge → tell the backend it was https (else it
    # force-redirects to https and loops). Don't override Host; nginx uses the
    # backend's host so the edge routes to the backend, not back to the frontend.
    proxy_set_header X-Forwarded-Proto https;
    proxy_ssl_server_name on;

    # API → backend.
    location /auth/      { proxy_pass ${ONBOARDING_API_URL}; }
    location /onb/       { proxy_pass ${ONBOARDING_API_URL}; }
    location = /validate { proxy_pass ${ONBOARDING_API_URL}; }

    # Bare /auth is the SPA sign-in page; everything else is client-side routing.
    location = /auth { try_files /index.html =404; }
    location /       { try_files $uri /index.html; }
}
NGINXCONF
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
