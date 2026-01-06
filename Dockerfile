# YouFlac Docker Build
# Multi-stage build for minimal image size

# ============================================
# Stage 1: Build Frontend
# ============================================
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy package files first for better caching
COPY frontend/package.json ./
RUN npm install --silent

# Copy frontend source and build
COPY frontend/ ./
RUN npm run build

# ============================================
# Stage 2: Build Backend
# ============================================
FROM golang:1.23-bookworm AS backend-builder

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY backend/ ./backend/
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o youflac-server ./cmd/server

# ============================================
# Stage 3: Runtime Image
# ============================================
FROM debian:bookworm-slim

LABEL org.opencontainers.image.title="YouFlac"
LABEL org.opencontainers.image.description="YouTube Video + FLAC Audio = Perfect MKV"
LABEL org.opencontainers.image.source="https://github.com/kushie/youflac"

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    python3 \
    python3-mutagen \
    python3-websockets \
    python3-brotli \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean

# Install yt-dlp (latest release)
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp \
    && chmod +x /usr/local/bin/yt-dlp

# Create non-root user
RUN useradd -m -u 1000 youflac

# Create directories
RUN mkdir -p /config /downloads /app \
    && chown -R youflac:youflac /config /downloads /app

WORKDIR /app

# Copy built artifacts
COPY --from=backend-builder /app/youflac-server ./
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

# Set ownership
RUN chown -R youflac:youflac /app

# Switch to non-root user
USER youflac

# Environment variables with defaults
ENV PORT=8080 \
    OUTPUT_DIR=/downloads \
    CONFIG_DIR=/config \
    VIDEO_QUALITY=best \
    CONCURRENT_DOWNLOADS=2 \
    NAMING_TEMPLATE=jellyfin \
    GENERATE_NFO=true \
    EMBED_COVER_ART=true \
    LYRICS_ENABLED=false \
    LYRICS_EMBED_MODE=lrc \
    THEME=dark \
    ACCENT_COLOR=pink

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:${PORT}/api/health || exit 1

# Volumes for persistent data
VOLUME ["/config", "/downloads"]

# Start server
CMD ["./youflac-server"]
