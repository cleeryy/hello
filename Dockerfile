# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Only module files first to leverage Docker layer cache
COPY go.mod go.sum ./
RUN go mod download

# Now copy the rest of the source code
COPY . .

# Tidy modules and build the binary in one layer
RUN go mod tidy && \
    go build -o hello .

# Runtime stage
FROM alpine:3.18

WORKDIR /root/

# Copy the built binary from the named builder stage
COPY --from=builder /app/hello /root/hello

EXPOSE 8080

CMD ["./hello"]
