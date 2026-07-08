FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /billing-agent .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /billing-agent /billing-agent
EXPOSE 8099
USER nonroot:nonroot
ENTRYPOINT ["/billing-agent"]
