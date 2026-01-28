FROM golang:1.25.3-alpine AS builder
WORKDIR /app

COPY go.mod .

RUN go mod download

COPY . .

RUN rm go.sum
RUN go build -o hello .

FROM alpine:latest

WORKDIR /root/

COPY --from=0 /app/hello /root/hello

EXPOSE 8080

CMD ["./hello"]
