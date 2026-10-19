# Stage 1: Build - Use alpine for small footprint with all tooling needed
FROM cgr.dev/chainguard/go:latest AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd cmd
COPY internal internal

# Build with CGO disabled for static linking (smaller binary, fewer deps)
# Use -a to force recompile of all packages regardless of mtime
# Link against libc (musl) so runtime can be even smaller
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-s -w" -o server ./cmd/server

# Stage 2: Runtime - Use distroless for hardened, tiny image
# Only copies the binary and config needed to run
FROM cgr.dev/chainguard/static:latest AS runtime

WORKDIR /app

# Copy built binary
COPY --from=builder /app/server ./server

# Expose ports for API and potential health checks
EXPOSE 8080

# Run the server binary
CMD ["./server"]
