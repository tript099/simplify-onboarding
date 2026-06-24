# ── build ──────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/onboarding ./cmd/onboarding

# ── run ────────────────────────────────────────────────────────────
FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl && adduser -D -u 10001 app
USER app
COPY --from=build /out/onboarding /usr/local/bin/onboarding
EXPOSE 8090
# Curl-based healthcheck (avoids the silent-hang class of bug; see project memory).
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
  CMD curl -fsS http://localhost:8090/health || exit 1
ENTRYPOINT ["onboarding"]
