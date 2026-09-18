FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# Now copy the rest of the source code
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /wol-api ./cmd/api

FROM alpine:3.18
WORKDIR /app

COPY --from=builder /wol-api /app/wol-api

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -qO- http://127.0.0.1:8080/health || exit 1

CMD ["./wol-api"]
