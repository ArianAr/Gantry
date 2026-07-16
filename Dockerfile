# syntax=docker/dockerfile:1

# Multi-arch builds: compile on the builder host (BUILDPLATFORM), not under QEMU.
# Go cross-compiles to TARGETARCH natively — arm64 images no longer emulate the
# entire Go toolchain (the previous bottleneck).

# --- Phase 1: frontend (once, on host arch) ---
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci || npm install
COPY frontend/ ./
RUN npm run build

# --- Phase 2: Go binary (native toolchain, cross-compile to target) ---
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
WORKDIR /src
ENV GOTOOLCHAIN=auto
ARG TARGETOS=linux
ARG TARGETARCH
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/dist ./frontend/dist

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
  -ldflags "-s -w \
    -X github.com/ArianAr/Gantry/internal/version.Version=${VERSION} \
    -X github.com/ArianAr/Gantry/internal/version.Commit=${COMMIT} \
    -X github.com/ArianAr/Gantry/internal/version.BuildDate=${BUILD_DATE}" \
  -o /out/gantry .

# Data directory owned by distroless nonroot (uid/gid 65532)
FROM alpine:3.21 AS data
RUN mkdir -p /data && chown -R 65532:65532 /data

# --- Phase 3: minimal runtime (per-target arch) ---
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=builder /out/gantry /gantry
COPY --from=data --chown=65532:65532 /data /data
USER nonroot:nonroot
EXPOSE 8080
ENV GANTRY_ADDR=:8080
ENV GANTRY_DB=/data/gantry.db
VOLUME ["/data"]
ENTRYPOINT ["/gantry"]
CMD ["-addr", ":8080", "-db", "/data/gantry.db"]

LABEL org.opencontainers.image.title="Gantry" \
      org.opencontainers.image.description="Multi-provider S3 sync engine" \
      org.opencontainers.image.source="https://github.com/ArianAr/Gantry" \
      org.opencontainers.image.licenses="GPL-3.0"
