# syntax=docker/dockerfile:1

# Stage 1: build the SPA on the build host (no qemu: the target is arm64).
FROM --platform=$BUILDPLATFORM node:24.22.3-alpine AS web
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# Production SPA must talk to the Go API, not the preview client.
ARG VITE_API_MODE=server
RUN VITE_API_MODE=$VITE_API_MODE npm run build

# Stage 2: cross-compile the single static arm64 binary with the SPA embedded.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=web /app/dist ./internal/web/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
      -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
      -o /meteran ./cmd/meteran

# Stage 3: runtime.
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata curl \
 && adduser -D -u 10001 meteran \
 && mkdir -p /data \
 && chown meteran:meteran /data
COPY --from=build /meteran /meteran
USER meteran
ENV METERAN_DB=/data/meteran.db \
    TZ=Asia/Jakarta
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD curl -sf http://localhost:8080/api/health >/dev/null || exit 1
ENTRYPOINT ["/meteran"]