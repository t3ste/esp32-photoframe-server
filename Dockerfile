ARG BUILD_FROM=node:20-alpine

# Build Stage for Go
FROM golang:alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git build-base

# Copy Go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy Source
COPY server/ ./server/
RUN CGO_ENABLED=1 go build -o photoframe-server ./server

# Build Stage for Frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# Final Stage
FROM $BUILD_FROM

WORKDIR /app

# Install runtime dependencies
# Node.js and NPM are needed for the processor script
# Canvas dependencies: cairo, pango, jpeg, giflib, librsvg
RUN apk add --no-cache \
    nodejs \
    npm \
    cairo-dev \
    pango-dev \
    jpeg-dev \
    giflib-dev \
    librsvg-dev \
    tzdata \
    ffmpeg \
    build-base \
    python3 \
    font-inter

# Create directories
RUN mkdir -p /app/bin /app/static /app/data

# Copy Binary
COPY --from=builder /app/photoframe-server /app/photoframe-server

# Copy Frontend Build
COPY --from=frontend-builder /app/dist /app/static

# Copy Processor Scripts
# Download process-cli files from GitHub
ADD https://raw.githubusercontent.com/aitjcize/esp32-photoframe/main/process-cli/image-processor.js /app/bin/image-processor.js
ADD https://raw.githubusercontent.com/aitjcize/esp32-photoframe/main/process-cli/cli.js /app/bin/cli.js
ADD https://raw.githubusercontent.com/aitjcize/esp32-photoframe/main/process-cli/utils.js /app/bin/utils.js
ADD https://raw.githubusercontent.com/aitjcize/esp32-photoframe/main/process-cli/server.js /app/bin/server.js

# Install dependencies for Processor Script
WORKDIR /app/bin
# Install dependencies required by process-cli (commander, node-fetch, form-data, etc)
RUN npm install canvas exif-parser heic-convert form-data commander node-fetch

WORKDIR /app

# Environment Variables
ENV PORT=8080
ENV STATIC_DIR=/app/static
ENV DB_PATH=/data/photoframe.db
ENV DATA_DIR=/data

EXPOSE 9607

ENTRYPOINT ["/app/photoframe-server"]
