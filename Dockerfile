# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build all binaries
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/ingest ./cmd/ingest
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/pipeline ./cmd/pipeline
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/report ./cmd/report
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/replay ./cmd/replay
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/backtest ./cmd/backtest

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates tzdata

# Copy binaries from builder
COPY --from=builder /bin/ingest /app/ingest
COPY --from=builder /bin/pipeline /app/pipeline
COPY --from=builder /bin/report /app/report
COPY --from=builder /bin/replay /app/replay
COPY --from=builder /bin/backtest /app/backtest

# Copy SQL migrations
COPY sql/ /app/sql/

# Default command
ENTRYPOINT ["/app/ingest"]
