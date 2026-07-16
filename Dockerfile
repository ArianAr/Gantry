# syntax=docker/dockerfile:1

# --- Phase 1: frontend ---
FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci || npm install
COPY frontend/ ./
RUN npm run build

# --- Phase 2: Go binary ---
FROM golang:1.24-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/dist ./frontend/dist

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
  -ldflags "-s -w \
    -X github.com/ArianAr/Gantry/internal/version.Version=${VERSION} \
    -X github.com/ArianAr/Gantry/internal/version.Commit=${COMMIT} \
    -X github.com/ArianAr/Gantry/internal/version.BuildDate=${BUILD_DATE}" \
  -o /out/gantry .

# --- Phase 3: minimal runtime ---
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=builder /out/gantry /gantry
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
