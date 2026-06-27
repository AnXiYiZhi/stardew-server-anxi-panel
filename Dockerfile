# ============================================================
# Stage 1: Build frontend (React/Vite)
# ============================================================
FROM node:22-alpine AS frontend-builder

WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install --frozen-lockfile 2>/dev/null || npm install
COPY frontend/ ./
RUN npm run build

# ============================================================
# Stage 2: Build backend (Go)
# ============================================================
FROM golang:1.25-alpine AS backend-builder

ARG VERSION=dev
ARG COMMIT=
ARG BUILD_DATE=

WORKDIR /src

# Cache dependency downloads.
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy frontend build output into the static embed directory.
COPY --from=frontend-builder /app/frontend/dist/ internal/static/frontend_dist/

# Copy Go source and build.
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w \
    -X 'github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config.buildVersion=${VERSION}' \
    -X 'github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config.buildCommit=${COMMIT}' \
    -X 'github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config.buildDate=${BUILD_DATE}'" \
    -o /app/panel ./cmd/panel

# ============================================================
# Stage 3: Runtime image
# ============================================================
FROM alpine:3.20

ARG VERSION=dev
ARG COMMIT=
ARG BUILD_DATE=

LABEL org.opencontainers.image.title="stardew-server-anxi-panel" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}"

RUN apk add --no-cache \
    docker-cli \
    docker-cli-compose \
    ca-certificates \
    tzdata

COPY --from=backend-builder /app/panel /app/panel

RUN mkdir -p /data

EXPOSE 8090

VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8090/health || exit 1

ENTRYPOINT ["/app/panel"]
