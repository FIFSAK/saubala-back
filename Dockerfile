# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache modules first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
# Static binaries (no cgo) for both the server and the seeder.
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
      go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/saubala-back \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
      go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

# ---- runtime stage ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget \
 && adduser -D -u 10001 app
WORKDIR /app

COPY --from=build /out/server /app/server
COPY --from=build /out/seed   /app/seed

USER app
EXPOSE 8080

ENV APP_MODE=prod \
    HTTP_PORT=:8080 \
    APP_PATH=/api/v1

HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/server"]
