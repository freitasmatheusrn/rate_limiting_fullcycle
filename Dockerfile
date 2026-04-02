FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o rate_limiter .

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/rate_limiter .
COPY .env .

EXPOSE 8080

CMD ["./rate_limiter"]
