# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY main.go ./
COPY internal/ internal/

# Build static binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -a -trimpath -installsuffix cgo \
    -o main .

# Production stage - using scratch for minimal size
FROM scratch AS production

# Copy the static binary
COPY --from=builder /app/main /main

EXPOSE 8080

ENTRYPOINT ["/main"]
