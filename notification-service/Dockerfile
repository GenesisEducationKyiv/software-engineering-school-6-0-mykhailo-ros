FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY gen/ gen/
COPY notification-service/go.mod notification-service/go.sum notification-service/
WORKDIR /app/notification-service
RUN go mod download
WORKDIR /app
COPY notification-service/ notification-service/
WORKDIR /app/notification-service
RUN go build -o main ./cmd/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/notification-service/main .
CMD ["./main"]
