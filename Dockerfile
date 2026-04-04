# Admin UI build stage
FROM node:22-alpine AS admin-ui-builder

WORKDIR /ui

COPY web/admin-ui/package.json web/admin-ui/package-lock.json ./
RUN npm ci

COPY web/admin-ui/ ./
RUN npm run build

# Go build stage
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy the prebuilt admin UI bundle into the Go workspace
COPY --from=admin-ui-builder /ui/dist /app/web/admin-ui/dist

# Build the application for the requested target platform
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /support-bot ./cmd/bot

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /support-bot /app/support-bot

# Copy built admin UI
COPY --from=builder /app/web/admin-ui/dist /app/web/admin-ui/dist

# Copy config file
COPY config.yaml /app/config.yaml

# Run the bot
CMD ["/app/support-bot"]
